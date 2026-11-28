// Package musicbrainz matches music videos to MusicBrainz recordings
// (https://musicbrainz.org). Its core data is CC0. MusicBrainz asks for at
// most one request a second and a User-Agent naming the application.
package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Client searches MusicBrainz. One Client is shared, so its limiter keeps
// every caller to one request a second.
type Client struct {
	BaseURL   string // default https://musicbrainz.org/ws/2
	UserAgent string
	HTTP      *http.Client
	Every     time.Duration // default 1.1 s
	// Backoff is the first wait after a 503, MusicBrainz's "slow down"; it
	// doubles on each retry. Default 2 s.
	Backoff time.Duration

	mu   sync.Mutex
	next time.Time
}

// Recording is a search result.
type Recording struct {
	ID               string
	Title            string
	Artist           string // as credited, with join phrases
	Score            int    // MusicBrainz's search score, 0 to 100
	FirstReleaseDate string
	Releases         []Release
}

// Release is a release a recording is on.
type Release struct {
	Title          string
	Status         string // Official, Promotion, Bootleg…
	Date           string
	PrimaryType    string // Album, Single, EP…
	SecondaryTypes []string
	GroupDate      string // the release group's first release date
	Artist         string // the release's artist credit ("Various Artists"); empty when not given
}

// SearchRecordings looks a song up by title and artist.
func (c *Client) SearchRecordings(ctx context.Context, artist, title string) ([]Recording, error) {
	q := fmt.Sprintf(`recording:"%s"`, escape(title))
	if artist != "" {
		q += fmt.Sprintf(` AND artist:"%s"`, escape(artist))
	}
	var out struct {
		Recordings []struct {
			ID               string `json:"id"`
			Title            string `json:"title"`
			Score            int    `json:"score"`
			FirstReleaseDate string `json:"first-release-date"`
			ArtistCredit     []struct {
				Name       string `json:"name"`
				JoinPhrase string `json:"joinphrase"`
			} `json:"artist-credit"`
			Releases []struct {
				Title        string `json:"title"`
				Status       string `json:"status"`
				Date         string `json:"date"`
				ArtistCredit []struct {
					Name       string `json:"name"`
					JoinPhrase string `json:"joinphrase"`
				} `json:"artist-credit"`
				ReleaseGroup struct {
					PrimaryType      string   `json:"primary-type"`
					SecondaryTypes   []string `json:"secondary-types"`
					FirstReleaseDate string   `json:"first-release-date"`
				} `json:"release-group"`
			} `json:"releases"`
		} `json:"recordings"`
	}
	// Every compilation's copy of a song can be its own recording, all
	// scoring 100, so ask for as many as MusicBrainz gives: the album
	// recording has to be among them.
	if err := c.get(ctx, "/recording", url.Values{"query": {q}, "limit": {"100"}}, &out); err != nil {
		return nil, err
	}
	recs := make([]Recording, 0, len(out.Recordings))
	for _, r := range out.Recordings {
		rec := Recording{ID: r.ID, Title: r.Title, Score: r.Score, FirstReleaseDate: r.FirstReleaseDate}
		var b strings.Builder
		for _, a := range r.ArtistCredit {
			b.WriteString(a.Name + a.JoinPhrase)
		}
		rec.Artist = strings.TrimSpace(b.String())
		for _, rel := range r.Releases {
			var ra strings.Builder
			for _, a := range rel.ArtistCredit {
				ra.WriteString(a.Name + a.JoinPhrase)
			}
			rec.Releases = append(rec.Releases, Release{Title: rel.Title, Status: rel.Status, Date: rel.Date,
				PrimaryType: rel.ReleaseGroup.PrimaryType, SecondaryTypes: rel.ReleaseGroup.SecondaryTypes,
				GroupDate: rel.ReleaseGroup.FirstReleaseDate, Artist: strings.TrimSpace(ra.String())})
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// Album picks the release to name for a recording, by AlbumRank then
// AlbumDate: the artist's own official album or EP first, then their single,
// then their compilations; anything by other artists ("Various Artists") or
// unofficial last. The same rule as the CineVault Cache's.
func (r Recording) Album() (Release, bool) {
	if len(r.Releases) == 0 {
		return Release{}, false
	}
	rels := append([]Release(nil), r.Releases...)
	sort.SliceStable(rels, func(i, j int) bool {
		if a, b := AlbumRank(rels[i], r.Artist, r.Title), AlbumRank(rels[j], r.Artist, r.Title); a != b {
			return a < b
		}
		return AlbumDate(rels[i]) < AlbumDate(rels[j])
	})
	return rels[0], true
}

// AlbumRank orders releases for naming a song's album; lower is better. An
// album or EP named after the song itself counts as a single: MusicBrainz
// often types a one-song release as an EP (3 Doors Down's "It's Not My
// Time" EP, January 2008, months before the album).
func AlbumRank(rel Release, artist, song string) int {
	n := 0
	if rel.Artist != "" && !SameArtist(rel.Artist, artist) {
		n += 20
	}
	if rel.Status != "Official" {
		n += 10
	}
	switch rel.PrimaryType {
	case "Album", "EP":
		switch {
		case len(rel.SecondaryTypes) > 0: // compilation, live, soundtrack…
			n += 2
		case song != "" && sameTitle(rel.Title, song):
			n++
		}
	case "Single":
		n++
	default:
		n += 3
	}
	return n
}

// AlbumDate is a release's date for sorting. A date given only as a year
// or month sorts after the exact dates within it ("1994" after
// "1994-01-25"), and a missing date sorts last.
func AlbumDate(rel Release) string {
	d := rel.GroupDate
	if d == "" {
		d = rel.Date
	}
	switch len(d) {
	case 4:
		return d + "-99-99"
	case 7:
		return d + "-99"
	case 10:
		return d
	}
	return "9999"
}

// sameTitle compares titles ignoring case, punctuation and typographic
// apostrophes ("It’s Not My Time" and "It's not my time").
func sameTitle(a, b string) bool {
	key := func(s string) string {
		var out strings.Builder
		for _, r := range strings.ToLower(s) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				out.WriteRune(r)
			}
		}
		return out.String()
	}
	return key(a) != "" && key(a) == key(b)
}

// SameArtist reports whether two artist credits name the same act, allowing
// for joint credits ("Queen" and "Queen & David Bowie").
func SameArtist(a, b string) bool {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

// escape quotes a value for MusicBrainz's Lucene search syntax.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

func (c *Client) wait(ctx context.Context) error {
	every := c.Every
	if every == 0 {
		every = 1100 * time.Millisecond
	}
	c.mu.Lock()
	now := time.Now()
	at := c.next
	if at.Before(now) {
		at = now
	}
	c.next = at.Add(every)
	c.mu.Unlock()
	if d := time.Until(at); d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// retries is how many times a 503 is retried before the video is marked
// "error" (the next scan or Match tries it again).
const retries = 3

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	base := c.BaseURL
	if base == "" {
		base = "https://musicbrainz.org/ws/2"
	}
	q.Set("fmt", "json")
	u := strings.TrimRight(base, "/") + path + "?" + q.Encode()
	backoff := c.Backoff
	if backoff == 0 {
		backoff = 2 * time.Second
	}
	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return err
		}
		wait, err := c.try(ctx, u, out)
		if wait == 0 || attempt == retries {
			return err
		}
		if wait < 0 {
			wait = backoff << attempt
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// try makes one request. A 503 returns how long to wait before retrying:
// MusicBrainz's Retry-After (at most 30 s), or -1 for the caller's backoff.
func (c *Client) try(ctx context.Context, u string, out any) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("MusicBrainz: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		return retryAfter(resp.Header.Get("Retry-After")), fmt.Errorf("MusicBrainz answered 503 (rate limited)")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("MusicBrainz answered %d", resp.StatusCode)
	}
	return 0, json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}

// retryAfter reads a Retry-After header in seconds; -1 when absent or odd.
func retryAfter(h string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil || n <= 0 {
		return -1
	}
	if n > 30 {
		n = 30
	}
	return time.Duration(n) * time.Second
}
