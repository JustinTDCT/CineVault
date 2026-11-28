// Package comicvine matches comic series to Comic Vine volumes
// (https://comicvine.gamespot.com): the series' first year, publisher,
// description and cover. It needs a free API key. Comic Vine asks for a
// unique User-Agent and limits each key to 200 requests an hour per
// resource, so requests are spaced 2 seconds apart.
package comicvine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrBadKey means Comic Vine rejected the API key; every other request
// would fail the same way.
var ErrBadKey = errors.New("Comic Vine rejected the API key (check CINEVAULT_COMICVINE_KEY)")

// Client talks to Comic Vine.
type Client struct {
	BaseURL   string // default https://comicvine.gamespot.com/api
	APIKey    string
	UserAgent string
	HTTP      *http.Client
	Every     time.Duration // default 2 s

	mu   sync.Mutex
	next time.Time
}

// Volume is a comic series.
type Volume struct {
	ID          int
	Name        string
	StartYear   int
	Publisher   string
	Deck        string // a one-line summary
	Description string // plain text
	ImageURL    string
	IssueCount  int
}

// SearchVolumes looks a series up by name.
func (c *Client) SearchVolumes(ctx context.Context, name string) ([]Volume, error) {
	q := url.Values{
		"resources":  {"volume"},
		"query":      {name},
		"limit":      {"20"},
		"field_list": {"id,name,start_year,publisher,deck,description,image,count_of_issues"},
	}
	var out struct {
		Results []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			StartYear string `json:"start_year"`
			Publisher *struct {
				Name string `json:"name"`
			} `json:"publisher"`
			Deck        string `json:"deck"`
			Description string `json:"description"`
			Image       *struct {
				MediumURL   string `json:"medium_url"`
				OriginalURL string `json:"original_url"`
			} `json:"image"`
			CountOfIssues int `json:"count_of_issues"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/search/", q, &out); err != nil {
		return nil, err
	}
	vols := make([]Volume, 0, len(out.Results))
	for _, r := range out.Results {
		v := Volume{ID: r.ID, Name: r.Name, Deck: strings.TrimSpace(r.Deck), Description: PlainText(r.Description), IssueCount: r.CountOfIssues}
		v.StartYear, _ = strconv.Atoi(strings.TrimSpace(r.StartYear))
		if r.Publisher != nil {
			v.Publisher = r.Publisher.Name
		}
		if r.Image != nil {
			v.ImageURL = r.Image.MediumURL
			if v.ImageURL == "" {
				v.ImageURL = r.Image.OriginalURL
			}
		}
		vols = append(vols, v)
	}
	return vols, nil
}

var (
	reTags   = regexp.MustCompile(`(?s)<[^>]*>`)
	reBlocks = regexp.MustCompile(`(?i)</(p|h[1-6]|li|div)>|<br\s*/?>`)
	reBlank  = regexp.MustCompile(`\n{3,}`)
)

// PlainText turns Comic Vine's HTML description into plain text: the first
// part, before any heading such as "Collected Editions" or an issue list,
// at most 1,500 characters.
func PlainText(s string) string {
	if i := strings.Index(strings.ToLower(s), "<h"); i > 0 {
		s = s[:i]
	}
	s = reBlocks.ReplaceAllString(s, "\n\n")
	s = html.UnescapeString(reTags.ReplaceAllString(s, ""))
	s = reBlank.ReplaceAllString(strings.TrimSpace(s), "\n\n")
	if r := []rune(s); len(r) > 1500 {
		s = strings.TrimSpace(string(r[:1500])) + "…"
	}
	return s
}

func (c *Client) wait(ctx context.Context) error {
	every := c.Every
	if every == 0 {
		every = 2 * time.Second
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
		base = "https://comicvine.gamespot.com/api"
	}
	q.Set("api_key", c.APIKey)
	q.Set("format", "json")
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
		// The URL holds the key; say only which service failed.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("Comic Vine: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrBadKey
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("Comic Vine: %w", err)
	}
	var status struct {
		StatusCode int    `json:"status_code"`
		Error      string `json:"error"`
	}
	if json.Unmarshal(body, &status) == nil && status.StatusCode != 0 && status.StatusCode != 1 {
		switch status.StatusCode {
		case 100:
			return ErrBadKey
		case 107:
			return errors.New("Comic Vine's hourly limit is used up; try again in an hour")
		}
		return fmt.Errorf("Comic Vine: %s", status.Error)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Comic Vine answered %d", resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}
