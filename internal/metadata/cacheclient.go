package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
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

// ── Envelope Helper ──

// apiEnvelope represents the standard { "status": "...", "data": {...} } response wrapper.
type apiEnvelope struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

// unwrapEnvelope decodes the standard API envelope and returns the raw data payload.
func unwrapEnvelope(resp *http.Response) (json.RawMessage, error) {
	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	if env.Status != "ok" {
		return nil, fmt.Errorf("api returned status %q", env.Status)
	}
	return env.Data, nil
}

// ── Cache Server Response Types ──

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
	Genres          json.RawMessage `json:"genres,omitempty"`
	CastCrew        json.RawMessage `json:"cast_crew,omitempty"`
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
	PosterURLs   json.RawMessage `json:"poster_urls,omitempty"`
	BackdropURLs json.RawMessage `json:"backdrop_urls,omitempty"`
	Descriptions json.RawMessage `json:"descriptions,omitempty"`
	LogoURLs     json.RawMessage `json:"logo_urls,omitempty"`
	// Ratings
	IMDBRating      *float64 `json:"imdb_rating,omitempty"`
	RTCriticScore   *int     `json:"rt_critic_score,omitempty"`
	RTAudienceScore *int     `json:"rt_audience_score,omitempty"`
	MetacriticScore *int     `json:"metacritic_score,omitempty"`
	OMDbEnriched    bool     `json:"omdb_enriched"`
	FanartEnriched  bool     `json:"fanart_enriched"`
	// Keywords from TMDB
	Keywords json.RawMessage `json:"keywords,omitempty"`
	// Extended ratings and certifications
	ContentRatings json.RawMessage `json:"content_ratings,omitempty"`
	// Multi-source field arrays (JSON)
	Taglines          json.RawMessage `json:"taglines,omitempty"`
	ContentRatingsAll json.RawMessage `json:"content_ratings_multi,omitempty"`
	Trailers          json.RawMessage `json:"trailers,omitempty"`
	Runtimes          json.RawMessage `json:"runtimes,omitempty"`
	FieldSources      json.RawMessage `json:"field_sources,omitempty"`
	// Cache server local image paths
	PosterPath   *string `json:"poster_path,omitempty"`
	BackdropPath *string `json:"backdrop_path,omitempty"`
	// Enrichment flags
	EditionsDiscovered  bool `json:"editions_discovered"`
	AllSourcesScraped   bool `json:"all_sources_scraped"`
	PostersDownloaded   bool `json:"posters_downloaded"`
	BackdropsDownloaded bool `json:"backdrops_downloaded"`
	// AI-discovered editions (from cache server JSONB metadata)
	AvailableEditions []cacheAvailableEdition `json:"available_editions,omitempty"`
	// Edition matched by cache server when edition param was sent
	MatchedEdition *MatchedEditionInfo `json:"matched_edition,omitempty"`
}

type cacheAvailableEdition struct {
	EditionType    string   `json:"edition_type"`
	Description    string   `json:"description,omitempty"`
	EditionTitle   string   `json:"edition_title,omitempty"`
	ContentSummary string   `json:"new_content_summary,omitempty"`
	ContentRating  string   `json:"content_rating,omitempty"`
	KnownRes       []string `json:"known_resolutions,omitempty"`
	RuntimeMin     *int     `json:"runtime_minutes,omitempty"`
	AdditionalMin  *int     `json:"additional_runtime_minutes,omitempty"`
	EditionYear    *int     `json:"edition_release_year,omitempty"`
	Verified       bool     `json:"verified"`
	Source         string   `json:"source"`
}

type cacheEditionSummary struct {
	EditionType        string  `json:"edition_type"`
	EditionTitle       *string `json:"edition_title,omitempty"`
	Source             string  `json:"source"`
	KnownResolutions   *string `json:"known_resolutions,omitempty"`   // JSON: ["4K","1080p"]
	ContentRating      *string `json:"content_rating,omitempty"`
	Verified           bool    `json:"verified"`
	VerificationSource *string `json:"verification_source,omitempty"`
}

type cacheLookupResponse struct {
	Hit               bool                  `json:"hit"`
	Confidence        float64               `json:"confidence"`
	Entry             *cacheEntry           `json:"entry,omitempty"`
	Source            string                `json:"source"`
	AvailableEditions []cacheEditionSummary `json:"available_editions,omitempty"`
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

	// AvailableEditions lists known alternate editions (AI-discovered)
	AvailableEditions []EditionSummary

	// MatchedEdition is the edition matched by the cache server when an
	// edition param was sent with the lookup request.
	MatchedEdition *MatchedEditionInfo

	// ── New unified metadata fields ──

	// Metacritic score from OMDb
	MetacriticScore *int

	// Multi-country content ratings (raw JSON for storage)
	ContentRatingsJSON *string // {"US":"PG-13","GB":"12A"}
	ContentRatingsAll  *string // [{"source":"tmdb","country":"US","rating":"PG-13"}]

	// Multi-source field arrays (raw JSON for storage)
	TaglinesJSON     *string // [{"source":"tmdb","text":"..."}]
	TrailersJSON     *string // [{"source":"tmdb","url":"...","name":"..."}]
	DescriptionsJSON *string // [{"source":"tmdb","text":"..."}]

	// All artwork URLs from all sources
	AllPosterURLs   []string
	AllBackdropURLs []string
	AllLogoURLs     []string

	// Cache server local image paths (for proxied serving)
	CacheServerPosterPath   *string
	CacheServerBackdropPath *string

	// Enrichment status flags
	EditionsDiscovered bool
	AllSourcesScraped  bool
}

// EditionSummary is a lightweight reference to a known edition from the cache server.
type EditionSummary struct {
	EditionType        string   `json:"edition_type"`
	EditionTitle       *string  `json:"edition_title,omitempty"`
	Source             string   `json:"source"` // "openai"
	KnownResolutions   []string `json:"known_resolutions,omitempty"`
	ContentRating      *string  `json:"content_rating,omitempty"`
	Verified           bool     `json:"verified"`
	VerificationSource *string  `json:"verification_source,omitempty"`
}

// MatchedEditionInfo holds edition data returned by the cache server when
// the requested edition matched one of the record's available editions.
type MatchedEditionInfo struct {
	EditionType       string   `json:"edition_type"`
	Description       string   `json:"description,omitempty"`
	EditionTitle      string   `json:"edition_title,omitempty"`
	RuntimeMinutes    *int     `json:"runtime_minutes,omitempty"`
	AdditionalRuntime *int     `json:"additional_runtime_minutes,omitempty"`
	EditionYear       *int     `json:"edition_release_year,omitempty"`
	ContentSummary    string   `json:"new_content_summary,omitempty"`
	KnownResolutions  []string `json:"known_resolutions,omitempty"`
	ContentRating     string   `json:"content_rating,omitempty"`
	Verified          bool     `json:"verified"`
	Source            string   `json:"source"`
}

// rawToString converts json.RawMessage to *string for pass-through storage.
func rawToString(raw json.RawMessage) *string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	s := string(raw)
	return &s
}

// ── Lookup ──

// mediaTypeToURLType maps CineVault media types to cache server URL path types.
func mediaTypeToURLType(mediaType models.MediaType) string {
	switch mediaType {
	case models.MediaTypeTVShows:
		return "tv_show"
	case models.MediaTypeMusic:
		return "music_track"
	case models.MediaTypeMusicVideos:
		return "music_video"
	case models.MediaTypeAudiobooks:
		return "audiobook"
	case models.MediaTypeAdultMovies:
		return "adult_movie"
	default:
		return "movie"
	}
}

// Lookup queries the cache server for metadata. Returns nil if the cache
// server is unreachable or returns a miss. An optional edition string can be
// provided to request edition enrichment from the cache server.
func (c *CacheClient) Lookup(title string, year *int, mediaType models.MediaType, edition ...string) *CacheLookupResult {
	urlType := mediaTypeToURLType(mediaType)

	reqURL := fmt.Sprintf("%s/api/v1/lookup/%s?title=%s",
		c.baseURL, urlType, url.QueryEscape(title))
	if year != nil && *year > 0 {
		reqURL += fmt.Sprintf("&year=%d", *year)
	}
	if len(edition) > 0 && edition[0] != "" {
		reqURL += "&edition=" + url.QueryEscape(edition[0])
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		log.Printf("[cache-client] request error: %v", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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

	// Read cache status and confidence from response headers
	cacheStatus := resp.Header.Get("X-Cache-Status")
	confidence := 0.0
	if confStr := resp.Header.Get("X-Match-Confidence"); confStr != "" {
		if parsed, err := strconv.ParseFloat(confStr, 64); err == nil {
			confidence = parsed
		}
	}

	// Unwrap the { "status": "ok", "data": {...} } envelope
	data, err := unwrapEnvelope(resp)
	if err != nil {
		log.Printf("[cache-client] envelope error: %v", err)
		return nil
	}

	// Data contains the record fields directly (no nested "entry")
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		log.Printf("[cache-client] decode error: %v", err)
		return nil
	}

	if entry.Title == "" && entry.TMDBID == 0 {
		return nil
	}

	// Derive source from entry or cache header
	source := entry.Source
	if source == "" {
		if cacheStatus == "hit" {
			source = "hit"
		} else {
			source = "tmdb"
		}
	}

	// Construct a cacheLookupResponse so convertCacheResponse can be reused
	lookupResp := &cacheLookupResponse{
		Hit:        true,
		Confidence: confidence,
		Entry:      &entry,
		Source:     source,
	}

	return c.convertCacheResponse(lookupResp)
}

// LookupByTMDB queries the cache server by TMDB ID. Used for deferred
// edition re-queries where the TMDB ID is already known.
func (c *CacheClient) LookupByTMDB(tmdbID int, mediaType models.MediaType) *CacheLookupResult {
	urlType := mediaTypeToURLType(mediaType)
	reqURL := fmt.Sprintf("%s/api/v1/lookup/%s?tmdb_id=%d", c.baseURL, urlType, tmdbID)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		log.Printf("[cache-client] tmdb lookup request error: %v", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("[cache-client] tmdb lookup unreachable: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[cache-client] tmdb lookup returned %d", resp.StatusCode)
		return nil
	}

	confidence := 0.0
	if confStr := resp.Header.Get("X-Match-Confidence"); confStr != "" {
		if parsed, err := strconv.ParseFloat(confStr, 64); err == nil {
			confidence = parsed
		}
	}

	data, err := unwrapEnvelope(resp)
	if err != nil {
		log.Printf("[cache-client] tmdb lookup envelope error: %v", err)
		return nil
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		log.Printf("[cache-client] tmdb lookup decode error: %v", err)
		return nil
	}

	if entry.Title == "" && entry.TMDBID == 0 {
		return nil
	}

	lookupResp := &cacheLookupResponse{
		Hit:        true,
		Confidence: confidence,
		Entry:      &entry,
		Source:     "hit",
	}

	return c.convertCacheResponse(lookupResp)
}

// BatchLookupItem holds lookup parameters for a single item in a batch.
type BatchLookupItem struct {
	Title     string
	Year      *int
	MediaType models.MediaType
}

// BatchLookup queries the cache server for multiple items.
// The batch endpoint is not available in the new cache-server API;
// items are looked up individually.
func (c *CacheClient) BatchLookup(items []BatchLookupItem) []*CacheLookupResult {
	if len(items) == 0 {
		return nil
	}

	log.Printf("[cache-client] DEPRECATED: BatchLookup — using sequential individual lookups (%d items)", len(items))

	results := make([]*CacheLookupResult, len(items))
	for i, it := range items {
		results[i] = c.Lookup(it.Title, it.Year, it.MediaType)
	}
	return results
}

// convertCacheResponse builds a CacheLookupResult from a raw cache response.
func (c *CacheClient) convertCacheResponse(lookupResp *cacheLookupResponse) *CacheLookupResult {
	if !lookupResp.Hit || lookupResp.Entry == nil {
		return nil
	}

	entry := lookupResp.Entry
	result := &CacheLookupResult{
		Hit:        true,
		Confidence: lookupResp.Confidence,
		Source:     lookupResp.Source,
	}

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
	match.Description = pickPreferred(rawToString(entry.Descriptions), metadataSource, entry.Description)

	// Use preferred poster from multi-source array if available
	match.PosterURL = pickPreferredURL(rawToString(entry.PosterURLs), metadataSource, entry.PosterURL)
	if match.PosterURL == nil && entry.PosterPath != nil && *entry.PosterPath != "" {
		url := CacheImageURL(*entry.PosterPath)
		match.PosterURL = &url
	}

	// Use preferred backdrop
	match.BackdropURL = pickPreferredURL(rawToString(entry.BackdropURLs), metadataSource, entry.BackdropURL)
	if match.BackdropURL == nil && entry.BackdropPath != nil && *entry.BackdropPath != "" {
		url := CacheImageURL(*entry.BackdropPath)
		match.BackdropURL = &url
	}

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

	// Parse genres from JSON
	if len(entry.Genres) > 0 && string(entry.Genres) != "null" {
		var genres []string
		if err := json.Unmarshal(entry.Genres, &genres); err != nil {
			log.Printf("[cache-client] genres JSON parse error for %q: %v", entry.Title, err)
		} else {
			match.Genres = genres
			result.Genres = genres
		}
	}

	// Parse keywords from JSON
	if len(entry.Keywords) > 0 && string(entry.Keywords) != "null" {
		var keywords []string
		if err := json.Unmarshal(entry.Keywords, &keywords); err != nil {
			log.Printf("[cache-client] keywords JSON parse error for %q: %v", entry.Title, err)
		} else {
			match.Keywords = keywords
			result.Keywords = keywords
		}
	}

	result.Match = match

	// Pass through cast/crew JSON so scanner can use it directly
	result.CastCrewJSON = rawToString(entry.CastCrew)

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

	// Metacritic score
	result.MetacriticScore = entry.MetacriticScore

	// Build external IDs JSON for storage
	result.ExternalIDsJSON = buildExternalIDsJSON(entry, true)

	// ── New unified metadata fields ──

	// Multi-country content ratings
	result.ContentRatingsJSON = rawToString(entry.ContentRatings)
	result.ContentRatingsAll = rawToString(entry.ContentRatingsAll)

	// Multi-source arrays (raw JSON pass-through for storage)
	result.TaglinesJSON = rawToString(entry.Taglines)
	result.TrailersJSON = rawToString(entry.Trailers)
	result.DescriptionsJSON = rawToString(entry.Descriptions)

	// Enrichment status
	result.EditionsDiscovered = entry.EditionsDiscovered
	result.AllSourcesScraped = entry.AllSourcesScraped

	// Parse artwork URL arrays from JSON — cache server sends objects with source/url/path
	result.AllPosterURLs = parseArtworkURLArray(rawToString(entry.PosterURLs))
	result.AllBackdropURLs = parseArtworkURLArray(rawToString(entry.BackdropURLs))
	result.AllLogoURLs = parseArtworkURLArray(rawToString(entry.LogoURLs))

	// Cache server local image paths
	result.CacheServerPosterPath = entry.PosterPath
	result.CacheServerBackdropPath = entry.BackdropPath

	// Extract artist/album from MusicBrainz cache entries for music hierarchy
	if metadataSource == "musicbrainz" {
		ccStr := rawToString(entry.CastCrew)
		match.ArtistName = extractArtistFromCastCrew(ccStr)
		match.ArtistMBID = extractArtistMBIDFromCastCrew(ccStr)
		if entry.OriginalTitle != nil && *entry.OriginalTitle != "" {
			match.AlbumTitle = *entry.OriginalTitle
		}
	}

	// Build available editions from the flattened cache entry data
	if len(entry.AvailableEditions) > 0 {
		editions := make([]EditionSummary, 0, len(entry.AvailableEditions))
		for _, ae := range entry.AvailableEditions {
			title := ae.EditionTitle
			var titlePtr *string
			if title != "" {
				titlePtr = &title
			}
			cr := ae.ContentRating
			var crPtr *string
			if cr != "" {
				crPtr = &cr
			}
			es := EditionSummary{
				EditionType:      ae.EditionType,
				EditionTitle:     titlePtr,
				Source:           ae.Source,
				KnownResolutions: ae.KnownRes,
				ContentRating:    crPtr,
				Verified:         ae.Verified,
			}
			editions = append(editions, es)
		}
		result.AvailableEditions = editions
	}

	if entry.MatchedEdition != nil {
		result.MatchedEdition = entry.MatchedEdition
	}

	return result
}

// ── Search (multi-result) ──

// cacheSearchResult mirrors the cache server's SearchResult model.
type cacheSearchResult struct {
	TMDBID      int     `json:"tmdb_id"`
	Title       string  `json:"title"`
	Year        *int    `json:"year,omitempty"`
	Overview    string  `json:"overview,omitempty"`
	PosterPath  string  `json:"poster_path,omitempty"`
	VoteAverage float64 `json:"vote_average,omitempty"`
	Confidence  float64 `json:"confidence"`
}

type cacheSearchResponse struct {
	Results []cacheSearchResult `json:"results"`
}

// Search queries the cache server's multi-result search endpoint and returns
// all results above minConfidence, up to maxResults. This mirrors the
// Jellyfin/Plex approach of returning multiple candidates for user selection.
func (c *CacheClient) Search(query string, mediaType models.MediaType, year *int, minConfidence float64, maxResults int) []*models.MetadataMatch {
	urlType := mediaTypeToURLType(mediaType)

	reqURL := fmt.Sprintf("%s/api/v1/search/%s?title=%s&max=%d",
		c.baseURL, urlType, url.QueryEscape(query), maxResults)
	if year != nil && *year > 0 {
		reqURL += fmt.Sprintf("&year=%d", *year)
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		log.Printf("[cache-client] search request error: %v", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("[cache-client] search unreachable: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[cache-client] search returned %d for %q", resp.StatusCode, query)
		return nil
	}

	data, err := unwrapEnvelope(resp)
	if err != nil {
		log.Printf("[cache-client] search envelope error: %v", err)
		return nil
	}

	var searchResp cacheSearchResponse
	if err := json.Unmarshal(data, &searchResp); err != nil {
		log.Printf("[cache-client] search decode error: %v", err)
		return nil
	}

	var matches []*models.MetadataMatch
	for _, sr := range searchResp.Results {
		if sr.Confidence < minConfidence {
			continue
		}

		m := &models.MetadataMatch{
			Source:     "tmdb",
			ExternalID: fmt.Sprintf("%d", sr.TMDBID),
			Title:      sr.Title,
			Year:       sr.Year,
			Confidence: sr.Confidence,
		}
		if sr.Overview != "" {
			m.Description = &sr.Overview
		}
		if sr.PosterPath != "" {
			posterURL := "https://image.tmdb.org/t/p/w500" + sr.PosterPath
			m.PosterURL = &posterURL
		}
		if sr.VoteAverage > 0 {
			m.Rating = &sr.VoteAverage
		}
		matches = append(matches, m)
	}

	log.Printf("[cache-client] search for %q returned %d result(s) (of %d from server)",
		query, len(matches), len(searchResp.Results))
	return matches
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
		Name        string `json:"name"`
		Character   string `json:"character,omitempty"`
		Job         string `json:"job,omitempty"`
		ID          int    `json:"id,omitempty"`
		ProfilePath string `json:"profile_path,omitempty"`
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
			ID:          c.ID,
			Name:        c.Name,
			Character:   c.Character,
			ProfilePath: c.ProfilePath,
			Order:       i,
		})
	}
	for _, c := range cc.Crew {
		credits.Crew = append(credits.Crew, TMDBCrewMember{
			ID:          c.ID,
			Name:        c.Name,
			Job:         c.Job,
			ProfilePath: c.ProfilePath,
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
	// MediaType overrides the default media type derived from match.Source.
	// Use this for TV shows (models.MediaTypeTVShows) since TMDB source defaults to "movie".
	MediaType models.MediaType
}

// Contribute is a no-op — the new cache-server auto-fetches metadata.
func (c *CacheClient) Contribute(match *models.MetadataMatch, extras ...ContributeExtras) {
	log.Printf("[cache-client] contribute not needed with new cache-server")
}

// ── Phase 6: Cache Integration Endpoints ──

// CacheTVSeason holds season data from the cache server.
type CacheTVSeason struct {
	ID           string           `json:"id"`
	SeasonNumber int              `json:"season_number"`
	Title        *string          `json:"title,omitempty"`
	Description  *string          `json:"description,omitempty"`
	PosterURL    *string          `json:"poster_url,omitempty"`
	AirDate      *string          `json:"air_date,omitempty"`
	EpisodeCount int              `json:"episode_count"`
	Episodes     []CacheTVEpisode `json:"episodes,omitempty"`
}

// CacheTVEpisode holds episode data from the cache server.
type CacheTVEpisode struct {
	ID             string   `json:"id"`
	EpisodeNumber  int      `json:"episode_number"`
	AbsoluteNumber *int     `json:"absolute_number,omitempty"`
	Title          *string  `json:"title,omitempty"`
	Description    *string  `json:"description,omitempty"`
	AirDate        *string  `json:"air_date,omitempty"`
	Runtime        *int     `json:"runtime,omitempty"`
	StillURL       *string  `json:"still_url,omitempty"`
	Rating         *float64 `json:"rating,omitempty"`
}

// CachePerformer holds performer data from the cache server.
type CachePerformer struct {
	ID                 string  `json:"id"`
	TMDBID             *int    `json:"tmdb_id,omitempty"`
	Name               string  `json:"name"`
	PhotoURL           *string `json:"photo_url,omitempty"`
	PhotoPath          *string `json:"photo_path,omitempty"`
	Bio                *string `json:"bio,omitempty"`
	BirthDate          *string `json:"birth_date,omitempty"`
	DeathDate          *string `json:"death_date,omitempty"`
	BirthPlace         *string `json:"birth_place,omitempty"`
	Gender             *int    `json:"gender,omitempty"`
	KnownForDepartment *string `json:"known_for_department,omitempty"`
	Source             string  `json:"source"`
}

// CacheCollection holds collection data from the cache server.
type CacheCollection struct {
	ID           string  `json:"id"`
	TMDBID       *int    `json:"tmdb_id,omitempty"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	PosterURL    *string `json:"poster_url,omitempty"`
	BackdropURL  *string `json:"backdrop_url,omitempty"`
	MemberIDs    *string `json:"member_ids,omitempty"`
}

// GetTVSeason — not yet available in the new cache-server API.
func (c *CacheClient) GetTVSeason(tmdbID, seasonNumber int) (*CacheTVSeason, error) {
	log.Printf("[cache-client] GetTVSeason not available in new cache-server API")
	return nil, nil
}

// GetTVSeasons — not yet available in the new cache-server API.
func (c *CacheClient) GetTVSeasons(tmdbID int, includeEpisodes bool) ([]CacheTVSeason, error) {
	log.Printf("[cache-client] GetTVSeasons not available in new cache-server API")
	return nil, nil
}

// GetPerformer — not yet available in the new cache-server API.
func (c *CacheClient) GetPerformer(tmdbID int) (*CachePerformer, error) {
	log.Printf("[cache-client] GetPerformer not available in new cache-server API")
	return nil, nil
}

// GetCollection — not yet available in the new cache-server API.
func (c *CacheClient) GetCollection(tmdbID int) (*CacheCollection, error) {
	log.Printf("[cache-client] GetCollection not available in new cache-server API")
	return nil, nil
}

// CollectionArtwork holds poster and backdrop artwork items for a collection.
type CollectionArtwork struct {
	Posters   []CollectionArtworkItem `json:"posters"`
	Backdrops []CollectionArtworkItem `json:"backdrops"`
}

// CollectionArtworkItem holds a single artwork entry with source and URL/path.
type CollectionArtworkItem struct {
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
	Path   string `json:"path,omitempty"`
}

// GetCollectionArtwork — not yet available in the new cache-server API.
func (c *CacheClient) GetCollectionArtwork(tmdbID int) (*CollectionArtwork, error) {
	log.Printf("[cache-client] GetCollectionArtwork not available in new cache-server API")
	return nil, nil
}

// ── Edition Metadata ──

// CacheEdition holds edition-specific metadata from the cache server.
type CacheEdition struct {
	ID                 string  `json:"id"`
	TMDBID             int     `json:"tmdb_id"`
	EditionType        string  `json:"edition_type"`
	EditionTitle       *string `json:"edition_title,omitempty"`
	Overview           *string `json:"overview,omitempty"`
	Tagline            *string `json:"tagline,omitempty"`
	Runtime            *int    `json:"runtime,omitempty"`
	TheatricalRuntime  *int    `json:"theatrical_runtime,omitempty"`
	AdditionalRuntime  *int    `json:"additional_runtime,omitempty"`
	EditionReleaseYear *int    `json:"edition_release_year,omitempty"`
	NewContentSummary  *string `json:"new_content_summary,omitempty"`
	AddedScenes        *string `json:"added_scenes,omitempty"`
	Differences        *string `json:"differences,omitempty"`
	CanonStatus        *string `json:"canon_status,omitempty"`
	PopularityNotes    *string  `json:"popularity_notes,omitempty"`
	Keywords           *string  `json:"keywords,omitempty"`
	Source             string   `json:"source"`
	KnownResolutions   *string  `json:"known_resolutions,omitempty"` // JSON: ["4K","1080p"]
	ContentRating      *string  `json:"content_rating,omitempty"`
	Verified           bool     `json:"verified"`
	VerificationSource *string  `json:"verification_source,omitempty"`
}

// FetchEditionMetadata — not yet available in the new cache-server API.
func (c *CacheClient) FetchEditionMetadata(tmdbID int, title string, year *int, editionType string) (*CacheEdition, error) {
	log.Printf("[cache-client] FetchEditionMetadata not available in new cache-server API")
	return nil, nil
}

// GetEdition — not yet available in the new cache-server API.
func (c *CacheClient) GetEdition(tmdbID int, editionType string) (*CacheEdition, error) {
	log.Printf("[cache-client] GetEdition not available in new cache-server API")
	return nil, nil
}

// ListEditions — not yet available in the new cache-server API.
func (c *CacheClient) ListEditions(tmdbID int) ([]CacheEdition, error) {
	log.Printf("[cache-client] ListEditions not available in new cache-server API")
	return nil, nil
}

// BatchContributeItem matches the cache server's batch contribute item format.
type BatchContributeItem struct {
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
	FileHash         *string  `json:"file_hash,omitempty"`
}

// ContributeBatch is a no-op — the new cache-server auto-fetches metadata.
func (c *CacheClient) ContributeBatch(items []BatchContributeItem) (succeeded, failed int) {
	log.Printf("[cache-client] contribute not needed with new cache-server")
	return 0, 0
}

// addInstanceVersion adds the X-Instance-Version header from the app version.
func addInstanceVersion(req *http.Request) {
	ver := readVersion()
	if ver != nil {
		req.Header.Set("X-Instance-Version", *ver)
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

// CacheServerURL is the default cache server address (used for registration and fallback).
const CacheServerURL = "http://cache.cine-vault.tv:8090"

// resolvedCacheURL holds the active cache server URL (set by EnsureRegistered from DB).
var resolvedCacheURL = CacheServerURL

type cacheRegisterRequest struct {
	AppName    string  `json:"app_name"`
	AppVersion *string `json:"app_version,omitempty"`
	OwnerName  *string `json:"owner_name,omitempty"`
	OwnerEmail *string `json:"owner_email,omitempty"`
	WanIP      *string `json:"wan_ip,omitempty"`
}

type cacheRegisterResponse struct {
	ClientID string `json:"client_id"`
	APIKey   string `json:"api_key"`
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
	ownerName := "Justin Dube"
	email := "justin@thedubes.net"

	reqBody := cacheRegisterRequest{
		AppName:    "CineVault",
		AppVersion: ver,
		OwnerName:  &ownerName,
		OwnerEmail: &email,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal register request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(resolvedCacheURL+"/api/v1/register", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("cache server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return "", fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	// Unwrap the { "status": "ok", "data": {...} } envelope
	data, err := unwrapEnvelope(resp)
	if err != nil {
		return "", fmt.Errorf("registration envelope error: %w", err)
	}

	var regResp cacheRegisterResponse
	if err := json.Unmarshal(data, &regResp); err != nil {
		return "", fmt.Errorf("decode register response: %w", err)
	}

	if regResp.APIKey == "" {
		return "", fmt.Errorf("server returned empty API key")
	}

	log.Printf("[cache-client] registered with cache server, client_id=%s", regResp.ClientID)
	return regResp.APIKey, nil
}

// EnsureRegistered checks if a cache API key already exists in the database.
// If not, it registers with the cache server and stores the key.
// Returns a ready-to-use CacheClient, or nil if registration fails.
func EnsureRegistered(settingsRepo *repository.SettingsRepository) *CacheClient {
	// Use DB-stored URL if available, otherwise fall back to hardcoded constant
	serverURL := CacheServerURL
	if dbURL, _ := settingsRepo.Get("cache_server_url"); dbURL != "" {
		serverURL = dbURL
	}
	resolvedCacheURL = serverURL
	log.Printf("[cache-client] using cache server URL: %s", serverURL)

	apiKey, _ := settingsRepo.Get("cache_server_api_key")
	if apiKey != "" {
		return NewCacheClient(serverURL, apiKey)
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
	_ = settingsRepo.Set("cache_server_url", serverURL)

	return NewCacheClient(serverURL, key)
}

// ── Content Rating Resolution ──

// countryRatingEntry represents a single content rating from a specific source/country.
type countryRatingEntry struct {
	Source  string `json:"source"`
	Country string `json:"country"`
	Rating  string `json:"rating"`
}

// ResolveContentRating parses the multi-country ratings JSON and returns
// the rating for the preferred country. Falls back to US, then first available.
func ResolveContentRating(ratingsJSON string, preferredCountry string) string {
	var ratings []countryRatingEntry
	if err := json.Unmarshal([]byte(ratingsJSON), &ratings); err != nil || len(ratings) == 0 {
		return ""
	}

	// Look for preferred country first
	for _, r := range ratings {
		if r.Country == preferredCountry && r.Rating != "" {
			return r.Rating
		}
	}
	// Fall back to US
	if preferredCountry != "US" {
		for _, r := range ratings {
			if r.Country == "US" && r.Rating != "" {
				return r.Rating
			}
		}
	}
	// Fall back to first available
	for _, r := range ratings {
		if r.Rating != "" {
			return r.Rating
		}
	}
	return ""
}

// CacheImageURL builds a full URL for a cache server local image path.
func CacheImageURL(basePath string) string {
	if basePath == "" {
		return ""
	}
	return resolvedCacheURL + "/images/" + basePath
}

// artworkEntry represents one item in the cache server's poster_urls/backdrop_urls arrays.
type artworkEntry struct {
	Source string `json:"source"`
	URL    string `json:"url"`
	Path   string `json:"path"`
}

// parseArtworkURLArray parses the JSON artwork array from the cache server.
// It prefers the local cached path (via /images/) when available, otherwise falls back to the original URL.
// extractArtistFromCastCrew parses the first artist name from the MusicBrainz-style
// cast/crew JSON: {"cast":[{"name":"ArtistName","character":"Artist","mbid":"..."}],...}
func extractArtistFromCastCrew(castCrew *string) string {
	if castCrew == nil || *castCrew == "" {
		return ""
	}
	var cc struct {
		Cast []struct {
			Name string `json:"name"`
		} `json:"cast"`
	}
	if err := json.Unmarshal([]byte(*castCrew), &cc); err != nil {
		return ""
	}
	if len(cc.Cast) > 0 {
		return cc.Cast[0].Name
	}
	return ""
}

func extractArtistMBIDFromCastCrew(castCrew *string) string {
	if castCrew == nil || *castCrew == "" {
		return ""
	}
	var cc struct {
		Cast []struct {
			MBID string `json:"mbid"`
		} `json:"cast"`
	}
	if err := json.Unmarshal([]byte(*castCrew), &cc); err != nil {
		return ""
	}
	if len(cc.Cast) > 0 {
		return cc.Cast[0].MBID
	}
	return ""
}

func parseArtworkURLArray(raw *string) []string {
	if raw == nil || *raw == "" || *raw == "null" {
		return nil
	}

	// Try parsing as array of objects (current cache server format)
	var entries []artworkEntry
	if err := json.Unmarshal([]byte(*raw), &entries); err == nil && len(entries) > 0 {
		urls := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.Path != "" {
				urls = append(urls, CacheImageURL(e.Path))
			} else if e.URL != "" {
				urls = append(urls, e.URL)
			}
		}
		return urls
	}

	// Fall back to plain string array (legacy format)
	var plain []string
	if err := json.Unmarshal([]byte(*raw), &plain); err == nil {
		return plain
	}

	return nil
}

// CacheMusicArtist represents a music artist from the cache server.
type CacheMusicArtist struct {
	ID        string  `json:"id"`
	MBID      string  `json:"mbid"`
	Name      string  `json:"name"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	PhotoPath *string `json:"photo_path,omitempty"`
}

// LookupMusicArtist — not yet available in the new cache-server API.
func (c *CacheClient) LookupMusicArtist(mbid, name string) *CacheMusicArtist {
	log.Printf("[cache-client] LookupMusicArtist not available in new cache-server API")
	return nil
}
