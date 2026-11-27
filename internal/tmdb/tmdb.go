// Package tmdb is a small client for The Movie Database (TMDb) API v3.
//
// Data and images come from TMDb (https://www.themoviedb.org). CineVault
// must show TMDb attribution wherever this data appears.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

// DefaultBaseURL is TMDb's API endpoint.
const DefaultBaseURL = "https://api.themoviedb.org/3"

// ImageBaseURL is TMDb's image CDN; append a size and a path, e.g. "w500" + "/abc.jpg".
const ImageBaseURL = "https://image.tmdb.org/t/p/"

// Errors returned by the client.
var (
	ErrUnauthorized = errors.New("TMDb rejected the API token")
	ErrNotFound     = errors.New("not found on TMDb")
)

// Client calls the TMDb API.
type Client struct {
	// Token is a TMDb "API Read Access Token" (sent as a Bearer token).
	Token    string
	BaseURL  string // defaults to DefaultBaseURL
	Language string // defaults to "en-US"
	HTTP     *http.Client

	// MinInterval spaces requests out; TMDb allows roughly 40-50 per second.
	MinInterval time.Duration

	mu   sync.Mutex
	last time.Time
}

// New creates a client for token.
func New(token string) *Client {
	return &Client{Token: token, MinInterval: 25 * time.Millisecond}
}

// SearchResult is one movie from a search.
type SearchResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"` // "1979-05-25", may be empty
	Popularity    float64 `json:"popularity"`
}

// Year returns the release year, or 0.
func (r SearchResult) Year() int { return yearOf(r.ReleaseDate) }

// Movie is a movie's details.
type Movie struct {
	ID            int     `json:"id"`
	IMDbID        string  `json:"imdb_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	Tagline       string  `json:"tagline"`
	ReleaseDate   string  `json:"release_date"`
	Runtime       int     `json:"runtime"` // minutes
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	VoteAverage   float64 `json:"vote_average"`
	Genres        []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Collection *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"belongs_to_collection"`
	Credits Credits `json:"credits"`
}

// Credits is who is in and who made a movie or show.
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// CastMember is one actor and their role.
type CastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}

// CrewMember is one crew credit.
type CrewMember struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

// MaxCast is how many cast members are kept for a title.
const MaxCast = 15

// TopCast returns the first MaxCast cast members in billing order; never nil.
func (c Credits) TopCast() []CastMember {
	out := append([]CastMember{}, c.Cast...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	if len(out) > MaxCast {
		out = out[:MaxCast]
	}
	return out
}

// Directors lists the directors, each once; never nil.
func (c Credits) Directors() []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range c.Crew {
		if m.Job == "Director" && !seen[m.Name] {
			seen[m.Name] = true
			out = append(out, m.Name)
		}
	}
	return out
}

// Year returns the release year, or 0.
func (m Movie) Year() int { return yearOf(m.ReleaseDate) }

// GenreNames lists the genres; never nil.
func (m Movie) GenreNames() []string {
	out := make([]string, 0, len(m.Genres))
	for _, g := range m.Genres {
		out = append(out, g.Name)
	}
	return out
}

// SearchMovie searches by title, optionally narrowed to a release year (0 = any).
func (c *Client) SearchMovie(ctx context.Context, query string, year int) ([]SearchResult, error) {
	q := url.Values{"query": {query}, "include_adult": {"false"}}
	if year > 0 {
		q.Set("primary_release_year", strconv.Itoa(year))
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	if err := c.get(ctx, "/search/movie", q, &out); err != nil {
		return nil, err
	}
	if out.Results == nil {
		out.Results = []SearchResult{}
	}
	return out.Results, nil
}

// Movie fetches a movie's details by TMDb ID.
func (c *Client) Movie(ctx context.Context, id int) (Movie, error) {
	var m Movie
	err := c.get(ctx, "/movie/"+strconv.Itoa(id), url.Values{"append_to_response": {"credits"}}, &m)
	return m, err
}

func (c *Client) get(ctx context.Context, path string, q url.Values, dest any) error {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	lang := c.Language
	if lang == "" {
		lang = "en-US"
	}
	q.Set("language", lang)
	u := base + path + "?" + q.Encode()

	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Accept", "application/json")

		hc := c.HTTP
		if hc == nil {
			hc = &http.Client{Timeout: 20 * time.Second}
		}
		resp, err := hc.Do(req)
		if err != nil {
			return fmt.Errorf("TMDb request failed: %w", err)
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			err := json.NewDecoder(resp.Body).Decode(dest)
			resp.Body.Close()
			if err != nil {
				return fmt.Errorf("TMDb response: %w", err)
			}
			return nil
		case resp.StatusCode == http.StatusTooManyRequests && attempt < 3:
			resp.Body.Close()
			delay := time.Second
			if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 && s <= 30 {
				delay = time.Duration(s) * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		case resp.StatusCode == http.StatusUnauthorized:
			resp.Body.Close()
			return ErrUnauthorized
		case resp.StatusCode == http.StatusNotFound:
			resp.Body.Close()
			return ErrNotFound
		default:
			resp.Body.Close()
			return fmt.Errorf("TMDb returned HTTP %d", resp.StatusCode)
		}
	}
}

// wait spaces requests at least MinInterval apart.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	next := c.last.Add(c.MinInterval)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	c.last = next
	c.mu.Unlock()
	if d := time.Until(next); d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	return nil
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}
