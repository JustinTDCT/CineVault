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
	Year            *int     `json:"year,omitempty"`
	ReleaseDate     *string  `json:"release_date,omitempty"`
	Description     *string  `json:"description,omitempty"`
	PosterURL       *string  `json:"poster_url,omitempty"`
	Genres          *string  `json:"genres,omitempty"`
	CastCrew        *string  `json:"cast_crew,omitempty"`
	ContentRating   *string  `json:"content_rating,omitempty"`
	Runtime         *int     `json:"runtime,omitempty"`
	IMDBRating      *float64 `json:"imdb_rating,omitempty"`
	RTCriticScore   *int     `json:"rt_critic_score,omitempty"`
	RTAudienceScore *int     `json:"rt_audience_score,omitempty"`
	OMDbEnriched    bool     `json:"omdb_enriched"`
}

type cacheLookupResponse struct {
	Hit        bool        `json:"hit"`
	Confidence float64     `json:"confidence"`
	Entry      *cacheEntry `json:"entry,omitempty"`
	Source     string      `json:"source"`
}

type cacheContributeRequest struct {
	TMDBID        int     `json:"tmdb_id"`
	IMDBID        *string `json:"imdb_id,omitempty"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	Year          *int    `json:"year,omitempty"`
	Description   *string `json:"description,omitempty"`
	PosterURL     *string `json:"poster_url,omitempty"`
	Genres        *string `json:"genres,omitempty"`
	ContentRating *string `json:"content_rating,omitempty"`
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
	if entry.Description != nil {
		match.Description = entry.Description
	}
	if entry.PosterURL != nil {
		match.PosterURL = entry.PosterURL
	}
	if entry.IMDBID != nil {
		match.IMDBId = *entry.IMDBID
	}
	if entry.ContentRating != nil {
		match.ContentRating = entry.ContentRating
	}

	// Parse genres from JSON string
	if entry.Genres != nil && *entry.Genres != "" {
		var genres []string
		if err := json.Unmarshal([]byte(*entry.Genres), &genres); err == nil {
			match.Genres = genres
			result.Genres = genres
		}
	}

	result.Match = match

	// Extract ratings if OMDb enriched
	if entry.OMDbEnriched {
		result.Ratings = &OMDbRatings{
			IMDBRating:    entry.IMDBRating,
			RTScore:       entry.RTCriticScore,
			AudienceScore: entry.RTAudienceScore,
		}
	}

	return result
}

// ── Contribute ──

// Contribute pushes a locally-fetched metadata result back to the cache server
// so other instances can benefit. Supports TMDB, MusicBrainz, and OpenLibrary sources.
func (c *CacheClient) Contribute(match *models.MetadataMatch) {
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
		TMDBID:        tmdbID,
		MediaType:     mediaType,
		Title:         match.Title,
		Year:          match.Year,
		Description:   match.Description,
		PosterURL:     match.PosterURL,
		Genres:        genresJSON,
		ContentRating: match.ContentRating,
	}
	if match.IMDBId != "" {
		req.IMDBID = &match.IMDBId
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
