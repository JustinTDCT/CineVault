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
	"strings"
	"sync"
	"time"
)

// Client searches MusicBrainz. One Client is shared, so its limiter keeps
// every caller to one request a second.
type Client struct {
	BaseURL   string // default https://musicbrainz.org/ws/2
	UserAgent string
	HTTP      *http.Client
	Every     time.Duration // default 1.1 s

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
				ReleaseGroup struct {
					PrimaryType      string   `json:"primary-type"`
					SecondaryTypes   []string `json:"secondary-types"`
					FirstReleaseDate string   `json:"first-release-date"`
				} `json:"release-group"`
			} `json:"releases"`
		} `json:"recordings"`
	}
	if err := c.get(ctx, "/recording", url.Values{"query": {q}, "limit": {"15"}}, &out); err != nil {
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
			rec.Releases = append(rec.Releases, Release{Title: rel.Title, Status: rel.Status, Date: rel.Date,
				PrimaryType: rel.ReleaseGroup.PrimaryType, SecondaryTypes: rel.ReleaseGroup.SecondaryTypes,
				GroupDate: rel.ReleaseGroup.FirstReleaseDate})
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// Album picks the release to name for a recording: an official studio album
// first, then a single or EP, then anything; the earliest of the best kind.
// The same rule as the CineVault Cache's.
func (r Recording) Album() (Release, bool) {
	if len(r.Releases) == 0 {
		return Release{}, false
	}
	rank := func(x Release) int {
		n := 0
		if x.Status != "Official" {
			n += 10
		}
		switch x.PrimaryType {
		case "Album":
			if len(x.SecondaryTypes) > 0 {
				n += 2
			}
		case "Single", "EP":
			n++
		default:
			n += 3
		}
		return n
	}
	date := func(x Release) string {
		for _, d := range []string{x.GroupDate, x.Date} {
			if d != "" {
				return d
			}
		}
		return "9999"
	}
	rels := append([]Release(nil), r.Releases...)
	sort.SliceStable(rels, func(i, j int) bool {
		if a, b := rank(rels[i]), rank(rels[j]); a != b {
			return a < b
		}
		return date(rels[i]) < date(rels[j])
	})
	return rels[0], true
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

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if err := c.wait(ctx); err != nil {
		return err
	}
	base := c.BaseURL
	if base == "" {
		base = "https://musicbrainz.org/ws/2"
	}
	q.Set("fmt", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("MusicBrainz: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MusicBrainz answered %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}
