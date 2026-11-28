// Package cachesync connects this server to the CineVault Cache (cache spec
// §3–§5a): lookups for newly matched titles, the scheduled update check, and
// the daily list of titles shared with the cache. The cache is never in the
// critical path: every failure leaves the server on its own TMDb data.
package cachesync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	mrand "math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
)

// Cache states of a title (spec §3). "Update available" is applied at once,
// so it never rests on a title.
const (
	StateNone     = "none"
	StatePending  = "pending"
	StateCached   = "cached"
	StateNotFound = "not_found"
	StateError    = "error"
)

// Setting keys.
const (
	keyEnabled        = "cache.enabled"
	keyToken          = "cache.token" // saved in Settings; never returned by the API
	keyInterval       = "cache.check_interval"
	keyShare          = "cache.share"
	keyInstall        = "cache.install_id"
	keyLastCheck      = "cache.last_check"
	keyLastShare      = "cache.last_share"
	keyShareAt        = "cache.share_minute" // minute of the day to share, chosen at random once
	keyLastError      = "cache.last_error"
	keyLastShareError = "cache.last_share_error"
)

// Intervals offered in Settings (spec §4).
var Intervals = map[string]time.Duration{"1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour, "168h": 7 * 24 * time.Hour, "manual": 0}

// DefaultInterval is Daily.
const DefaultInterval = "24h"

// Item is one title the cache knows about or could: a matched movie or show.
type Item struct {
	TitleID         int64
	Key             cacheapi.Key
	Name            string
	State           string
	Version         int
	RetryAt         time.Time
	ContributedHash string
	// Owned are the editions the library has files for, in Edition List
	// order, with their measured runtimes (movies only).
	Owned []cacheapi.ContributeEdition
}

// Store is what the service needs from the database.
type Store interface {
	CacheItems(ctx context.Context) ([]Item, error)
	ApplyRecord(ctx context.Context, titleID int64, rec cacheapi.Record) error
	SetCacheState(ctx context.Context, titleID int64, state string, retryAt time.Time, msg string) error
	SetContributed(ctx context.Context, hashes map[int64]string) error
	Setting(ctx context.Context, key string, v any) (bool, error)
	SaveSetting(ctx context.Context, key string, v any) error
}

// Service runs the cache client. Client holds the service address and, when
// CINEVAULT_CACHE_TOKEN is set, its token; otherwise a token saved in
// Settings is used. With no token only sharing works (free servers).
type Service struct {
	Store  Store
	Client *cacheapi.Client
	Log    *slog.Logger
	// Now is the clock; tests replace it.
	Now func() time.Time

	mu      sync.Mutex // one check at a time
	running bool

	cur atomic.Pointer[connection]
}

// connection is the client in use and whether the cache is turned on. It is
// replaced whole when Settings change, so running calls keep the one they
// started with.
type connection struct {
	client  *cacheapi.Client // nil when the service has no address
	enabled bool
	source  string // where the token came from: TokenFromEnvironment, TokenSaved or ""
}

// Where a token came from.
const (
	TokenFromEnvironment = "environment"
	TokenSaved           = "saved"
)

// ErrTokenFromEnvironment means the token is set by CINEVAULT_CACHE_TOKEN and
// can't be changed in Settings.
var ErrTokenFromEnvironment = errors.New("the token is set by CINEVAULT_CACHE_TOKEN; remove it there to manage it in Settings")

// ErrTokenShape means a token doesn't look like one the cache makes.
var ErrTokenShape = errors.New("a CineVault Cache token starts with cvc_ and is about 56 characters long")

// ErrTokenRejected means the cache didn't accept a token.
var ErrTokenRejected = errors.New("the CineVault Cache didn't accept this token")

// Init reads the master switch and any saved token. Call it once at startup;
// until then the service uses Client as given, turned on.
func (s *Service) Init(ctx context.Context) error {
	enabled := true
	if _, err := s.Store.Setting(ctx, keyEnabled, &enabled); err != nil {
		return err
	}
	var saved string
	if _, err := s.Store.Setting(ctx, keyToken, &saved); err != nil {
		return err
	}
	s.connect(enabled, saved)
	return nil
}

// connect builds the connection: the environment's token wins over a saved one.
func (s *Service) connect(enabled bool, saved string) {
	c := &connection{enabled: enabled}
	if s.Client != nil {
		cl := *s.Client
		switch {
		case cl.Token != "":
			c.source = TokenFromEnvironment
		case saved != "":
			cl.Token, c.source = saved, TokenSaved
		}
		c.client = &cl
	}
	s.cur.Store(c)
}

func (s *Service) conn() *connection {
	if c := s.cur.Load(); c != nil {
		return c
	}
	c := &connection{client: s.Client, enabled: true}
	if s.Client != nil && s.Client.Token != "" {
		c.source = TokenFromEnvironment
	}
	return c
}

// Enabled reports whether the cache is turned on in Settings.
func (s *Service) Enabled() bool { return s.conn().enabled }

// Subscribed reports whether the cache is on and a token is set.
func (s *Service) Subscribed() bool {
	c := s.conn()
	return c.enabled && c.client != nil && c.client.Token != ""
}

// SetToken checks a token with the cache and saves it. A token the cache
// refuses isn't saved; when the cache can't be reached the token is saved
// anyway and the returned note says it wasn't checked.
func (s *Service) SetToken(ctx context.Context, token string) (string, error) {
	if s.conn().source == TokenFromEnvironment {
		return "", ErrTokenFromEnvironment
	}
	token = strings.TrimSpace(token)
	if !validToken(token) {
		return "", ErrTokenShape
	}
	note := ""
	if s.Client != nil {
		try := *s.Client
		try.Token = token
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := try.Subscription(c)
		cancel()
		switch {
		case cacheapi.IsAuth(err):
			return "", ErrTokenRejected
		case err != nil && !cacheapi.IsInactive(err):
			note = "Saved, but not checked: " + describe(err) + "."
		}
	}
	if err := s.Store.SaveSetting(ctx, keyToken, token); err != nil {
		return "", err
	}
	s.connect(s.Enabled(), token)
	return note, nil
}

// ClearToken removes a saved token.
func (s *Service) ClearToken(ctx context.Context) error {
	if s.conn().source == TokenFromEnvironment {
		return ErrTokenFromEnvironment
	}
	if err := s.Store.SaveSetting(ctx, keyToken, ""); err != nil {
		return err
	}
	s.connect(s.Enabled(), "")
	return nil
}

// validToken checks a token's shape: "cvc_" and base32, as the cache makes them.
func validToken(t string) bool {
	if !strings.HasPrefix(t, "cvc_") || len(t) < 20 || len(t) > 200 {
		return false
	}
	for _, r := range t[4:] {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '=' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// tokenHint is the end of the token, to recognise it by: "…X7QF".
func tokenHint(t string) string {
	if len(t) < 8 {
		return ""
	}
	return "…" + t[len(t)-4:]
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Settings are the user's choices for the cache.
type Settings struct {
	// Enabled is the master switch: off, the server never contacts the
	// cache (no lookups, checks, reports or sharing).
	Enabled       bool   `json:"enabled"`
	CheckInterval string `json:"check_interval"` // a key of Intervals
	Share         bool   `json:"share"`          // "Help build the CineVault Cache"
}

// LoadSettings reads the settings, with the defaults: on, Daily, sharing on.
func (s *Service) LoadSettings(ctx context.Context) (Settings, error) {
	out := Settings{Enabled: true, CheckInterval: DefaultInterval, Share: true}
	if _, err := s.Store.Setting(ctx, keyEnabled, &out.Enabled); err != nil {
		return out, err
	}
	if _, err := s.Store.Setting(ctx, keyInterval, &out.CheckInterval); err != nil {
		return out, err
	}
	if _, err := s.Store.Setting(ctx, keyShare, &out.Share); err != nil {
		return out, err
	}
	if _, ok := Intervals[out.CheckInterval]; !ok {
		out.CheckInterval = DefaultInterval
	}
	return out, nil
}

// SaveSettings stores the settings.
func (s *Service) SaveSettings(ctx context.Context, st Settings) error {
	if _, ok := Intervals[st.CheckInterval]; !ok {
		return fmt.Errorf("check_interval must be one of 1h, 6h, 24h, 168h or manual")
	}
	if err := s.Store.SaveSetting(ctx, keyInterval, st.CheckInterval); err != nil {
		return err
	}
	if err := s.Store.SaveSetting(ctx, keyShare, st.Share); err != nil {
		return err
	}
	if err := s.Store.SaveSetting(ctx, keyEnabled, st.Enabled); err != nil {
		return err
	}
	saved := ""
	if c := s.conn(); c.source == TokenSaved {
		saved = c.client.Token
	}
	s.connect(st.Enabled, saved)
	return nil
}

// CheckResult summarises one check.
type CheckResult struct {
	Looked    int    `json:"looked_up"`
	Applied   int    `json:"applied"`
	Pending   int    `json:"pending"`
	NotFound  int    `json:"not_found"`
	Unchanged int    `json:"unchanged"`
	Error     string `json:"error,omitempty"`
}

// ErrBusy means a check is already running.
var ErrBusy = errors.New("a cache check is already running")

// Check looks up new titles and asks the cache what changed (spec §4). Free
// servers have nothing to check.
func (s *Service) Check(ctx context.Context) (CheckResult, error) {
	var res CheckResult
	if !s.Subscribed() {
		return res, nil
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return res, ErrBusy
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	err := s.check(ctx, &res)
	_ = s.Store.SaveSetting(ctx, keyLastCheck, s.now().UTC())
	msg := ""
	if err != nil {
		msg = describe(err)
		res.Error = msg
	}
	_ = s.Store.SaveSetting(ctx, keyLastError, msg)
	return res, err
}

func (s *Service) check(ctx context.Context, res *CheckResult) error {
	items, err := s.Store.CacheItems(ctx)
	if err != nil {
		return err
	}
	now := s.now()

	// New titles: one lookup each, so a ready record arrives straight away.
	var sync []cacheapi.SyncItem
	byKey := map[cacheapi.Key]Item{}
	for _, it := range items {
		byKey[it.Key] = it
		if it.RetryAt.After(now) {
			continue
		}
		if it.State == StateNone || it.State == StateNotFound || it.State == StateError && it.Version == 0 {
			res.Looked++
			if err := s.lookup(ctx, it, res); err != nil {
				return err
			}
			continue
		}
		sync = append(sync, cacheapi.SyncItem{Kind: it.Key.Kind, TMDbID: it.Key.TMDbID, MBID: it.Key.MBID, HaveVersion: it.Version})
	}

	// Everything else in one batched call.
	if len(sync) == 0 {
		return nil
	}
	resp, err := s.conn().client.SyncAll(ctx, sync)
	if err != nil {
		return err
	}
	res.Unchanged += resp.Unchanged
	res.Pending += len(resp.Pending)
	for _, k := range resp.Pending {
		if it, ok := byKey[k]; ok && it.State != StatePending {
			_ = s.Store.SetCacheState(ctx, it.TitleID, StatePending, time.Time{}, "")
		}
	}
	for _, k := range resp.NotFound {
		if it, ok := byKey[k]; ok {
			res.NotFound++
			_ = s.Store.SetCacheState(ctx, it.TitleID, StateNotFound, now.Add(30*24*time.Hour), "")
		}
	}
	var keys []cacheapi.Key
	for _, c := range resp.Changed {
		keys = append(keys, c.Key())
	}
	if len(keys) == 0 {
		return nil
	}
	recs, err := s.conn().client.Records(ctx, keys)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		if it, ok := byKey[rec.Key]; ok {
			if err := s.apply(ctx, it, rec); err != nil {
				return err
			}
			res.Applied++
		}
	}
	return nil
}

// LookupNew looks up titles that haven't been asked about yet, for example
// right after a scan matched them. Errors are logged, never returned: the
// scan has already succeeded with direct sources.
func (s *Service) LookupNew(ctx context.Context) {
	if !s.Subscribed() {
		return
	}
	items, err := s.Store.CacheItems(ctx)
	if err != nil {
		s.Log.Warn("cache: list titles", "error", err)
		return
	}
	var res CheckResult
	for _, it := range items {
		if it.State != StateNone {
			continue
		}
		if err := s.lookup(ctx, it, &res); err != nil {
			s.Log.Warn("cache lookup stopped", "error", describe(err))
			_ = s.Store.SaveSetting(ctx, keyLastError, describe(err))
			return
		}
	}
	if res.Applied+res.Pending+res.NotFound > 0 {
		s.Log.Info("cache lookups", "cached", res.Applied, "pending", res.Pending, "not_found", res.NotFound)
	}
}

// lookup asks for one title. It returns an error only when the whole run
// should stop (bad token, inactive subscription, cache unreachable).
func (s *Service) lookup(ctx context.Context, it Item, res *CheckResult) error {
	var editions []string
	for _, e := range it.Owned {
		editions = append(editions, e.Edition)
	}
	r, err := s.conn().client.Lookup(ctx, it.Key, editions...)
	switch {
	case err == nil:
	case cacheapi.IsAuth(err) || cacheapi.IsInactive(err):
		return err
	case cacheapi.IsRetryable(err):
		_ = s.Store.SetCacheState(ctx, it.TitleID, StateError, s.now().Add(time.Hour), describe(err))
		return err
	default: // a bad request for this one title; don't stop the run
		_ = s.Store.SetCacheState(ctx, it.TitleID, StateError, s.now().Add(24*time.Hour), describe(err))
		return nil
	}
	switch r.State {
	case cacheapi.Ready:
		res.Applied++
		return s.apply(ctx, it, *r.Record)
	case cacheapi.Researching:
		res.Pending++
		return s.Store.SetCacheState(ctx, it.TitleID, StatePending, time.Time{}, "")
	default:
		res.NotFound++
		return s.Store.SetCacheState(ctx, it.TitleID, StateNotFound, s.now().Add(30*24*time.Hour), "")
	}
}

func (s *Service) apply(ctx context.Context, it Item, rec cacheapi.Record) error {
	if err := rec.Validate(); err != nil {
		// The cache promises valid records; a bad one is a bug there. Keep
		// what we have and say so.
		s.Log.Warn("cache record rejected", "title", it.Name, "error", err)
		return s.Store.SetCacheState(ctx, it.TitleID, StateError, s.now().Add(24*time.Hour), "the cache sent a record that failed validation")
	}
	return s.Store.ApplyRecord(ctx, it.TitleID, rec)
}

// ShareResult summarises one share.
type ShareResult struct {
	Sent     int `json:"sent"`
	Accepted int `json:"accepted"`
}

// Share sends the titles added or changed since the last share (spec §5a).
// It runs for free and subscribed servers alike, when sharing is on.
func (s *Service) Share(ctx context.Context) (ShareResult, error) {
	var res ShareResult
	st, err := s.LoadSettings(ctx)
	c := s.conn()
	if err != nil || !st.Share || !c.enabled || c.client == nil {
		return res, err
	}
	install, err := s.installID(ctx)
	if err != nil {
		return res, err
	}
	items, err := s.Store.CacheItems(ctx)
	if err != nil {
		return res, err
	}
	var out []cacheapi.ContributeItem
	hashes := map[int64]string{}
	for _, it := range items {
		ci := cacheapi.ContributeItem{Kind: it.Key.Kind, TMDbID: it.Key.TMDbID, MBID: it.Key.MBID, Editions: it.Owned}
		b, _ := json.Marshal(ci)
		sum := sha256.Sum256(b)
		h := hex.EncodeToString(sum[:8])
		if h == it.ContributedHash {
			continue
		}
		out = append(out, ci)
		hashes[it.TitleID] = h
	}
	_ = s.Store.SaveSetting(ctx, keyLastShare, s.now().UTC())
	if len(out) == 0 {
		_ = s.Store.SaveSetting(ctx, keyLastShareError, "")
		return res, nil
	}
	res.Sent = len(out)
	res.Accepted, err = c.client.Contribute(ctx, install, out)
	if err != nil {
		_ = s.Store.SaveSetting(ctx, keyLastShareError, describe(err))
		return res, err
	}
	_ = s.Store.SaveSetting(ctx, keyLastShareError, "")
	return res, s.Store.SetContributed(ctx, hashes)
}

// installID returns this server's random install ID, making it on first use.
func (s *Service) installID(ctx context.Context) (string, error) {
	var id string
	if ok, err := s.Store.Setting(ctx, keyInstall, &id); err != nil {
		return "", err
	} else if ok && id != "" {
		return id, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	return id, s.Store.SaveSetting(ctx, keyInstall, id)
}

// Run does the scheduled work until ctx ends: the update check at the user's
// interval and sharing once a day at a random minute (spec §4, §5a). Failed
// checks are retried with back-off.
func (s *Service) Run(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	backoff := 0
	for {
		s.tick(ctx, &backoff)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Service) tick(ctx context.Context, backoff *int) {
	st, err := s.LoadSettings(ctx)
	if err != nil {
		return
	}
	now := s.now()
	if every := Intervals[st.CheckInterval]; every > 0 && s.Subscribed() {
		var last time.Time
		_, _ = s.Store.Setting(ctx, keyLastCheck, &last)
		wait := every
		if *backoff > 0 { // after a failure: 5 min, 10, 20 ... up to the interval
			wait = min(every, time.Duration(5*math.Pow(2, float64(*backoff-1)))*time.Minute)
		}
		if now.Sub(last) >= wait {
			res, err := s.Check(ctx)
			switch {
			case errors.Is(err, ErrBusy):
			case err != nil:
				*backoff++
				s.Log.Warn("cache check failed", "error", describe(err), "retry_in", min(every, time.Duration(5*math.Pow(2, float64(*backoff-1)))*time.Minute))
			default:
				*backoff = 0
				if res.Looked+res.Applied+res.NotFound > 0 {
					s.Log.Info("cache check", "looked_up", res.Looked, "applied", res.Applied, "pending", res.Pending, "not_found", res.NotFound, "unchanged", res.Unchanged)
				}
			}
		}
	}
	if st.Share {
		var minute int
		if ok, _ := s.Store.Setting(ctx, keyShareAt, &minute); !ok {
			minute = mrand.IntN(24 * 60)
			_ = s.Store.SaveSetting(ctx, keyShareAt, minute)
		}
		var last time.Time
		_, _ = s.Store.Setting(ctx, keyLastShare, &last)
		due := time.Date(now.Year(), now.Month(), now.Day(), 0, minute, 0, 0, now.Location())
		if due.After(now) {
			due = due.Add(-24 * time.Hour)
		}
		if last.Before(due) {
			if r, err := s.Share(ctx); err != nil {
				s.Log.Debug("sharing titles with the cache failed; trying again tomorrow", "error", describe(err))
			} else if r.Sent > 0 {
				s.Log.Info("shared titles with the cache", "sent", r.Sent, "accepted", r.Accepted)
			}
		}
	}
}

// Status is what Settings shows.
type Status struct {
	// Configured: the cache is on and a token is set.
	Configured bool   `json:"configured"`
	URL        string `json:"url"`
	// TokenSource is "environment", "saved" or "" (none); TokenHint is its
	// last characters. The token itself is never sent.
	TokenSource    string                 `json:"token_source,omitempty"`
	TokenHint      string                 `json:"token_hint,omitempty"`
	Subscription   *cacheapi.Subscription `json:"subscription,omitempty"`
	Problem        string                 `json:"problem,omitempty"`
	Settings       Settings               `json:"settings"`
	LastCheck      *time.Time             `json:"last_check,omitempty"`
	LastError      string                 `json:"last_error,omitempty"`
	LastShare      *time.Time             `json:"last_share,omitempty"`
	LastShareError string                 `json:"last_share_error,omitempty"`
	Checking       bool                   `json:"checking"`
	Counts         map[string]int         `json:"counts"`
}

// Status reports the connection, the subscription and the item counts.
func (s *Service) Status(ctx context.Context) (Status, error) {
	conn := s.conn()
	st := Status{Configured: s.Subscribed(), TokenSource: conn.source, Counts: map[string]int{}}
	s.mu.Lock()
	st.Checking = s.running
	s.mu.Unlock()
	if conn.client != nil {
		st.URL = conn.client.BaseURL
		st.TokenHint = tokenHint(conn.client.Token)
	}
	var err error
	if st.Settings, err = s.LoadSettings(ctx); err != nil {
		return st, err
	}
	var t time.Time
	if ok, _ := s.Store.Setting(ctx, keyLastCheck, &t); ok {
		st.LastCheck = &t
	}
	var t2 time.Time
	if ok, _ := s.Store.Setting(ctx, keyLastShare, &t2); ok {
		st.LastShare = &t2
	}
	_, _ = s.Store.Setting(ctx, keyLastError, &st.LastError)
	_, _ = s.Store.Setting(ctx, keyLastShareError, &st.LastShareError)
	items, err := s.Store.CacheItems(ctx)
	if err != nil {
		return st, err
	}
	for _, it := range items {
		st.Counts[it.State]++
	}
	if st.Configured {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		sub, err := conn.client.Subscription(c)
		if err != nil {
			st.Problem = describe(err)
		} else {
			st.Subscription = &sub
		}
	}
	return st, nil
}

// Report sends "Report a mistake" for one of the title's editions.
func (s *Service) Report(ctx context.Context, key cacheapi.Key, r cacheapi.Report) (cacheapi.ReportResponse, error) {
	if !s.Subscribed() {
		return cacheapi.ReportResponse{}, errors.New("reports need the CineVault Cache turned on and a subscription")
	}
	r.Key = key
	if err := r.Validate(); err != nil {
		return cacheapi.ReportResponse{}, err
	}
	return s.conn().client.Report(ctx, r)
}

// describe turns a cache error into a sentence for Settings and logs.
func describe(err error) string {
	var e *cacheapi.Error
	if errors.As(err, &e) && e.Message != "" {
		return e.Message
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "the CineVault Cache didn't answer in time"
	}
	return "can't reach the CineVault Cache: " + err.Error()
}
