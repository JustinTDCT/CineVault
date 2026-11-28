// Package openlibrary matches books and audiobooks to Open Library works
// (https://openlibrary.org): the work, its first publication year, its
// description, subjects and cover. No key is needed; Open Library asks for a
// User-Agent naming the application and a contact, and a modest rate.
package openlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client talks to Open Library. One Client is shared, so its limiter keeps
// every caller to one request a second.
type Client struct {
	BaseURL   string // default https://openlibrary.org
	UserAgent string
	HTTP      *http.Client
	Every     time.Duration // default 1 s

	mu   sync.Mutex
	next time.Time
}

// Work is a book as a search result: one work covers every edition of it.
type Work struct {
	ID               string // "OL45883W"
	Title            string
	Authors          []string
	FirstPublishYear int
	CoverID          int
	EditionCount     int
}

// CoverURL returns the work's cover, large, or "".
func (w Work) CoverURL() string {
	if w.CoverID <= 0 {
		return ""
	}
	return "https://covers.openlibrary.org/b/id/" + strconv.Itoa(w.CoverID) + "-L.jpg"
}

// Search looks a book up by title and author.
func (c *Client) Search(ctx context.Context, title, author string) ([]Work, error) {
	q := url.Values{
		"title":  {title},
		"limit":  {"20"},
		"fields": {"key,title,author_name,first_publish_year,cover_i,edition_count"},
	}
	if author != "" {
		q.Set("author", author)
	}
	var out struct {
		Docs []struct {
			Key              string   `json:"key"` // "/works/OL45883W"
			Title            string   `json:"title"`
			AuthorName       []string `json:"author_name"`
			FirstPublishYear int      `json:"first_publish_year"`
			CoverI           int      `json:"cover_i"`
			EditionCount     int      `json:"edition_count"`
		} `json:"docs"`
	}
	if err := c.get(ctx, "/search.json", q, &out); err != nil {
		return nil, err
	}
	works := make([]Work, 0, len(out.Docs))
	for _, d := range out.Docs {
		id := strings.TrimPrefix(d.Key, "/works/")
		if id == d.Key || id == "" {
			continue
		}
		works = append(works, Work{ID: id, Title: d.Title, Authors: d.AuthorName, FirstPublishYear: d.FirstPublishYear,
			CoverID: d.CoverI, EditionCount: d.EditionCount})
	}
	return works, nil
}

// Details is what a work's own page adds to a search result.
type Details struct {
	Description string
	Subjects    []string
}

// WorkDetails returns a work's description and subjects.
func (c *Client) WorkDetails(ctx context.Context, id string) (Details, error) {
	var out struct {
		Description json.RawMessage `json:"description"` // a string, or {"type", "value"}
		Subjects    []string        `json:"subjects"`
	}
	if err := c.get(ctx, "/works/"+url.PathEscape(id)+".json", nil, &out); err != nil {
		return Details{}, err
	}
	d := Details{Subjects: out.Subjects}
	var s string
	if json.Unmarshal(out.Description, &s) == nil {
		d.Description = s
	} else {
		var v struct {
			Value string `json:"value"`
		}
		if json.Unmarshal(out.Description, &v) == nil {
			d.Description = v.Value
		}
	}
	d.Description = CleanDescription(d.Description)
	return d, nil
}

// CleanDescription drops what Open Library descriptions often carry after
// the text itself: a "----------" line followed by links, and "([source][1])"
// notes.
func CleanDescription(s string) string {
	if i := strings.Index(s, "\n----------"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "([source]"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// Genres picks up to max readable genres from a work's subjects, skipping
// library notes ("nyt:…", "Accessible book", "Large type books") and long
// subject headings.
func Genres(subjects []string, max int) []string {
	skip := []string{"accessible book", "protected daisy", "in library", "large type", "open library", "lending library", "reading level", "staff picks", "long now"}
	var out []string
	seen := map[string]bool{}
outer:
	for _, s := range subjects {
		s = strings.TrimSpace(s)
		l := strings.ToLower(s)
		if s == "" || len(s) > 30 || strings.ContainsAny(s, ":=,") || strings.Contains(l, " -- ") || seen[l] {
			continue
		}
		for _, k := range skip {
			if strings.Contains(l, k) {
				continue outer
			}
		}
		seen[l] = true
		out = append(out, strings.ToUpper(s[:1])+s[1:])
		if len(out) == max {
			break
		}
	}
	return out
}

func (c *Client) wait(ctx context.Context) error {
	every := c.Every
	if every == 0 {
		every = time.Second
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

// retries is how many times a 429 or 503 is retried, after 2, 4 and 8 s.
const retries = 3

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	base := c.BaseURL
	if base == "" {
		base = "https://openlibrary.org"
	}
	u := strings.TrimRight(base, "/") + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
			return fmt.Errorf("Open Library: %w", err)
		}
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) && attempt < retries {
			resp.Body.Close()
			select {
			case <-time.After((2 * time.Second) << attempt):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Open Library answered %d", resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
	}
}
