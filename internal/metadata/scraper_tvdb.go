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

// TVDBScraper provides metadata from TheTVDB.com for TV shows.
// Requires a user-provided API key stored in system_settings.
type TVDBScraper struct {
	apiKey string
	client *http.Client
	token  string // JWT auth token
}

func NewTVDBScraper(apiKey string) *TVDBScraper {
	return &TVDBScraper{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *TVDBScraper) Name() string { return "tvdb" }

// authenticate gets a JWT token from TVDB API v4.
func (s *TVDBScraper) authenticate() error {
	if s.token != "" {
		return nil // already authenticated
	}

	body := fmt.Sprintf(`{"apikey":"%s"}`, s.apiKey)
	resp, err := s.client.Post("https://api4.thetvdb.com/v4/login", "application/json", strings.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("TVDB auth failed with status %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	s.token = result.Data.Token
	return nil
}

// tvdbRequest makes an authenticated request to the TVDB API.
func (s *TVDBScraper) tvdbRequest(endpoint string) (*http.Response, error) {
	if err := s.authenticate(); err != nil {
		return nil, fmt.Errorf("TVDB auth: %w", err)
	}

	reqURL := "https://api4.thetvdb.com/v4" + endpoint
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/json")

	return s.client.Do(req)
}

func (s *TVDBScraper) Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TVDB API key not configured")
	}

	searchType := "series"
	if mediaType == models.MediaTypeMovies || mediaType == models.MediaTypeAdultMovies {
		searchType = "movie"
	}

	endpoint := fmt.Sprintf("/search?query=%s&type=%s", url.QueryEscape(query), searchType)
	if year != nil {
		endpoint += fmt.Sprintf("&year=%d", *year)
	}

	resp, err := s.tvdbRequest(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TVDB search returned %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			TVDBID      string `json:"tvdb_id"`
			ObjectID    string `json:"objectID"`
			Name        string `json:"name"`
			Overview    string `json:"overview"`
			ImageURL    string `json:"image_url"`
			Year        string `json:"year"`
			PrimaryType string `json:"primary_type"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var matches []*models.MetadataMatch
	for _, r := range result.Data {
		var yr *int
		if r.Year != "" {
			y := 0
			fmt.Sscanf(r.Year, "%d", &y)
			if y > 0 {
				yr = &y
			}
		}
		overview := r.Overview
		var posterURL *string
		if r.ImageURL != "" {
			posterURL = &r.ImageURL
		}

		// Use tvdb_id or objectID
		externalID := r.TVDBID
		if externalID == "" {
			externalID = r.ObjectID
		}

		matches = append(matches, &models.MetadataMatch{
			Source:      "tvdb",
			ExternalID:  externalID,
			Title:       r.Name,
			Year:        yr,
			Description: &overview,
			PosterURL:   posterURL,
			Confidence:  titleSimilarity(query, r.Name),
		})
	}
	return matches, nil
}

func (s *TVDBScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TVDB API key not configured")
	}

	endpoint := fmt.Sprintf("/series/%s/extended", externalID)
	resp, err := s.tvdbRequest(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TVDB details returned %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			Overview     string `json:"overview"`
			Image        string `json:"image"`
			Year         string `json:"year"`
			FirstAired   string `json:"firstAired"`
			OriginalNetwork struct {
				Name string `json:"name"`
			} `json:"originalNetwork"`
			Genres []struct {
				Name string `json:"name"`
			} `json:"genres"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	r := result.Data
	var yr *int
	if r.Year != "" {
		y := 0
		fmt.Sscanf(r.Year, "%d", &y)
		if y > 0 {
			yr = &y
		}
	}
	overview := r.Overview
	var posterURL *string
	if r.Image != "" {
		posterURL = &r.Image
	}
	var genres []string
	for _, g := range r.Genres {
		genres = append(genres, g.Name)
	}

	return &models.MetadataMatch{
		Source:      "tvdb",
		ExternalID:  fmt.Sprintf("%d", r.ID),
		Title:       r.Name,
		Year:        yr,
		Description: &overview,
		PosterURL:   posterURL,
		Genres:      genres,
		Confidence:  1.0,
	}, nil
}
