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
	Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error)
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

func (s *TMDBScraper) Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	searchType := "movie"
	if mediaType == models.MediaTypeTVShows {
		searchType = "tv"
	}

	reqURL := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s",
		searchType, s.apiKey, url.QueryEscape(query))

	// TMDB filters out adult content by default; include it for adult libraries
	if mediaType == models.MediaTypeAdultMovies {
		reqURL += "&include_adult=true"
	}

	// Pass year to TMDB for more accurate results
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

	return &models.MetadataMatch{
		Source:        "tmdb",
		ExternalID:    fmt.Sprintf("%d", r.ID),
		Title:         r.Title,
		Year:          year,
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
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		Overview    string  `json:"overview"`
		Tagline     string  `json:"tagline"`
		PosterPath  string  `json:"poster_path"`
		BackdropPath string `json:"backdrop_path"`
		ReleaseDate string  `json:"release_date"`
		VoteAverage float64 `json:"vote_average"`
		IMDBId      string  `json:"imdb_id"`
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

	return &DetailsWithCredits{
		Details: &models.MetadataMatch{
			Source:           "tmdb",
			ExternalID:       fmt.Sprintf("%d", r.ID),
			Title:            r.Title,
			Year:             year,
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

	return &models.MetadataMatch{
		Source:      "tmdb",
		ExternalID:  fmt.Sprintf("%d", r.ID),
		Title:       r.Name,
		Year:        year,
		Description: &overview,
		PosterURL:   posterURL,
		Rating:      &rating,
		Genres:      genres,
		IMDBId:      r.ExternalIDs.IMDBId,
		Confidence:  1.0,
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

func (s *MusicBrainzScraper) Search(query string, mediaType models.MediaType, year *int) ([]*models.MetadataMatch, error) {
	// For music, search releases (albums); for music videos, search recordings
	endpoint := "release"
	if mediaType == models.MediaTypeMusicVideos {
		endpoint = "recording"
	}

	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/%s/?query=%s&fmt=json&limit=10",
		endpoint, url.QueryEscape(query))

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/1.0 (https://github.com/JustinTDCT/CineVault)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 503 {
		return nil, fmt.Errorf("MusicBrainz rate limited")
	}

	if endpoint == "recording" {
		var result struct {
			Recordings []struct {
				ID             string `json:"id"`
				Title          string `json:"title"`
				Score          int    `json:"score"`
				Length         int    `json:"length"`
				FirstReleaseDate string `json:"first-release-date"`
				ArtistCredit   []struct {
					Name   string `json:"name"`
					Artist struct {
						Name string `json:"name"`
					} `json:"artist"`
				} `json:"artist-credit"`
				Releases []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Date  string `json:"date"`
				} `json:"releases"`
			} `json:"recordings"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		var matches []*models.MetadataMatch
		for _, r := range result.Recordings {
			// Extract year
			var year *int
			dateStr := r.FirstReleaseDate
			if dateStr != "" && len(dateStr) >= 4 {
				y := 0
				fmt.Sscanf(dateStr[:4], "%d", &y)
				if y > 0 {
					year = &y
				}
			}
			// Build description with artist names
			var desc *string
			var artists []string
			for _, ac := range r.ArtistCredit {
				name := ac.Name
				if ac.Artist.Name != "" {
					name = ac.Artist.Name
				}
				artists = append(artists, name)
			}
			if len(artists) > 0 {
				d := "Recording by " + strings.Join(artists, ", ")
				desc = &d
			}

			matches = append(matches, &models.MetadataMatch{
				Source:      "musicbrainz",
				ExternalID:  r.ID,
				Title:       r.Title,
				Year:        year,
				Description: desc,
				Confidence:  float64(r.Score) / 100.0,
			})
		}
		return matches, nil
	}

	// Release search
	var result struct {
		Releases []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			Score        int    `json:"score"`
			Date         string `json:"date"`
			Country      string `json:"country"`
			ArtistCredit []struct {
				Name   string `json:"name"`
				Artist struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"artist-credit"`
			ReleaseGroup struct {
				PrimaryType string `json:"primary-type"`
			} `json:"release-group"`
			CoverArtArchive struct {
				Front bool `json:"front"`
			} `json:"cover-art-archive"`
		} `json:"releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var matches []*models.MetadataMatch
	for _, r := range result.Releases {
		var year *int
		if r.Date != "" && len(r.Date) >= 4 {
			y := 0
			fmt.Sscanf(r.Date[:4], "%d", &y)
			if y > 0 {
				year = &y
			}
		}

		var desc *string
		var artists []string
		for _, ac := range r.ArtistCredit {
			name := ac.Name
			if ac.Artist.Name != "" {
				name = ac.Artist.Name
			}
			artists = append(artists, name)
		}
		if len(artists) > 0 {
			releaseType := "Release"
			if r.ReleaseGroup.PrimaryType != "" {
				releaseType = r.ReleaseGroup.PrimaryType
			}
			d := releaseType + " by " + strings.Join(artists, ", ")
			desc = &d
		}

		// Cover art URL (if available)
		var posterURL *string
		if r.CoverArtArchive.Front {
			p := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500", r.ID)
			posterURL = &p
		}

		matches = append(matches, &models.MetadataMatch{
			Source:      "musicbrainz",
			ExternalID:  r.ID,
			Title:       r.Title,
			Year:        year,
			Description: desc,
			PosterURL:   posterURL,
			Confidence:  float64(r.Score) / 100.0,
		})
	}
	return matches, nil
}

// GetDetails fetches detailed metadata for a MusicBrainz recording or release.
func (s *MusicBrainzScraper) GetDetails(externalID string) (*models.MetadataMatch, error) {
	// Try as a release first (more common for music libraries), then recording
	match, err := s.getReleaseDetails(externalID)
	if err != nil || match == nil {
		match, err = s.getRecordingDetails(externalID)
	}
	if match == nil {
		return &models.MetadataMatch{
			Source:     "musicbrainz",
			ExternalID: externalID,
			Confidence: 1.0,
		}, nil
	}
	return match, err
}

func (s *MusicBrainzScraper) getReleaseDetails(releaseID string) (*models.MetadataMatch, error) {
	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/release/%s?inc=artists+recordings+tags+release-groups&fmt=json", releaseID)

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/1.0 (https://github.com/JustinTDCT/CineVault)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MusicBrainz release details returned %d", resp.StatusCode)
	}

	var r struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Date         string `json:"date"`
		ArtistCredit []struct {
			Name   string `json:"name"`
			Artist struct {
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"artist-credit"`
		ReleaseGroup struct {
			Title       string `json:"title"`
			PrimaryType string `json:"primary-type"`
		} `json:"release-group"`
		Tags []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"tags"`
		CoverArtArchive struct {
			Front bool `json:"front"`
		} `json:"cover-art-archive"`
		Media []struct {
			TrackCount int `json:"track-count"`
			Tracks     []struct {
				Title  string `json:"title"`
				Length int    `json:"length"`
			} `json:"tracks"`
		} `json:"media"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var year *int
	if r.Date != "" && len(r.Date) >= 4 {
		y := 0
		fmt.Sscanf(r.Date[:4], "%d", &y)
		if y > 0 {
			year = &y
		}
	}

	var desc *string
	var artists []string
	for _, ac := range r.ArtistCredit {
		name := ac.Name
		if ac.Artist.Name != "" {
			name = ac.Artist.Name
		}
		artists = append(artists, name)
	}
	if len(artists) > 0 {
		releaseType := "Release"
		if r.ReleaseGroup.PrimaryType != "" {
			releaseType = r.ReleaseGroup.PrimaryType
		}
		d := releaseType + " by " + strings.Join(artists, ", ")
		desc = &d
	}

	var genres []string
	for _, t := range r.Tags {
		genres = append(genres, t.Name)
	}

	var posterURL *string
	if r.CoverArtArchive.Front {
		p := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500", r.ID)
		posterURL = &p
	}

	return &models.MetadataMatch{
		Source:      "musicbrainz",
		ExternalID:  r.ID,
		Title:       r.Title,
		Year:        year,
		Description: desc,
		PosterURL:   posterURL,
		Genres:      genres,
		Confidence:  1.0,
	}, nil
}

func (s *MusicBrainzScraper) getRecordingDetails(recordingID string) (*models.MetadataMatch, error) {
	reqURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording/%s?inc=artists+releases+tags&fmt=json", recordingID)

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "CineVault/1.0 (https://github.com/JustinTDCT/CineVault)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MusicBrainz recording details returned %d", resp.StatusCode)
	}

	var r struct {
		ID             string `json:"id"`
		Title          string `json:"title"`
		Length         int    `json:"length"`
		FirstReleaseDate string `json:"first-release-date"`
		ArtistCredit   []struct {
			Name   string `json:"name"`
			Artist struct {
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"artist-credit"`
		Releases []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Date  string `json:"date"`
			CoverArtArchive struct {
				Front bool `json:"front"`
			} `json:"cover-art-archive"`
		} `json:"releases"`
		Tags []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var year *int
	if r.FirstReleaseDate != "" && len(r.FirstReleaseDate) >= 4 {
		y := 0
		fmt.Sscanf(r.FirstReleaseDate[:4], "%d", &y)
		if y > 0 {
			year = &y
		}
	}

	var desc *string
	var artists []string
	for _, ac := range r.ArtistCredit {
		name := ac.Name
		if ac.Artist.Name != "" {
			name = ac.Artist.Name
		}
		artists = append(artists, name)
	}
	if len(artists) > 0 {
		d := "Recording by " + strings.Join(artists, ", ")
		desc = &d
	}

	var genres []string
	for _, t := range r.Tags {
		genres = append(genres, t.Name)
	}

	// Try to get cover art from the first release
	var posterURL *string
	for _, rel := range r.Releases {
		if rel.CoverArtArchive.Front {
			p := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500", rel.ID)
			posterURL = &p
			break
		}
	}

	return &models.MetadataMatch{
		Source:      "musicbrainz",
		ExternalID:  r.ID,
		Title:       r.Title,
		Year:        year,
		Description: desc,
		PosterURL:   posterURL,
		Genres:      genres,
		Confidence:  1.0,
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

// ──────────────────── TVDB ────────────────────

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

// ──────────────────── fanart.tv ────────────────────

// FanartTVClient fetches extended artwork from fanart.tv.
type FanartTVClient struct {
	apiKey string
	client *http.Client
}

// FanartArtwork holds the different artwork types available from fanart.tv.
type FanartArtwork struct {
	LogoURL     string
	ClearArtURL string
	BannerURL   string
	DiscURL     string
	ThumbURL    string
	BackdropURL string
}

func NewFanartTVClient(apiKey string) *FanartTVClient {
	return &FanartTVClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetMovieArtwork fetches extended artwork for a movie by TMDB ID.
func (c *FanartTVClient) GetMovieArtwork(tmdbID string) (*FanartArtwork, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("fanart.tv API key not configured")
	}

	reqURL := fmt.Sprintf("https://webservice.fanart.tv/v3/movies/%s?api_key=%s", tmdbID, c.apiKey)
	resp, err := c.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fanart.tv returned %d", resp.StatusCode)
	}

	var result struct {
		HDMovieLogos    []fanartImage `json:"hdmovielogo"`
		MovieLogos      []fanartImage `json:"movielogo"`
		HDClearArt      []fanartImage `json:"hdmovieclearart"`
		MovieClearArt   []fanartImage `json:"movieclearart"`
		MovieBanners    []fanartImage `json:"moviebanner"`
		MovieDiscs      []fanartImage `json:"moviedisc"`
		MovieThumbs     []fanartImage `json:"moviethumb"`
		MovieBackgrounds []fanartImage `json:"moviebackground"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	art := &FanartArtwork{}
	art.LogoURL = firstFanartURL(result.HDMovieLogos, result.MovieLogos)
	art.ClearArtURL = firstFanartURL(result.HDClearArt, result.MovieClearArt)
	art.BannerURL = firstFanartURL(result.MovieBanners)
	art.DiscURL = firstFanartURL(result.MovieDiscs)
	art.ThumbURL = firstFanartURL(result.MovieThumbs)
	art.BackdropURL = firstFanartURL(result.MovieBackgrounds)

	return art, nil
}

// GetTVArtwork fetches extended artwork for a TV show by TVDB ID.
func (c *FanartTVClient) GetTVArtwork(tvdbID string) (*FanartArtwork, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("fanart.tv API key not configured")
	}

	reqURL := fmt.Sprintf("https://webservice.fanart.tv/v3/tv/%s?api_key=%s", tvdbID, c.apiKey)
	resp, err := c.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fanart.tv returned %d", resp.StatusCode)
	}

	var result struct {
		HDTVLogos       []fanartImage `json:"hdtvlogo"`
		ClearLogos      []fanartImage `json:"clearlogo"`
		HDClearArt      []fanartImage `json:"hdclearart"`
		ClearArt        []fanartImage `json:"clearart"`
		TVBanners       []fanartImage `json:"tvbanner"`
		TVThumbs        []fanartImage `json:"tvthumb"`
		ShowBackgrounds []fanartImage `json:"showbackground"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	art := &FanartArtwork{}
	art.LogoURL = firstFanartURL(result.HDTVLogos, result.ClearLogos)
	art.ClearArtURL = firstFanartURL(result.HDClearArt, result.ClearArt)
	art.BannerURL = firstFanartURL(result.TVBanners)
	art.ThumbURL = firstFanartURL(result.TVThumbs)
	art.BackdropURL = firstFanartURL(result.ShowBackgrounds)

	return art, nil
}

type fanartImage struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Likes string `json:"likes"`
	Lang  string `json:"lang"`
}

// firstFanartURL returns the URL of the first image from multiple preference-ordered slices.
// Prefers English-language images.
func firstFanartURL(imageSets ...[]fanartImage) string {
	// First pass: find an English image
	for _, images := range imageSets {
		for _, img := range images {
			if (img.Lang == "en" || img.Lang == "") && img.URL != "" {
				return img.URL
			}
		}
	}
	// Second pass: any language
	for _, images := range imageSets {
		if len(images) > 0 && images[0].URL != "" {
			return images[0].URL
		}
	}
	return ""
}
