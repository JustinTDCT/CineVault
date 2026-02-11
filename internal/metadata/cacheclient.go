package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
)

// CacheClient talks to the CineVault Metadata Cache Server.
type CacheClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewCacheClient creates a client for the cache server.
func NewCacheClient(baseURL, apiKey string) *CacheClient {
	return &CacheClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// ── Cache Server Response Types ──

type cacheLookupRequest struct {
	Title        string `json:"title"`
	Year         *int   `json:"year,omitempty"`
	Type         string `json:"type"`
	FileHash     string `json:"file_hash,omitempty"`
	IncludeAdult bool   `json:"include_adult,omitempty"`
}

type cacheEntry struct {
	TMDBID          int      `json:"tmdb_id"`
	IMDBID          *string  `json:"imdb_id,omitempty"`
	MediaType       string   `json:"media_type"`
	Title           string   `json:"title"`
	OriginalTitle   *string  `json:"original_title,omitempty"`
	SortTitle       *string  `json:"sort_title,omitempty"`
	Year            *int     `json:"year,omitempty"`
	ReleaseDate     *string  `json:"release_date,omitempty"`
	Description     *string  `json:"description,omitempty"`
	PosterURL       *string  `json:"poster_url,omitempty"`
	BackdropURL     *string  `json:"backdrop_url,omitempty"`
	Genres          *string  `json:"genres,omitempty"`
	CastCrew        *string  `json:"cast_crew,omitempty"`
	ContentRating   *string  `json:"content_rating,omitempty"`
	Runtime         *int     `json:"runtime,omitempty"`
	// Extended metadata
	Tagline          *string `json:"tagline,omitempty"`
	OriginalLanguage *string `json:"original_language,omitempty"`
	Country          *string `json:"country,omitempty"`
	TrailerURL       *string `json:"trailer_url,omitempty"`
	LogoURL          *string `json:"logo_url,omitempty"`
	BannerURL        *string `json:"banner_url,omitempty"`
	TVDBID           *int    `json:"tvdb_id,omitempty"`
	CollectionID     *int    `json:"collection_id,omitempty"`
	CollectionName   *string `json:"collection_name,omitempty"`
	// Source tracking
	Source     string  `json:"source,omitempty"`
	ExternalID *string `json:"external_id,omitempty"`
	// Multi-source aggregated arrays
	PosterURLs   *string `json:"poster_urls,omitempty"`
	BackdropURLs *string `json:"backdrop_urls,omitempty"`
	Descriptions *string `json:"descriptions,omitempty"`
	LogoURLs     *string `json:"logo_urls,omitempty"`
	// Ratings
	IMDBRating      *float64 `json:"imdb_rating,omitempty"`
	RTCriticScore   *int     `json:"rt_critic_score,omitempty"`
	RTAudienceScore *int     `json:"rt_audience_score,omitempty"`
	OMDbEnriched    bool     `json:"omdb_enriched"`
	FanartEnriched  bool     `json:"fanart_enriched"`
	// Keywords from TMDB
	Keywords        *string  `json:"keywords,omitempty"`
}

type cacheLookupResponse struct {
	Hit        bool        `json:"hit"`
	Confidence float64     `json:"confidence"`
	Entry      *cacheEntry `json:"entry,omitempty"`
	Source     string      `json:"source"`
}

type cacheContributeRequest struct {
	TMDBID           int      `json:"tmdb_id"`
	IMDBID           *string  `json:"imdb_id,omitempty"`
	MediaType        string   `json:"media_type"`
	Title            string   `json:"title"`
	OriginalTitle    *string  `json:"original_title,omitempty"`
	Year             *int     `json:"year,omitempty"`
	ReleaseDate      *string  `json:"release_date,omitempty"`
	Description      *string  `json:"description,omitempty"`
	PosterURL        *string  `json:"poster_url,omitempty"`
	BackdropURL      *string  `json:"backdrop_url,omitempty"`
	Genres           *string  `json:"genres,omitempty"`
	ContentRating    *string  `json:"content_rating,omitempty"`
	CastCrew         *string  `json:"cast_crew,omitempty"`
	Runtime          *int     `json:"runtime,omitempty"`
	Tagline          *string  `json:"tagline,omitempty"`
	OriginalLanguage *string  `json:"original_language,omitempty"`
	Country          *string  `json:"country,omitempty"`
	TrailerURL       *string  `json:"trailer_url,omitempty"`
	LogoURL          *string  `json:"logo_url,omitempty"`
	BannerURL        *string  `json:"banner_url,omitempty"`
	TVDBID           *int     `json:"tvdb_id,omitempty"`
	CollectionID     *int     `json:"collection_id,omitempty"`
	CollectionName   *string  `json:"collection_name,omitempty"`
	IMDBRating       *float64 `json:"imdb_rating,omitempty"`
	RTCriticScore    *int     `json:"rt_critic_score,omitempty"`
	RTAudienceScore  *int     `json:"rt_audience_score,omitempty"`
	Keywords         *string  `json:"keywords,omitempty"`
}

// ── Cache Lookup Result ──

// CacheLookupResult holds everything returned from the cache server,
// already converted to CineVault types.
type CacheLookupResult struct {
	Hit        bool
	Confidence float64
	Source     string // "hit", "hash_hit", "tmdb", "miss"

	// Metadata match (same format as TMDB scraper returns)
	Match *models.MetadataMatch

	// Ratings from OMDb (if cache server already enriched)
	Ratings *OMDbRatings

	// Genre names parsed from the cache
	Genres []string

	// Cast/crew JSON from cache (avoids separate TMDB credits call)
	CastCrewJSON *string

	// Runtime in minutes from cache
	Runtime *int

	// ExternalIDsJSON is a ready-to-store JSON string of all external source IDs
	ExternalIDsJSON *string

	// Extended metadata from cache
	LogoURL       *string
	BannerURL     *string
	BackdropURL   *string
	OriginalTitle *string
	SortTitle     *string
	ReleaseDate   *string

	// Keywords from TMDB (for mood tagging)
	Keywords []string
}

// ── Lookup ──

// mediaTypeToCacheType maps CineVault media types to cache server media types.
func mediaTypeToCacheType(mediaType models.MediaType) string {
	switch mediaType {
	case models.MediaTypeTVShows:
		return "tv"
	case models.MediaTypeMusic:
		return "music"
	case models.MediaTypeMusicVideos:
		return "music_video"
	case models.MediaTypeAudiobooks:
		return "audiobook"
	default:
		return "movie"
	}
}

// Lookup queries the cache server for metadata. Returns nil if the cache
// server is unreachable or returns a miss.
func (c *CacheClient) Lookup(title string, year *int, mediaType models.MediaType) *CacheLookupResult {
	cacheType := mediaTypeToCacheType(mediaType)

	reqBody := cacheLookupRequest{
		Title:        title,
		Year:         year,
		Type:         cacheType,
		IncludeAdult: mediaType == models.MediaTypeAdultMovies,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[cache-client] marshal error: %v", err)
		return nil
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/lookup", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("[cache-client] request error: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("[cache-client] unreachable: %v (falling back to direct TMDB)", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[cache-client] returned %d (falling back to direct TMDB)", resp.StatusCode)
		return nil
	}

	var lookupResp cacheLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&lookupResp); err != nil {
		log.Printf("[cache-client] decode error: %v", err)
		return nil
	}

	if !lookupResp.Hit || lookupResp.Entry == nil {
		return nil
	}

	entry := lookupResp.Entry
	result := &CacheLookupResult{
		Hit:        true,
		Confidence: lookupResp.Confidence,
		Source:     lookupResp.Source,
	}

	// Determine the metadata source from the cache response
	metadataSource := "tmdb"
	if lookupResp.Source == "porndb" || lookupResp.Source == "musicbrainz" || lookupResp.Source == "openlibrary" {
		metadataSource = lookupResp.Source
	}

	// Convert cache entry to MetadataMatch
	match := &models.MetadataMatch{
		Source:     metadataSource,
		ExternalID: fmt.Sprintf("%d", entry.TMDBID),
		Title:      entry.Title,
		Year:       entry.Year,
		Confidence: lookupResp.Confidence,
	}

	// Use preferred description from multi-source array if available
	match.Description = pickPreferred(entry.Descriptions, metadataSource, entry.Description)

	// Use preferred poster from multi-source array if available
	match.PosterURL = pickPreferredURL(entry.PosterURLs, metadataSource, entry.PosterURL)

	// Use preferred backdrop
	match.BackdropURL = pickPreferredURL(entry.BackdropURLs, metadataSource, entry.BackdropURL)

	if entry.IMDBID != nil {
		match.IMDBId = *entry.IMDBID
	}
	if entry.ContentRating != nil {
		match.ContentRating = entry.ContentRating
	}

	// Extended metadata fields
	match.Tagline = entry.Tagline
	match.OriginalLanguage = entry.OriginalLanguage
	match.Country = entry.Country
	match.TrailerURL = entry.TrailerURL
	match.CollectionID = entry.CollectionID
	match.CollectionName = entry.CollectionName

	// Parse genres from JSON string
	if entry.Genres != nil && *entry.Genres != "" {
		var genres []string
		if err := json.Unmarshal([]byte(*entry.Genres), &genres); err == nil {
			match.Genres = genres
			result.Genres = genres
		}
	}

	// Parse keywords from JSON string
	if entry.Keywords != nil && *entry.Keywords != "" {
		var keywords []string
		if err := json.Unmarshal([]byte(*entry.Keywords), &keywords); err == nil {
			match.Keywords = keywords
			result.Keywords = keywords
		}
	}

	result.Match = match

	// Pass through cast/crew JSON so scanner can use it directly
	result.CastCrewJSON = entry.CastCrew

	// Pass through runtime
	result.Runtime = entry.Runtime

	// Pass through artwork URLs from cache (logo, banner, backdrop)
	result.LogoURL = entry.LogoURL
	result.BannerURL = entry.BannerURL
	result.BackdropURL = match.BackdropURL

	// Pass through title variants and release date
	result.OriginalTitle = entry.OriginalTitle
	result.SortTitle = entry.SortTitle
	result.ReleaseDate = entry.ReleaseDate

	// Extract ratings (now enriched inline by cache server, not just OMDb flag)
	if entry.IMDBRating != nil || entry.RTCriticScore != nil || entry.RTAudienceScore != nil {
		result.Ratings = &OMDbRatings{
			IMDBRating:    entry.IMDBRating,
			RTScore:       entry.RTCriticScore,
			AudienceScore: entry.RTAudienceScore,
		}
	}

	// Build external IDs JSON for storage
	result.ExternalIDsJSON = buildExternalIDsJSON(entry, true)

	return result
}

// ── External ID helpers ──

// buildExternalIDsJSON constructs a JSON string of all external source IDs from a cache entry.
func buildExternalIDsJSON(entry *cacheEntry, cacheServer bool) *string {
	ids := map[string]interface{}{
		"source":       entry.Source,
		"cache_server": cacheServer,
	}
	if entry.TMDBID != 0 {
		ids["tmdb_id"] = fmt.Sprintf("%d", entry.TMDBID)
	}
	if entry.IMDBID != nil && *entry.IMDBID != "" {
		ids["imdb_id"] = *entry.IMDBID
	}
	if entry.TVDBID != nil && *entry.TVDBID != 0 {
		ids["tvdb_id"] = fmt.Sprintf("%d", *entry.TVDBID)
	}
	// The external_id field holds the source-specific ID (PornDB scene ID, MusicBrainz UUID, etc.)
	if entry.ExternalID != nil && *entry.ExternalID != "" {
		// Map source to a named key
		switch entry.Source {
		case "porndb":
			ids["tpdb_id"] = *entry.ExternalID
		case "musicbrainz":
			ids["musicbrainz_id"] = *entry.ExternalID
		case "openlibrary":
			ids["openlibrary_id"] = *entry.ExternalID
		default:
			if entry.TMDBID != 0 {
				ids["tmdb_id"] = *entry.ExternalID
			}
		}
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return nil
	}
	s := string(data)
	return &s
}

// BuildExternalIDsFromMatch constructs a JSON string of external IDs from a MetadataMatch
// (used when metadata is applied via direct scrapers, not the cache server).
func BuildExternalIDsFromMatch(source, externalID, imdbID string, cacheServer bool) *string {
	ids := map[string]interface{}{
		"source":       source,
		"cache_server": cacheServer,
	}
	if externalID != "" {
		switch source {
		case "tmdb":
			ids["tmdb_id"] = externalID
		case "porndb":
			ids["tpdb_id"] = externalID
		case "musicbrainz":
			ids["musicbrainz_id"] = externalID
		case "openlibrary":
			ids["openlibrary_id"] = externalID
		}
	}
	if imdbID != "" {
		ids["imdb_id"] = imdbID
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return nil
	}
	s := string(data)
	return &s
}

// ── Multi-source helpers ──

// sourceItem is used to parse the JSON arrays from the cache server.
type sourceItem struct {
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
	Text   string `json:"text,omitempty"`
}

// pickPreferred selects the best description from a multi-source array.
// Prefers the entry matching preferredSource; falls back to scalar.
func pickPreferred(descriptionsJSON *string, preferredSource string, fallback *string) *string {
	if descriptionsJSON == nil || *descriptionsJSON == "" {
		return fallback
	}
	var items []sourceItem
	if err := json.Unmarshal([]byte(*descriptionsJSON), &items); err != nil {
		return fallback
	}
	// Look for preferred source first
	for _, item := range items {
		if item.Source == preferredSource && item.Text != "" {
			return &item.Text
		}
	}
	// Fall back to first available
	for _, item := range items {
		if item.Text != "" {
			return &item.Text
		}
	}
	return fallback
}

// pickPreferredURL selects the best poster/backdrop URL from a multi-source array.
func pickPreferredURL(urlsJSON *string, preferredSource string, fallback *string) *string {
	if urlsJSON == nil || *urlsJSON == "" {
		return fallback
	}
	var items []sourceItem
	if err := json.Unmarshal([]byte(*urlsJSON), &items); err != nil {
		return fallback
	}
	for _, item := range items {
		if item.Source == preferredSource && item.URL != "" {
			return &item.URL
		}
	}
	for _, item := range items {
		if item.URL != "" {
			return &item.URL
		}
	}
	return fallback
}

// ParseCacheCredits converts the cache server's simplified cast_crew JSON
// into TMDBCredits format for use with enrichWithCredits.
func ParseCacheCredits(castCrewJSON string) *TMDBCredits {
	type cachePerson struct {
		Name      string `json:"name"`
		Character string `json:"character,omitempty"`
		Job       string `json:"job,omitempty"`
	}
	type cacheCredits struct {
		Cast []cachePerson `json:"cast"`
		Crew []cachePerson `json:"crew"`
	}

	var cc cacheCredits
	if err := json.Unmarshal([]byte(castCrewJSON), &cc); err != nil {
		log.Printf("[cache-client] failed to parse cache cast_crew: %v", err)
		return nil
	}

	credits := &TMDBCredits{}
	for i, c := range cc.Cast {
		credits.Cast = append(credits.Cast, TMDBCastMember{
			Name:      c.Name,
			Character: c.Character,
			Order:     i,
		})
	}
	for _, c := range cc.Crew {
		credits.Crew = append(credits.Crew, TMDBCrewMember{
			Name: c.Name,
			Job:  c.Job,
		})
	}

	return credits
}

// ── Contribute ──

// ContributeExtras holds optional extra data to include in a contribution.
type ContributeExtras struct {
	CastCrewJSON     *string
	Runtime          *int
	IMDBRating       *float64
	RTCriticScore    *int
	RTAudienceScore  *int
	Tagline          *string
	OriginalLanguage *string
	Country          *string
	TrailerURL       *string
	LogoURL          *string
	BannerURL        *string
	CollectionID     *int
	CollectionName   *string
	BackdropURL      *string
	Keywords         *string
	OriginalTitle    *string
	ReleaseDate      *string
	TVDBID           *int
}

// Contribute pushes a locally-fetched metadata result back to the cache server
// so other instances can benefit. Supports TMDB, MusicBrainz, and OpenLibrary sources.
// The optional extras parameter allows sending cast/crew, runtime, and ratings.
func (c *CacheClient) Contribute(match *models.MetadataMatch, extras ...ContributeExtras) {
	if match == nil || match.ExternalID == "" {
		return
	}

	// Only contribute sources the cache server understands
	switch match.Source {
	case "tmdb", "musicbrainz", "openlibrary":
		// OK
	default:
		return
	}

	tmdbID, err := strconv.Atoi(match.ExternalID)
	if err != nil {
		// For non-numeric IDs (MusicBrainz UUIDs, OpenLibrary keys), use 0
		tmdbID = 0
	}

	var genresJSON *string
	if len(match.Genres) > 0 {
		data, _ := json.Marshal(match.Genres)
		s := string(data)
		genresJSON = &s
	}

	// Map source to cache server media type
	mediaType := "movie"
	switch match.Source {
	case "musicbrainz":
		mediaType = "music"
	case "openlibrary":
		mediaType = "audiobook"
	}

	req := cacheContributeRequest{
		TMDBID:           tmdbID,
		MediaType:        mediaType,
		Title:            match.Title,
		Year:             match.Year,
		Description:      match.Description,
		PosterURL:        match.PosterURL,
		BackdropURL:      match.BackdropURL,
		Genres:           genresJSON,
		ContentRating:    match.ContentRating,
		Tagline:          match.Tagline,
		OriginalLanguage: match.OriginalLanguage,
		Country:          match.Country,
		TrailerURL:       match.TrailerURL,
		CollectionID:     match.CollectionID,
		CollectionName:   match.CollectionName,
	}
	if match.IMDBId != "" {
		req.IMDBID = &match.IMDBId
	}

	// Apply extras if provided
	if len(extras) > 0 {
		ex := extras[0]
		req.CastCrew = ex.CastCrewJSON
		req.Runtime = ex.Runtime
		req.IMDBRating = ex.IMDBRating
		req.RTCriticScore = ex.RTCriticScore
		req.RTAudienceScore = ex.RTAudienceScore
		if ex.Tagline != nil {
			req.Tagline = ex.Tagline
		}
		if ex.OriginalLanguage != nil {
			req.OriginalLanguage = ex.OriginalLanguage
		}
		if ex.Country != nil {
			req.Country = ex.Country
		}
		if ex.TrailerURL != nil {
			req.TrailerURL = ex.TrailerURL
		}
		if ex.LogoURL != nil {
			req.LogoURL = ex.LogoURL
		}
		if ex.BannerURL != nil {
			req.BannerURL = ex.BannerURL
		}
		if ex.CollectionID != nil {
			req.CollectionID = ex.CollectionID
		}
		if ex.CollectionName != nil {
			req.CollectionName = ex.CollectionName
		}
		if ex.BackdropURL != nil {
			req.BackdropURL = ex.BackdropURL
		}
		if ex.Keywords != nil {
			req.Keywords = ex.Keywords
		}
		if ex.OriginalTitle != nil {
			req.OriginalTitle = ex.OriginalTitle
		}
		if ex.ReleaseDate != nil {
			req.ReleaseDate = ex.ReleaseDate
		}
		if ex.TVDBID != nil {
			req.TVDBID = ex.TVDBID
		}
	}

	// Contribute keywords from match if extras didn't provide them
	if req.Keywords == nil && len(match.Keywords) > 0 {
		data, _ := json.Marshal(match.Keywords)
		s := string(data)
		req.Keywords = &s
	}

	bodyBytes, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", c.baseURL+"/api/v1/contribute", bytes.NewReader(bodyBytes))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		log.Printf("[cache-client] contribute failed: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		log.Printf("[cache-client] contributed: %s (%s)", match.Title, match.ExternalID)
	}
}

// ── Health Check ──

// IsAvailable checks if the cache server is reachable.
func (c *CacheClient) IsAvailable() bool {
	resp, err := c.client.Get(c.baseURL + "/api/v1/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ── Registration ──

// CacheServerURL is the hardcoded cache server address.
const CacheServerURL = "http://cache.cine-vault.tv:8090"

type cacheRegisterRequest struct {
	Name      string  `json:"name"`
	Version   *string `json:"version,omitempty"`
	Email     *string `json:"email,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	IP        *string `json:"ip,omitempty"`
}

type cacheRegisterResponse struct {
	InstanceID string `json:"instance_id"`
	APIKey     string `json:"api_key"`
}

// readVersion tries to read the application version from version.json.
func readVersion() *string {
	data, err := os.ReadFile("version.json")
	if err != nil {
		return nil
	}
	var v struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &v) == nil && v.Version != "" {
		return &v.Version
	}
	return nil
}

// Register sends a registration request to the cache server and returns the API key.
func Register() (string, error) {
	ver := readVersion()
	firstName := "Justin"
	lastName := "Dube"
	email := "justin@thedubes.net"

	reqBody := cacheRegisterRequest{
		Name:      "CineVault",
		Version:   ver,
		FirstName: &firstName,
		LastName:  &lastName,
		Email:     &email,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal register request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(CacheServerURL+"/api/v1/register", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("cache server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	var regResp cacheRegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return "", fmt.Errorf("decode register response: %w", err)
	}

	if regResp.APIKey == "" {
		return "", fmt.Errorf("server returned empty API key")
	}

	log.Printf("[cache-client] registered with cache server, instance_id=%s", regResp.InstanceID)
	return regResp.APIKey, nil
}

// EnsureRegistered checks if a cache API key already exists in the database.
// If not, it registers with the cache server and stores the key.
// Returns a ready-to-use CacheClient, or nil if registration fails.
func EnsureRegistered(settingsRepo *repository.SettingsRepository) *CacheClient {
	apiKey, _ := settingsRepo.Get("cache_server_api_key")
	if apiKey != "" {
		return NewCacheClient(CacheServerURL, apiKey)
	}

	// No key yet — register
	key, err := Register()
	if err != nil {
		log.Printf("[cache-client] auto-registration failed: %v (falling back to direct TMDB)", err)
		return nil
	}

	// Store the key for future use
	if err := settingsRepo.Set("cache_server_api_key", key); err != nil {
		log.Printf("[cache-client] failed to save API key: %v", err)
		// Still return a working client even if we can't persist
	}

	// Also store the URL for consistency
	_ = settingsRepo.Set("cache_server_url", CacheServerURL)

	return NewCacheClient(CacheServerURL, key)
}
