package cachesync

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
)

// fakeStore keeps items and settings in memory.
type fakeStore struct {
	mu       sync.Mutex
	items    []Item
	applied  map[int64]cacheapi.Record
	retry    map[int64]time.Time
	errs     map[int64]string
	settings map[string][]byte
}

func newStore(items ...Item) *fakeStore {
	return &fakeStore{items: items, applied: map[int64]cacheapi.Record{}, retry: map[int64]time.Time{}, errs: map[int64]string{}, settings: map[string][]byte{}}
}

func (f *fakeStore) CacheItems(context.Context) ([]Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Item(nil), f.items...), nil
}

func (f *fakeStore) find(id int64) *Item {
	for i := range f.items {
		if f.items[i].TitleID == id {
			return &f.items[i]
		}
	}
	return nil
}

func (f *fakeStore) ApplyRecord(_ context.Context, id int64, rec cacheapi.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied[id] = rec
	it := f.find(id)
	it.State, it.Version = StateCached, rec.RecordVersion
	return nil
}

func (f *fakeStore) SetCacheState(_ context.Context, id int64, state string, retryAt time.Time, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it := f.find(id)
	it.State, it.RetryAt = state, retryAt
	f.retry[id], f.errs[id] = retryAt, msg
	return nil
}

func (f *fakeStore) SetContributed(_ context.Context, hashes map[int64]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, h := range hashes {
		f.find(id).ContributedHash = h
	}
	return nil
}

func (f *fakeStore) Setting(_ context.Context, key string, v any) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.settings[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(b, v)
}

func (f *fakeStore) SaveSetting(_ context.Context, key string, v any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, err := json.Marshal(v)
	f.settings[key] = b
	return err
}

// fakeCache answers like the cache service: Aliens (679) is ready, Alien
// (348) is being researched, anything else is unknown.
type fakeCache struct {
	mu         sync.Mutex
	lookups    []string
	syncs      int
	contribute []cacheapi.ContributeRequest
	changed    []cacheapi.Changed
	status     int // forced status for every authed call
}

func (fc *fakeCache) server(t *testing.T) *httptest.Server {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join("..", "..", "pkg", "cacheapi", "testdata", "record_aliens.json"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	forced := func(w http.ResponseWriter) bool {
		if fc.status == 0 {
			return false
		}
		w.WriteHeader(fc.status)
		w.Write([]byte(`{"error":{"code":"token_revoked","message":"This token was revoked."}}`))
		return true
	}
	mux.HandleFunc("GET /v1/titles/{kind}/tmdb/{id}", func(w http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		fc.lookups = append(fc.lookups, r.PathValue("id")+"?"+r.URL.RawQuery)
		fc.mu.Unlock()
		if forced(w) {
			return
		}
		switch r.PathValue("id") {
		case "679":
			w.Write(golden)
		case "348":
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"status":"pending","retry_after_s":3600}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"code":"not_found","message":"no such title"}}`))
		}
	})
	mux.HandleFunc("POST /v1/sync", func(w http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		fc.syncs++
		fc.mu.Unlock()
		if forced(w) {
			return
		}
		var req cacheapi.SyncRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(cacheapi.SyncResponse{Changed: fc.changed, Pending: []cacheapi.Key{}, NotFound: []cacheapi.Key{}, Unchanged: len(req.Items) - len(fc.changed)})
	})
	mux.HandleFunc("POST /v1/records", func(w http.ResponseWriter, r *http.Request) {
		var req cacheapi.RecordsRequest
		json.NewDecoder(r.Body).Decode(&req)
		var rec cacheapi.Record
		json.Unmarshal(golden, &rec)
		out := cacheapi.RecordsResponse{}
		for _, k := range req.Keys {
			rr := rec
			rr.Key = k
			rr.RecordVersion = 9
			out.Records = append(out.Records, rr)
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /v1/subscription", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Authorization"), "BAD") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"code":"token_invalid","message":"This token isn't valid."}}`))
			return
		}
		w.Write([]byte(`{"status":"active","server_label":"Test"}`))
	})
	mux.HandleFunc("POST /v1/contribute", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("contribution carried a token")
		}
		var req cacheapi.ContributeRequest
		json.NewDecoder(r.Body).Decode(&req)
		fc.mu.Lock()
		fc.contribute = append(fc.contribute, req)
		fc.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(cacheapi.ContributeResponse{Accepted: len(req.Items)})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newService(t *testing.T, st *fakeStore, token string) (*Service, *fakeCache, *time.Time) {
	t.Helper()
	fc := &fakeCache{}
	srv := fc.server(t)
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	s := &Service{
		Store:  st,
		Client: &cacheapi.Client{BaseURL: srv.URL + "/", Token: token},
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return now },
	}
	return s, fc, &now
}

func movie(id int64, tmdb int, editions ...cacheapi.ContributeEdition) Item {
	return Item{TitleID: id, Key: cacheapi.Key{Kind: cacheapi.KindMovie, TMDbID: tmdb}, Name: "t", State: StateNone, Owned: editions}
}

func TestCheckLooksUpNewTitlesThenSyncs(t *testing.T) {
	st := newStore(
		movie(1, 679, cacheapi.ContributeEdition{Edition: "theatrical", RuntimeMin: 137}),
		movie(2, 348, cacheapi.ContributeEdition{Edition: "directors_cut", RuntimeMin: 116}),
		movie(3, 999),
	)
	s, fc, now := newService(t, st, "cvc_T")

	res, err := s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Looked != 3 || res.Applied != 1 || res.Pending != 1 || res.NotFound != 1 || fc.syncs != 0 {
		t.Fatalf("first check = %+v, syncs %d", res, fc.syncs)
	}
	if fc.lookups[0] != "679?editions=theatrical" {
		t.Errorf("lookup sent %q, want the owned editions", fc.lookups[0])
	}
	if st.find(1).State != StateCached || st.find(2).State != StatePending || st.find(3).State != StateNotFound {
		t.Errorf("states %s %s %s", st.find(1).State, st.find(2).State, st.find(3).State)
	}
	if !st.retry[3].Equal(now.Add(30 * 24 * time.Hour)) {
		t.Errorf("not found retry at %v", st.retry[3])
	}

	// Second check: cached and pending titles go in one sync; the unknown
	// title waits out its 30 days.
	fc.changed = []cacheapi.Changed{{Kind: cacheapi.KindMovie, TMDbID: 348, RecordVersion: 9}}
	fc.lookups = nil
	res, err = s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fc.syncs != 1 || len(fc.lookups) != 0 || res.Applied != 1 || st.find(2).State != StateCached || st.find(2).Version != 9 {
		t.Errorf("second check = %+v, syncs %d, lookups %v, alien %+v", res, fc.syncs, fc.lookups, *st.find(2))
	}
	var last time.Time
	if ok, _ := st.Setting(context.Background(), keyLastCheck, &last); !ok || !last.Equal(*now) {
		t.Errorf("last check %v", last)
	}
}

func TestBadTokenStopsAndIsReported(t *testing.T) {
	st := newStore(movie(1, 679), movie(2, 348))
	s, fc, _ := newService(t, st, "cvc_T")
	fc.status = http.StatusUnauthorized
	if _, err := s.Check(context.Background()); !cacheapi.IsAuth(err) {
		t.Fatalf("err = %v, want an auth error", err)
	}
	if len(fc.lookups) != 1 {
		t.Errorf("lookups after a bad token = %d, want 1", len(fc.lookups))
	}
	stat, err := s.Status(context.Background())
	if err != nil || stat.LastError != "This token was revoked." {
		t.Errorf("status last error %q, %v", stat.LastError, err)
	}
}

func TestFreeServerNeverLooksUp(t *testing.T) {
	st := newStore(movie(1, 679))
	s, fc, _ := newService(t, st, "")
	if _, err := s.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.LookupNew(context.Background())
	if len(fc.lookups)+fc.syncs != 0 {
		t.Errorf("a free server made %d lookups and %d syncs", len(fc.lookups), fc.syncs)
	}
}

func TestShareSendsOnlyChangesAndHonoursOptOut(t *testing.T) {
	st := newStore(
		movie(1, 679, cacheapi.ContributeEdition{Edition: "theatrical", RuntimeMin: 137}),
		movie(2, 348, cacheapi.ContributeEdition{Edition: "directors_cut", RuntimeMin: 116}),
	)
	s, fc, _ := newService(t, st, "") // free servers share too
	ctx := context.Background()

	res, err := s.Share(ctx)
	if err != nil || res.Sent != 2 || res.Accepted != 2 || len(fc.contribute) != 1 {
		t.Fatalf("first share = %+v, %v, %d requests", res, err, len(fc.contribute))
	}
	install := fc.contribute[0].Install
	if len(install) < 16 {
		t.Errorf("install id %q", install)
	}

	if res, _ := s.Share(ctx); res.Sent != 0 || len(fc.contribute) != 1 {
		t.Errorf("unchanged library re-sent %d titles", res.Sent)
	}

	st.find(2).Owned = append(st.find(2).Owned, cacheapi.ContributeEdition{Edition: "theatrical", RuntimeMin: 117})
	if res, _ := s.Share(ctx); res.Sent != 1 || fc.contribute[1].Items[0].TMDbID != 348 || fc.contribute[1].Install != install {
		t.Errorf("after adding an edition: sent %d, request %+v", res.Sent, fc.contribute[len(fc.contribute)-1])
	}

	if err := s.SaveSettings(ctx, Settings{CheckInterval: "24h", Share: false}); err != nil {
		t.Fatal(err)
	}
	st.find(1).Owned = nil
	if res, _ := s.Share(ctx); res.Sent != 0 || len(fc.contribute) != 2 {
		t.Error("shared with sharing turned off")
	}
}

func TestSettingsDefaultsAndValidation(t *testing.T) {
	s, _, _ := newService(t, newStore(), "")
	ctx := context.Background()
	got, err := s.LoadSettings(ctx)
	if err != nil || got != (Settings{Enabled: true, CheckInterval: "24h", Share: true}) {
		t.Errorf("defaults = %+v, %v", got, err)
	}
	if err := s.SaveSettings(ctx, Settings{CheckInterval: "2h"}); err == nil {
		t.Error("2h accepted")
	}
	if err := s.SaveSettings(ctx, Settings{Enabled: true, CheckInterval: "manual", Share: false}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.LoadSettings(ctx); got != (Settings{Enabled: true, CheckInterval: "manual", Share: false}) {
		t.Errorf("saved = %+v", got)
	}
}

func TestTickChecksOnScheduleAndSharesDaily(t *testing.T) {
	st := newStore(movie(1, 679, cacheapi.ContributeEdition{Edition: "theatrical", RuntimeMin: 137}))
	s, fc, now := newService(t, st, "cvc_T")
	ctx := context.Background()
	st.SaveSetting(ctx, keyShareAt, 11*60) // 11:00, an hour ago

	backoff := 0
	s.tick(ctx, &backoff)
	if len(fc.lookups) != 1 || len(fc.contribute) != 1 {
		t.Fatalf("first tick: %d lookups, %d shares", len(fc.lookups), len(fc.contribute))
	}
	*now = now.Add(time.Hour)
	s.tick(ctx, &backoff)
	if fc.syncs != 0 || len(fc.contribute) != 1 {
		t.Errorf("an hour later: %d syncs, %d shares; want none before the day is up", fc.syncs, len(fc.contribute))
	}
	*now = now.Add(24 * time.Hour)
	st.find(1).Owned[0].RuntimeMin = 138
	s.tick(ctx, &backoff)
	if fc.syncs != 1 || len(fc.contribute) != 2 {
		t.Errorf("next day: %d syncs, %d shares", fc.syncs, len(fc.contribute))
	}

	// A failure backs off 5 minutes before trying again.
	fc.status = http.StatusServiceUnavailable
	*now = now.Add(25 * time.Hour)
	s.tick(ctx, &backoff)
	if backoff != 1 {
		t.Fatalf("backoff = %d after a failure", backoff)
	}
	fc.status = 0
	*now = now.Add(4 * time.Minute)
	s.tick(ctx, &backoff)
	if fc.syncs != 2 {
		t.Errorf("retried too early: syncs %d", fc.syncs)
	}
	*now = now.Add(2 * time.Minute)
	s.tick(ctx, &backoff)
	if fc.syncs != 3 || backoff != 0 {
		t.Errorf("after 6 minutes: syncs %d, backoff %d", fc.syncs, backoff)
	}
}

const goodToken = "cvc_ABCDEFGHIJKLMNOPQRSTUVWXYZ234567ABCDEFGHIJKLMNOPQR"

func TestTokenSavedInSettings(t *testing.T) {
	st := newStore(movie(1, 679))
	s, fc, _ := newService(t, st, "")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Subscribed() {
		t.Fatal("subscribed with no token")
	}

	if _, err := s.SetToken(ctx, "hello"); !errors.Is(err, ErrTokenShape) {
		t.Errorf("bad shape: %v", err)
	}
	if _, err := s.SetToken(ctx, "cvc_BADBADBADBADBADBADBADBADBAD"); !errors.Is(err, ErrTokenRejected) {
		t.Errorf("refused token: %v", err)
	}
	if s.Subscribed() {
		t.Fatal("a refused token was used")
	}
	note, err := s.SetToken(ctx, "  "+goodToken+"\n")
	if err != nil || note != "" || !s.Subscribed() {
		t.Fatalf("set: %q %v", note, err)
	}
	// The status names where the token came from and its end, never the token.
	stat, _ := s.Status(ctx)
	b, _ := json.Marshal(stat)
	if stat.TokenSource != TokenSaved || stat.TokenHint != "…OPQR" || strings.Contains(string(b), goodToken) {
		t.Errorf("status = %s", b)
	}
	// Lookups use it.
	if _, err := s.Check(ctx); err != nil || len(fc.lookups) != 1 {
		t.Errorf("check: %v, %d lookups", err, len(fc.lookups))
	}

	// A restart reads it back.
	s2, _, _ := newService(t, st, "")
	if err := s2.Init(ctx); err != nil || !s2.Subscribed() {
		t.Errorf("after restart: subscribed=%v %v", s2.Subscribed(), err)
	}
	if err := s2.ClearToken(ctx); err != nil || s2.Subscribed() {
		t.Errorf("clear: %v", err)
	}
}

func TestEnvironmentTokenWins(t *testing.T) {
	st := newStore()
	st.SaveSetting(context.Background(), keyToken, goodToken)
	s, _, _ := newService(t, st, "cvc_FROMENVIRONMENTFROMENVIRONMENT")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if s.conn().client.Token != "cvc_FROMENVIRONMENTFROMENVIRONMENT" || s.conn().source != TokenFromEnvironment {
		t.Errorf("token from %s", s.conn().source)
	}
	if _, err := s.SetToken(ctx, goodToken); !errors.Is(err, ErrTokenFromEnvironment) {
		t.Errorf("set over the environment: %v", err)
	}
	if err := s.ClearToken(ctx); !errors.Is(err, ErrTokenFromEnvironment) {
		t.Errorf("clear over the environment: %v", err)
	}
}

func TestMasterSwitchOff(t *testing.T) {
	st := newStore(movie(1, 679, cacheapi.ContributeEdition{Edition: "theatrical", RuntimeMin: 137}))
	s, fc, _ := newService(t, st, goodToken)
	ctx := context.Background()
	if err := s.SaveSettings(ctx, Settings{Enabled: false, CheckInterval: "24h", Share: true}); err != nil {
		t.Fatal(err)
	}
	if s.Subscribed() || s.Enabled() {
		t.Fatal("still on")
	}
	st.SaveSetting(ctx, keyShareAt, 11*60)
	backoff := 0
	s.tick(ctx, &backoff)
	s.LookupNew(ctx)
	if _, err := s.Share(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Report(ctx, cacheapi.Key{Kind: "movie", TMDbID: 679}, cacheapi.Report{Edition: "theatrical", Field: "runtime", Message: "x"}); err == nil {
		t.Error("report sent while off")
	}
	if len(fc.lookups)+fc.syncs+len(fc.contribute) != 0 {
		t.Errorf("contacted the cache while off: %d lookups, %d syncs, %d shares", len(fc.lookups), fc.syncs, len(fc.contribute))
	}
	// A restart keeps it off.
	s2, _, _ := newService(t, st, goodToken)
	if err := s2.Init(ctx); err != nil || s2.Enabled() {
		t.Errorf("after restart: enabled=%v %v", s2.Enabled(), err)
	}
}
