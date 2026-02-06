package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
)

type Scraper interface {
	Search(query string, mediaType models.MediaType) ([]*models.MetadataMatch, error)
	GetDetails(externalID string) (*models.MetadataMatch, error)
	Name() string
}

// ──────────────────── TMDB ────────────────────

type TMDBScraper struct {
	apiKey string
	client *http.Client
}

func NewTMDBScraper(apiKey string) *TMDBScraper {
	return &TMDBScraper{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *TMDBScraper) Name() string { return "tmdb" }

type tmdbSearchResult struct {
	Results []struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		Name        string  `json:"name"`
		Overview    string  `json:"overview"`
		PosterPath  string  `json:"poster_path"`
		ReleaseDate string  `json:"release_date"`
		FirstAirDate string `json:"first_air_date"`
		VoteAverage float64 `json:"vote_average"`
	} `json:"results"`
}

func (s *TMDBScraper) Search(query string, mediaType models.MediaType) ([]*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	searchType := "movie"
	if mediaType == models.MediaTypeTVShows {
		searchType = "tv"
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s",
		searchType, s.apiKey, url.QueryEscape(query))

	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result tmdbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var matches []*models.MetadataMatch
	for _, r := range result.Results {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		dateStr := r.ReleaseDate
		if dateStr == "" {
			dateStr = r.FirstAirDate
		}
		var year *int
		if len(dateStr) >= 4 {
			y := 0
			fmt.Sscanf(dateStr[:4], "%d", &y)
			year = &y
		}
		overview := r.Overview
		var posterURL *string
		if r.PosterPath != "" {
			p := "https://image.tmdb.org/t/p/w500" + r.PosterPath
			posterURL = &p
		}
		rating := r.VoteAverage
		matches = append(matches, &models.MetadataMatch{
			Source:      "tmdb",
			ExternalID:  fmt.Sprintf("%d", r.ID),
			Title:       title,
			Year:        year,
			Description: &overview,
			PosterURL:   posterURL,
			Rating:      &rating,
			Confidence:  0.8,
		})
	}
	return matches, nil
}

func (s *TMDBScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%s?api_key=%s", externalID, s.apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		Overview    string  `json:"overview"`
		PosterPath  string  `json:"poster_path"`
		ReleaseDate string  `json:"release_date"`
		VoteAverage float64 `json:"vote_average"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var year *int
	if len(r.ReleaseDate) >= 4 {
		y := 0
		fmt.Sscanf(r.ReleaseDate[:4], "%d", &y)
		year = &y
	}
	overview := r.Overview
	var posterURL *string
	if r.PosterPath != "" {
		p := "https://image.tmdb.org/t/p/w500" + r.PosterPath
		posterURL = &p
	}
	rating := r.VoteAverage

	return &models.MetadataMatch{
		Source:      "tmdb",
		ExternalID:  fmt.Sprintf("%d", r.ID),
		Title:       r.Title,
		Year:        year,
		Description: &overview,
		PosterURL:   posterURL,
		Rating:      &rating,
		Confidence:  1.0,
	}, nil
}

// ──────────────────── TMDB TV Episode Details ────────────────────

// TMDBEpisode holds metadata for a single TV episode from TMDB.
type TMDBEpisode struct {
	EpisodeNumber int     `json:"episode_number"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
}

// GetTVSeasonEpisodes fetches episode details for a specific season of a TV show from TMDB.
func (s *TMDBScraper) GetTVSeasonEpisodes(tmdbShowID string, seasonNumber int) ([]TMDBEpisode, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/tv/%s/season/%d?api_key=%s",
		tmdbShowID, seasonNumber, s.apiKey)

	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TMDB season request returned %d", resp.StatusCode)
	}

	var result struct {
		Episodes []TMDBEpisode `json:"episodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Episodes, nil
}

// ──────────────────── MusicBrainz ────────────────────

type MusicBrainzScraper struct {
	client *http.Client
}

func NewMusicBrainzScraper() *MusicBrainzScraper {
	return &MusicBrainzScraper{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *MusicBrainzScraper) Name() string { return "musicbrainz" }

func (s *MusicBrainzScraper) Search(query string, mediaType models.MediaType) ([]*models.MetadataMatch, error) {
	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording/?query=%s&fmt=json&limit=10",
		url.QueryEscape(query))

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/0.3.0 (https://github.com/JustinTDCT/CineVault)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Recordings []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Score int    `json:"score"`
		} `json:"recordings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var matches []*models.MetadataMatch
	for _, r := range result.Recordings {
		matches = append(matches, &models.MetadataMatch{
			Source:     "musicbrainz",
			ExternalID: r.ID,
			Title:      r.Title,
			Confidence: float64(r.Score) / 100.0,
		})
	}
	return matches, nil
}

func (s *MusicBrainzScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	return &models.MetadataMatch{
		Source:     "musicbrainz",
		ExternalID: externalID,
		Confidence: 1.0,
	}, nil
}

// ──────────────────── Open Library ────────────────────

type OpenLibraryScraper struct {
	client *http.Client
}

func NewOpenLibraryScraper() *OpenLibraryScraper {
	return &OpenLibraryScraper{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *OpenLibraryScraper) Name() string { return "openlibrary" }

func (s *OpenLibraryScraper) Search(query string, mediaType models.MediaType) ([]*models.MetadataMatch, error) {
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
			p := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", d.CoverI)
			posterURL = &p
		}
		matches = append(matches, &models.MetadataMatch{
			Source:     "openlibrary",
			ExternalID: d.Key,
			Title:      d.Title,
			Year:       year,
			PosterURL:  posterURL,
			Confidence: 0.7,
		})
	}
	return matches, nil
}

func (s *OpenLibraryScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	return &models.MetadataMatch{
		Source:     "openlibrary",
		ExternalID: externalID,
		Confidence: 1.0,
	}, nil
}
