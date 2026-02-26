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
		ID            int     `json:"id"`
		Title         string  `json:"title"`
		Name          string  `json:"name"`
		OriginalTitle string  `json:"original_title"`
		OriginalName  string  `json:"original_name"`
		Overview      string  `json:"overview"`
		PosterPath    string  `json:"poster_path"`
		ReleaseDate   string  `json:"release_date"`
		FirstAirDate  string  `json:"first_air_date"`
		VoteAverage   float64 `json:"vote_average"`
		GenreIDs     []int   `json:"genre_ids"`
	} `json:"results"`
}

func (s *TMDBScraper) Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	matches, err := s.tmdbSearch(query, mediaType, year)
	if err != nil {
		return nil, err
	}

	// Fallback: if year was provided but no results, retry without year
	if len(matches) == 0 && year != nil && *year > 0 {
		matches, err = s.tmdbSearch(query, mediaType, nil)
		if err != nil {
			return nil, err
		}
	}
	return matches, nil
}

func (s *TMDBScraper) tmdbSearch(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	searchType := "movie"
	if mediaType == models.MediaTypeTVShows {
		searchType = "tv"
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s",
		searchType, s.apiKey, url.QueryEscape(query))

	if mediaType == models.MediaTypeAdultMovies {
		reqURL += "&include_adult=true"
	}

	if year != nil && *year > 0 {
		if searchType == "tv" {
			reqURL += fmt.Sprintf("&first_air_date_year=%d", *year)
		} else {
			reqURL += fmt.Sprintf("&year=%d", *year)
		}
	}

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
	for i, r := range result.Results {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		origTitle := r.OriginalTitle
		if origTitle == "" {
			origTitle = r.OriginalName
		}
		dateStr := r.ReleaseDate
		if dateStr == "" {
			dateStr = r.FirstAirDate
		}
		var resultYear *int
		if len(dateStr) >= 4 {
			y := 0
			fmt.Sscanf(dateStr[:4], "%d", &y)
			resultYear = &y
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

		conf := titleSimilarity(query, title)
		if origTitle != "" && origTitle != title {
			if origConf := titleSimilarity(query, origTitle); origConf > conf {
				conf = origConf
			}
		}

		// TMDB returns results in relevance order; small boost for top positions
		if i < 3 {
			conf += 0.05 * float64(3-i) / 3.0
			if conf > 1.0 {
				conf = 1.0
			}
		}

		matches = append(matches, &models.MetadataMatch{
			Source:      "tmdb",
			ExternalID:  fmt.Sprintf("%d", r.ID),
			Title:       title,
			Year:        resultYear,
			Description: &overview,
			PosterURL:   posterURL,
			Rating:      &rating,
			Genres:      genres,
			Confidence:  conf,
		})
	}
	return matches, nil
}

// ──────── TMDB release dates / content rating helpers ────────

type tmdbReleaseDateCountry struct {
	ISO31661     string             `json:"iso_3166_1"`
	ReleaseDates []tmdbReleaseEntry `json:"release_dates"`
}

type tmdbReleaseEntry struct {
	Certification string `json:"certification"`
	Type          int    `json:"type"`
}

// extractUSCertification returns the US MPAA certification (e.g. "PG-13", "R")
// from the TMDB release_dates response. Returns nil if not found.
func extractUSCertification(countries []tmdbReleaseDateCountry) *string {
	for _, c := range countries {
		if c.ISO31661 == "US" {
			for _, rd := range c.ReleaseDates {
				if rd.Certification != "" {
					cert := rd.Certification
					return &cert
				}
			}
		}
	}
	return nil
}

func (s *TMDBScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%s?api_key=%s&append_to_response=release_dates", externalID, s.apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r struct {
		ID            int     `json:"id"`
		Title         string  `json:"title"`
		OriginalTitle string  `json:"original_title"`
		Overview      string  `json:"overview"`
		PosterPath    string  `json:"poster_path"`
		ReleaseDate   string  `json:"release_date"`
		VoteAverage   float64 `json:"vote_average"`
		IMDBId        string  `json:"imdb_id"`
		Genres      []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
		ReleaseDates struct {
			Results []tmdbReleaseDateCountry `json:"results"`
		} `json:"release_dates"`
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

	contentRating := extractUSCertification(r.ReleaseDates.Results)

	var originalTitle *string
	if r.OriginalTitle != "" && r.OriginalTitle != r.Title {
		originalTitle = &r.OriginalTitle
	}
	var releaseDate *string
	if r.ReleaseDate != "" {
		releaseDate = &r.ReleaseDate
	}

	return &models.MetadataMatch{
		Source:        "tmdb",
		ExternalID:    fmt.Sprintf("%d", r.ID),
		Title:         r.Title,
		OriginalTitle: originalTitle,
		Year:          year,
		ReleaseDate:   releaseDate,
		Description:   &overview,
		PosterURL:     posterURL,
		Rating:        &rating,
		Genres:        genres,
		IMDBId:        r.IMDBId,
		ContentRating: contentRating,
		Confidence:    1.0,
	}, nil
}

// DetailsWithCredits bundles movie details and credits from a single TMDB API call.
type DetailsWithCredits struct {
	Details *models.MetadataMatch
	Credits *TMDBCredits
}

// GetDetailsWithCredits fetches movie details + credits in a single TMDB API call
// using append_to_response=credits, halving the number of requests per item.
func (s *TMDBScraper) GetDetailsWithCredits(externalID string) (*DetailsWithCredits, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%s?api_key=%s&append_to_response=credits,release_dates,videos,keywords",
		externalID, s.apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TMDB request returned %d", resp.StatusCode)
	}

	var r struct {
		ID            int     `json:"id"`
		Title         string  `json:"title"`
		OriginalTitle string  `json:"original_title"`
		Overview      string  `json:"overview"`
		Tagline       string  `json:"tagline"`
		PosterPath    string  `json:"poster_path"`
		BackdropPath  string  `json:"backdrop_path"`
		ReleaseDate   string  `json:"release_date"`
		VoteAverage   float64 `json:"vote_average"`
		IMDBId        string  `json:"imdb_id"`
		OriginalLanguage string `json:"original_language"`
		ProductionCountries []struct {
			ISO31661 string `json:"iso_3166_1"`
			Name     string `json:"name"`
		} `json:"production_countries"`
		Genres      []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
		BelongsToCollection *struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			PosterPath   string `json:"poster_path"`
			BackdropPath string `json:"backdrop_path"`
		} `json:"belongs_to_collection"`
		Videos struct {
			Results []struct {
				Type string `json:"type"`
				Site string `json:"site"`
				Key  string `json:"key"`
			} `json:"results"`
		} `json:"videos"`
		Credits      TMDBCredits `json:"credits"`
		ReleaseDates struct {
			Results []tmdbReleaseDateCountry `json:"results"`
		} `json:"release_dates"`
		Keywords struct {
			Keywords []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"keywords"`
		} `json:"keywords"`
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
	var backdropURL *string
	if r.BackdropPath != "" {
		b := "https://image.tmdb.org/t/p/w1280" + r.BackdropPath
		backdropURL = &b
	}
	rating := r.VoteAverage

	var genres []string
	for _, g := range r.Genres {
		genres = append(genres, g.Name)
	}

	contentRating := extractUSCertification(r.ReleaseDates.Results)

	// Build tagline
	var tagline *string
	if r.Tagline != "" {
		tagline = &r.Tagline
	}

	// Build country string from production countries
	var country *string
	if len(r.ProductionCountries) > 0 {
		var countries []string
		for _, c := range r.ProductionCountries {
			countries = append(countries, c.Name)
		}
		c := strings.Join(countries, ", ")
		country = &c
	}

	// Build original language
	var origLang *string
	if r.OriginalLanguage != "" {
		origLang = &r.OriginalLanguage
	}

	// Find YouTube trailer
	var trailerURL *string
	for _, v := range r.Videos.Results {
		if v.Type == "Trailer" && v.Site == "YouTube" && v.Key != "" {
			t := "https://www.youtube.com/watch?v=" + v.Key
			trailerURL = &t
			break
		}
	}

	// Collection data
	var collectionID *int
	var collectionName *string
	if r.BelongsToCollection != nil {
		collectionID = &r.BelongsToCollection.ID
		collectionName = &r.BelongsToCollection.Name
	}

	// Extract keywords
	var keywords []string
	for _, kw := range r.Keywords.Keywords {
		keywords = append(keywords, kw.Name)
	}

	// Original title
	var originalTitle *string
	if r.OriginalTitle != "" && r.OriginalTitle != r.Title {
		originalTitle = &r.OriginalTitle
	}

	// Release date (full date string for storage)
	var releaseDate *string
	if r.ReleaseDate != "" {
		releaseDate = &r.ReleaseDate
	}

	return &DetailsWithCredits{
		Details: &models.MetadataMatch{
			Source:           "tmdb",
			ExternalID:       fmt.Sprintf("%d", r.ID),
			Title:            r.Title,
			OriginalTitle:    originalTitle,
			Year:             year,
			ReleaseDate:      releaseDate,
			Description:      &overview,
			Tagline:          tagline,
			PosterURL:        posterURL,
			BackdropURL:      backdropURL,
			Rating:           &rating,
			Genres:           genres,
			IMDBId:           r.IMDBId,
			ContentRating:    contentRating,
			OriginalLanguage: origLang,
			Country:          country,
			TrailerURL:       trailerURL,
			CollectionID:     collectionID,
			CollectionName:   collectionName,
			Keywords:         keywords,
			Confidence:       1.0,
		},
		Credits: &r.Credits,
	}, nil
}

// GetTVDetails fetches TV show details from TMDB including external IDs (for IMDB ID).
func (s *TMDBScraper) GetTVDetails(externalID string) (*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/tv/%s?api_key=%s&append_to_response=external_ids",
		externalID, s.apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		OriginalName string  `json:"original_name"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		FirstAirDate string  `json:"first_air_date"`
		VoteAverage  float64 `json:"vote_average"`
		ExternalIDs  struct {
			IMDBId string `json:"imdb_id"`
		} `json:"external_ids"`
		Genres []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var year *int
	if len(r.FirstAirDate) >= 4 {
		y := 0
		fmt.Sscanf(r.FirstAirDate[:4], "%d", &y)
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

	var originalTitle *string
	if r.OriginalName != "" && r.OriginalName != r.Name {
		originalTitle = &r.OriginalName
	}
	var releaseDate *string
	if r.FirstAirDate != "" {
		releaseDate = &r.FirstAirDate
	}

	return &models.MetadataMatch{
		Source:        "tmdb",
		ExternalID:    fmt.Sprintf("%d", r.ID),
		Title:         r.Name,
		OriginalTitle: originalTitle,
		Year:          year,
		ReleaseDate:   releaseDate,
		Description:   &overview,
		PosterURL:     posterURL,
		Rating:        &rating,
		Genres:        genres,
		IMDBId:        r.ExternalIDs.IMDBId,
		Confidence:    1.0,
	}, nil
}

// ──────────────────── TMDB Credits ────────────────────

// TMDBCastMember represents a cast member from TMDB credits.
type TMDBCastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}

// TMDBCrewMember represents a crew member from TMDB credits.
type TMDBCrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

// TMDBCredits holds the cast and crew from TMDB.
type TMDBCredits struct {
	Cast []TMDBCastMember `json:"cast"`
	Crew []TMDBCrewMember `json:"crew"`
}

// GetMovieCredits fetches the cast and crew for a movie from TMDB.
func (s *TMDBScraper) GetMovieCredits(tmdbID string) (*TMDBCredits, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%s/credits?api_key=%s", tmdbID, s.apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TMDB credits request returned %d", resp.StatusCode)
	}

	var credits TMDBCredits
	if err := json.NewDecoder(resp.Body).Decode(&credits); err != nil {
		return nil, err
	}
	return &credits, nil
}

// GetTVCredits fetches the aggregate cast and crew for a TV show from TMDB.
func (s *TMDBScraper) GetTVCredits(tmdbID string) (*TMDBCredits, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/tv/%s/aggregate_credits?api_key=%s", tmdbID, s.apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TMDB TV credits request returned %d", resp.StatusCode)
	}

	// TV aggregate_credits has a slightly different structure for cast roles
	var raw struct {
		Cast []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			ProfilePath string `json:"profile_path"`
			Order       int    `json:"order"`
			Roles       []struct {
				Character string `json:"character"`
			} `json:"roles"`
		} `json:"cast"`
		Crew []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			ProfilePath string `json:"profile_path"`
			Department  string `json:"department"`
			Jobs        []struct {
				Job string `json:"job"`
			} `json:"jobs"`
		} `json:"crew"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	credits := &TMDBCredits{}
	for _, c := range raw.Cast {
		character := ""
		if len(c.Roles) > 0 {
			character = c.Roles[0].Character
		}
		credits.Cast = append(credits.Cast, TMDBCastMember{
			ID:          c.ID,
			Name:        c.Name,
			Character:   character,
			ProfilePath: c.ProfilePath,
			Order:       c.Order,
		})
	}
	for _, c := range raw.Crew {
		job := ""
		if len(c.Jobs) > 0 {
			job = c.Jobs[0].Job
		}
		credits.Crew = append(credits.Crew, TMDBCrewMember{
			ID:          c.ID,
			Name:        c.Name,
			Job:         job,
			Department:  c.Department,
			ProfilePath: c.ProfilePath,
		})
	}
	return credits, nil
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
