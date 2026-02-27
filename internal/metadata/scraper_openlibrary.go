package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
)

type OpenLibraryScraper struct {
	client *http.Client
}

func NewOpenLibraryScraper() *OpenLibraryScraper {
	return &OpenLibraryScraper{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *OpenLibraryScraper) Name() string { return "openlibrary" }

func (s *OpenLibraryScraper) Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	reqURL := fmt.Sprintf("https://openlibrary.org/search.json?q=%s&limit=10", url.QueryEscape(query))

	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Docs []struct {
			Key         string   `json:"key"`
			Title       string   `json:"title"`
			AuthorName  []string `json:"author_name"`
			FirstPublish int     `json:"first_publish_year"`
			CoverI      int      `json:"cover_i"`
			Subject     []string `json:"subject"`
			Publisher   []string `json:"publisher"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var matches []*models.MetadataMatch
	for _, d := range result.Docs {
		var year *int
		if d.FirstPublish > 0 {
			y := d.FirstPublish
			year = &y
		}
		var posterURL *string
		if d.CoverI > 0 {
			p := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", d.CoverI)
			posterURL = &p
		}
		// Build description from author(s)
		var desc *string
		if len(d.AuthorName) > 0 {
			description := "By " + strings.Join(d.AuthorName, ", ")
			desc = &description
		}
		// Use subjects as genres (top 5)
		var genres []string
		limit := 5
		if len(d.Subject) < limit {
			limit = len(d.Subject)
		}
		if limit > 0 {
			genres = d.Subject[:limit]
		}

		conf := titleSimilarity(query, d.Title)
		if year != nil && d.FirstPublish > 0 && *year == d.FirstPublish {
			conf += 0.15
			if conf > 1.0 {
				conf = 1.0
			}
		}

		matches = append(matches, &models.MetadataMatch{
			Source:      "openlibrary",
			ExternalID:  d.Key,
			Title:       d.Title,
			Year:        year,
			Description: desc,
			PosterURL:   posterURL,
			Genres:      genres,
			Confidence:  conf,
		})
	}
	return matches, nil
}

// GetDetails fetches full work details from Open Library including description,
// subjects, covers, and author information.
func (s *OpenLibraryScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	// Fetch work details
	reqURL := fmt.Sprintf("https://openlibrary.org%s.json", externalID)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &models.MetadataMatch{
			Source:     "openlibrary",
			ExternalID: externalID,
			Confidence: 1.0,
		}, nil
	}

	var work struct {
		Key         string      `json:"key"`
		Title       string      `json:"title"`
		Description interface{} `json:"description"` // string or {type, value}
		Subjects    []string    `json:"subjects"`
		Covers      []int       `json:"covers"`
		Authors     []struct {
			Author struct {
				Key string `json:"key"`
			} `json:"author"`
		} `json:"authors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&work); err != nil {
		return nil, err
	}

	match := &models.MetadataMatch{
		Source:     "openlibrary",
		ExternalID: externalID,
		Title:      work.Title,
		Confidence: 1.0,
	}

	// Description (can be string or {type, value} object)
	if work.Description != nil {
		switch v := work.Description.(type) {
		case string:
			match.Description = &v
		case map[string]interface{}:
			if val, ok := v["value"].(string); ok {
				match.Description = &val
			}
		}
	}

	// Cover art (use the largest available)
	if len(work.Covers) > 0 {
		p := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", work.Covers[0])
		match.PosterURL = &p
	}

	// Subjects as genres (top 10)
	if len(work.Subjects) > 0 {
		limit := 10
		if len(work.Subjects) < limit {
			limit = len(work.Subjects)
		}
		match.Genres = work.Subjects[:limit]
	}

	// Fetch author names
	var authorNames []string
	for _, aRef := range work.Authors {
		if aRef.Author.Key == "" {
			continue
		}
		authorResp, err := s.client.Get(fmt.Sprintf("https://openlibrary.org%s.json", aRef.Author.Key))
		if err != nil {
			continue
		}
		var author struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(authorResp.Body).Decode(&author); err == nil && author.Name != "" {
			authorNames = append(authorNames, author.Name)
		}
		authorResp.Body.Close()
		time.Sleep(100 * time.Millisecond) // Rate limit
	}
	if len(authorNames) > 0 && match.Description == nil {
		desc := "By " + strings.Join(authorNames, ", ")
		match.Description = &desc
	}

	return match, nil
}
