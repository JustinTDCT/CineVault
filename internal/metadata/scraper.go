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

// tmdbGenreMap maps TMDB genre IDs to human-readable names (movies).
var tmdbGenreMap = map[int]string{
	28: "Action", 12: "Adventure", 16: "Animation", 35: "Comedy", 80: "Crime",
	99: "Documentary", 18: "Drama", 10751: "Family", 14: "Fantasy", 36: "History",
	27: "Horror", 10402: "Music", 9648: "Mystery", 10749: "Romance",
	878: "Science Fiction", 10770: "TV Movie", 53: "Thriller", 10752: "War", 37: "Western",
	// TV-specific
	10759: "Action & Adventure", 10762: "Kids", 10763: "News", 10764: "Reality",
	10765: "Sci-Fi & Fantasy", 10766: "Soap", 10767: "Talk", 10768: "War & Politics",
}

type tmdbSearchResult struct {
	Results []struct {
		ID           int     `json:"id"`
		Title        string  `json:"title"`
		Name         string  `json:"name"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		ReleaseDate  string  `json:"release_date"`
		FirstAirDate string  `json:"first_air_date"`
		VoteAverage  float64 `json:"vote_average"`
		GenreIDs     []int   `json:"genre_ids"`
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
		var genres []string
		for _, gid := range r.GenreIDs {
			if name, ok := tmdbGenreMap[gid]; ok {
				genres = append(genres, name)
			}
		}
		matches = append(matches, &models.MetadataMatch{
			Source:      "tmdb",
			ExternalID:  fmt.Sprintf("%d", r.ID),
			Title:       title,
			Year:        year,
			Description: &overview,
			PosterURL:   posterURL,
			Rating:      &rating,
			Genres:      genres,
			Confidence:  titleSimilarity(query, title),
		})
	}
	return matches, nil
}

// titleSimilarity computes a confidence score between a search query and a result title.
// Exact match = 1.0, substring containment gets partial credit, otherwise uses word overlap.
func titleSimilarity(query, result string) float64 {
	q := strings.ToLower(strings.TrimSpace(query))
	r := strings.ToLower(strings.TrimSpace(result))

	if q == r {
		return 1.0
	}

	// If one is a strict prefix/substring of the other, high but not perfect
	if q == r || strings.HasPrefix(r, q+" ") || strings.HasPrefix(q, r+" ") {
		return 0.9
	}

	// Word-overlap scoring
	qWords := strings.Fields(q)
	rWords := strings.Fields(r)
	if len(qWords) == 0 || len(rWords) == 0 {
		return 0.0
	}

	rSet := make(map[string]bool, len(rWords))
	for _, w := range rWords {
		rSet[w] = true
	}

	matches := 0
	for _, w := range qWords {
		if rSet[w] {
			matches++
		}
	}

	// Jaccard-like: matched / total unique words
	total := len(qWords)
	if len(rWords) > total {
		total = len(rWords)
	}
	score := float64(matches) / float64(total)

	// Penalize if result has many extra words (e.g. query="Cloverfield" vs result="10 Cloverfield Lane")
	if len(rWords) > len(qWords) {
		score *= float64(len(qWords)) / float64(len(rWords))
	}

	return score
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
		IMDBId      string  `json:"imdb_id"`
		Genres      []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
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

	var genres []string
	for _, g := range r.Genres {
		genres = append(genres, g.Name)
	}

	return &models.MetadataMatch{
		Source:      "tmdb",
		ExternalID:  fmt.Sprintf("%d", r.ID),
		Title:       r.Title,
		Year:        year,
		Description: &overview,
		PosterURL:   posterURL,
		Rating:      &rating,
		Genres:      genres,
		IMDBId:      r.IMDBId,
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

// TMDBSeasonResult holds season-level metadata plus episodes from TMDB.
type TMDBSeasonResult struct {
	Name       string        `json:"name"`
	Overview   string        `json:"overview"`
	PosterPath string        `json:"poster_path"`
	AirDate    string        `json:"air_date"`
	Episodes   []TMDBEpisode `json:"episodes"`
}

// GetTVSeasonDetails fetches season details including poster and episodes from TMDB.
func (s *TMDBScraper) GetTVSeasonDetails(tmdbShowID string, seasonNumber int) (*TMDBSeasonResult, error) {
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

	var result TMDBSeasonResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
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

// ──────────────────── OMDb ────────────────────

// OMDbRatings holds the ratings fetched from the OMDb API.
type OMDbRatings struct {
	IMDBRating    *float64 // e.g. 7.8
	RTScore       *int     // Rotten Tomatoes critic % (0-100)
	AudienceScore *int     // Rotten Tomatoes audience % (0-100) – mapped from "Internet Movie Database" audience or Metacritic
}

// FetchOMDbRatings calls the OMDb API with the given IMDB ID and API key,
// returning IMDB rating, Rotten Tomatoes score, and audience score.
func FetchOMDbRatings(imdbID, apiKey string) (*OMDbRatings, error) {
	if imdbID == "" || apiKey == "" {
		return nil, fmt.Errorf("imdb_id and api_key are required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	reqURL := fmt.Sprintf("http://www.omdbapi.com/?i=%s&apikey=%s", url.QueryEscape(imdbID), url.QueryEscape(apiKey))

	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var omdb struct {
		Response string `json:"Response"`
		Error    string `json:"Error"`
		IMDBRating string `json:"imdbRating"`
		Ratings []struct {
			Source string `json:"Source"`
			Value  string `json:"Value"`
		} `json:"Ratings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&omdb); err != nil {
		return nil, err
	}
	if omdb.Response == "False" {
		return nil, fmt.Errorf("OMDb error: %s", omdb.Error)
	}

	result := &OMDbRatings{}

	// Parse IMDB rating (e.g. "7.8")
	if omdb.IMDBRating != "" && omdb.IMDBRating != "N/A" {
		var r float64
		fmt.Sscanf(omdb.IMDBRating, "%f", &r)
		result.IMDBRating = &r
	}

	// Parse ratings array
	for _, rating := range omdb.Ratings {
		switch rating.Source {
		case "Rotten Tomatoes":
			// Value is like "92%"
			var pct int
			fmt.Sscanf(rating.Value, "%d%%", &pct)
			result.RTScore = &pct
		case "Metacritic":
			// Value is like "76/100" – use as audience score fallback
			if result.AudienceScore == nil {
				var score int
				fmt.Sscanf(rating.Value, "%d/", &score)
				result.AudienceScore = &score
			}
		}
	}

	return result, nil
}
