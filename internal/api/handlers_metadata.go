package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// yearFromFilename extracts a 4-digit year from a filename, preferring the
// year closest to the filename's path (which is most likely the content year).
var identifyYearPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\((\d{4})\)`),            // (2021)
	regexp.MustCompile(`\[(\d{4})\]`),            // [2021]
	regexp.MustCompile(`[.\s_-](\d{4})[.\s_-]`),  // .2021. or -2021-
}

func yearFromFilename(filename string) *int {
	for _, p := range identifyYearPatterns {
		matches := p.FindStringSubmatch(filename)
		if len(matches) >= 2 {
			var y int
			fmt.Sscanf(matches[1], "%d", &y)
			if y >= 1900 && y <= 2100 {
				return &y
			}
		}
	}
	return nil
}

// getCacheClient returns a CacheClient if the cache server is enabled.
// Returns nil if disabled or registration fails.
func (s *Server) getCacheClient() *metadata.CacheClient {
	if s.settingsRepo == nil {
		return nil
	}
	enabled, _ := s.settingsRepo.Get("cache_server_enabled")
	if enabled == "false" {
		return nil
	}
	return metadata.EnsureRegistered(s.settingsRepo)
}

// ──────────────────── Metadata Handlers ────────────────────

func (s *Server) handleIdentifyMedia(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	// Clean the title and search applicable scrapers
	query := metadata.CleanTitleForSearch(media.Title)
	if query == "" {
		query = media.Title
	}

	// Extract year from the filename (not the DB) to avoid using a previously bad match
	fileYear := yearFromFilename(media.FileName)
	if fileYear == nil {
		fileYear = yearFromFilename(media.FilePath)
	}

	var allMatches []*models.MetadataMatch

	// ── Try cache server first if enabled ──
	usedCache := false
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		result := cacheClient.Lookup(query, fileYear, media.MediaType)
		if result != nil && result.Match != nil {
			allMatches = append(allMatches, result.Match)
			usedCache = true
			log.Printf("Identify: cache hit for %q → %q (confidence=%.2f)", query, result.Match.Title, result.Confidence)
		}
	}

	// ── Fall back to direct scrapers if cache is not enabled or returned no results ──
	if !usedCache {
		scrapers := metadata.ScrapersForMediaType(s.scrapers, media.MediaType)
		if len(scrapers) == 0 {
			scrapers = s.scrapers
		}

		for _, scraper := range scrapers {
			matches, err := scraper.Search(query, media.MediaType, fileYear)
			if err != nil {
				continue
			}
			allMatches = append(allMatches, matches...)
		}
	}

	// Apply year-aware scoring: boost matches with matching year, penalize mismatches
	if fileYear != nil && *fileYear > 0 {
		for _, m := range allMatches {
			if m.Year != nil {
				diff := *fileYear - *m.Year
				if diff < 0 {
					diff = -diff
				}
				if diff == 0 {
					m.Confidence = min(m.Confidence+0.15, 1.0)
				} else if diff <= 1 {
					m.Confidence = min(m.Confidence+0.05, 1.0)
				} else {
					m.Confidence = max(m.Confidence-0.3, 0.0)
				}
			}
		}
	}

	// Sort by confidence descending
	for i := 0; i < len(allMatches); i++ {
		for j := i + 1; j < len(allMatches); j++ {
			if allMatches[j].Confidence > allMatches[i].Confidence {
				allMatches[i], allMatches[j] = allMatches[j], allMatches[i]
			}
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: allMatches})
}

func (s *Server) handleApplyMetadata(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	var req struct {
		Source      string   `json:"source"`
		ExternalID  string   `json:"external_id"`
		Title       string   `json:"title"`
		Year        *int     `json:"year"`
		Description *string  `json:"description"`
		PosterURL   *string  `json:"poster_url"`
		Rating      *float64 `json:"rating"`
		Genres      []string `json:"genres"`
		IMDBId      string   `json:"imdb_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Download poster if URL provided
	var posterPath *string
	if req.PosterURL != nil && *req.PosterURL != "" && s.config.Paths.Preview != "" {
		filename := mediaID.String() + ".jpg"
		saved, dlErr := metadata.DownloadPoster(*req.PosterURL, filepath.Join(s.config.Paths.Preview, "posters"), filename)
		if dlErr != nil {
			log.Printf("Apply metadata: poster download failed for %s: %v", mediaID, dlErr)
		} else {
			// Use actual saved filename (may include _alt suffix from dedup)
			webPath := "/previews/posters/" + filepath.Base(saved)
			posterPath = &webPath
			log.Printf("Apply metadata: poster saved to %s", saved)
		}
	}

	// Apply metadata to media item and lock to prevent auto-overwrite
	if posterPath != nil {
		query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
			poster_path = $5, generated_poster = false, metadata_locked = true, updated_at = CURRENT_TIMESTAMP WHERE id = $6`
		_, err = s.db.DB.Exec(query, req.Title, req.Year, req.Description, req.Rating, *posterPath, mediaID)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
			metadata_locked = true, updated_at = CURRENT_TIMESTAMP WHERE id = $5`
		_, err = s.db.DB.Exec(query, req.Title, req.Year, req.Description, req.Rating, mediaID)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Link genre tags
	if len(req.Genres) > 0 {
		s.linkGenreTags(mediaID, req.Genres)
	}

	// ── Supplementary data (IMDB ID, ratings, credits) via cache or direct APIs ──
	imdbID := req.IMDBId
	var cacheResult *metadata.CacheLookupResult

	// Try cache server first for supplementary data
	cacheClient := s.getCacheClient()
	if cacheClient != nil && req.ExternalID != "" {
		cacheResult = cacheClient.Lookup(req.Title, req.Year, media.MediaType)
		if cacheResult != nil && cacheResult.Match != nil {
			// Use IMDB ID from cache if not provided
			if imdbID == "" && cacheResult.Match.IMDBId != "" {
				imdbID = cacheResult.Match.IMDBId
			}
			// Apply ratings from cache
			if cacheResult.Ratings != nil {
				if dbErr := s.mediaRepo.UpdateRatings(mediaID, cacheResult.Ratings.IMDBRating, cacheResult.Ratings.RTScore, cacheResult.Ratings.AudienceScore); dbErr != nil {
					log.Printf("Apply metadata: cache ratings update failed for %s: %v", mediaID, dbErr)
				}
			}
			// Apply cast/crew from cache
			if cacheResult.CastCrewJSON != nil && *cacheResult.CastCrewJSON != "" {
				credits := metadata.ParseCacheCredits(*cacheResult.CastCrewJSON)
				if credits != nil {
					s.enrichWithCredits(mediaID, credits)
				}
			}
			// Apply genres from cache if not already provided
			if len(req.Genres) == 0 && len(cacheResult.Genres) > 0 {
				s.linkGenreTags(mediaID, cacheResult.Genres)
			}
			log.Printf("Apply metadata: used cache for supplementary data on %q", req.Title)
		}
	}

	// Fall back to direct APIs if cache didn't provide what we need
	if cacheResult == nil || cacheResult.Match == nil {
		// Fetch IMDB ID from TMDB details if not available
		if imdbID == "" && req.Source == "tmdb" && req.ExternalID != "" {
			for _, sc := range s.scrapers {
				if t, ok := sc.(*metadata.TMDBScraper); ok {
					details, detailErr := t.GetDetails(req.ExternalID)
					if detailErr == nil && details.IMDBId != "" {
						imdbID = details.IMDBId
						if len(req.Genres) == 0 && len(details.Genres) > 0 {
							s.linkGenreTags(mediaID, details.Genres)
						}
					}
					break
				}
			}
		}

		// Fetch OMDb ratings
		if imdbID != "" {
			omdbKey, keyErr := s.settingsRepo.Get("omdb_api_key")
			if keyErr == nil && omdbKey != "" {
				ratings, omdbErr := metadata.FetchOMDbRatings(imdbID, omdbKey)
				if omdbErr != nil {
					log.Printf("Apply metadata: OMDb fetch failed for %s: %v", imdbID, omdbErr)
				} else {
					if dbErr := s.mediaRepo.UpdateRatings(mediaID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore); dbErr != nil {
						log.Printf("Apply metadata: ratings update failed for %s: %v", mediaID, dbErr)
					}
				}
			}
		}

		// Fetch cast/crew from TMDB credits
		if req.Source == "tmdb" && req.ExternalID != "" {
			for _, sc := range s.scrapers {
				if t, ok := sc.(*metadata.TMDBScraper); ok {
					var credits *metadata.TMDBCredits
					var credErr error
					if media.MediaType == models.MediaTypeTVShows {
						credits, credErr = t.GetTVCredits(req.ExternalID)
					} else {
						credits, credErr = t.GetMovieCredits(req.ExternalID)
					}
					if credErr != nil {
						log.Printf("Apply metadata: TMDB credits failed for %s: %v", req.ExternalID, credErr)
					} else if credits != nil {
						s.enrichWithCredits(mediaID, credits)
					}
					break
				}
			}
		}
	}

	// Store external IDs
	usedCache := cacheResult != nil && cacheResult.Match != nil
	if usedCache && cacheResult.ExternalIDsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(mediaID, *cacheResult.ExternalIDsJSON)
	} else {
		idsJSON := metadata.BuildExternalIDsFromMatch(req.Source, req.ExternalID, imdbID, false)
		if idsJSON != nil {
			_ = s.mediaRepo.UpdateExternalIDs(mediaID, *idsJSON)
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// linkGenreTags creates genre tags (if they don't exist) and links them to a media item.
func (s *Server) linkGenreTags(mediaItemID uuid.UUID, genres []string) {
	for _, genre := range genres {
		existing, _ := s.tagRepo.List("genre")
		var tagID uuid.UUID
		found := false
		for _, t := range existing {
			if strings.EqualFold(t.Name, genre) {
				tagID = t.ID
				found = true
				break
			}
		}
		if !found {
			tagID = uuid.New()
			tag := &models.Tag{
				ID:       tagID,
				Name:     genre,
				Category: models.TagCategoryGenre,
			}
			if err := s.tagRepo.Create(tag); err != nil {
				log.Printf("Apply metadata: create genre tag %q failed: %v", genre, err)
				continue
			}
		}
		if err := s.tagRepo.AssignToMedia(mediaItemID, tagID); err != nil {
			log.Printf("Apply metadata: assign genre tag %q to %s failed: %v", genre, mediaItemID, err)
		}
	}
}

// enrichWithCredits creates or finds performers from TMDB credits and links them to a media item.
func (s *Server) enrichWithCredits(mediaItemID uuid.UUID, credits *metadata.TMDBCredits) {
	if credits == nil {
		return
	}

	// Import cast (top 20)
	maxCast := 20
	if len(credits.Cast) < maxCast {
		maxCast = len(credits.Cast)
	}
	for i := 0; i < maxCast; i++ {
		member := credits.Cast[i]
		if member.Name == "" {
			continue
		}
		performer, err := s.findOrCreatePerformer(member.Name, models.PerformerActor, member.ProfilePath)
		if err != nil {
			log.Printf("Apply metadata: create performer %q failed: %v", member.Name, err)
			continue
		}
		charName := member.Character
		if err := s.performerRepo.LinkMedia(mediaItemID, performer.ID, "actor", charName, member.Order); err != nil {
			log.Printf("Apply metadata: link performer %q failed: %v", member.Name, err)
		}
	}

	// Import key crew
	importedCrew := 0
	for _, member := range credits.Crew {
		if member.Name == "" {
			continue
		}
		var perfType models.PerformerType
		switch member.Job {
		case "Director":
			perfType = models.PerformerDirector
		case "Producer", "Executive Producer":
			perfType = models.PerformerProducer
		case "Screenplay", "Writer", "Story":
			perfType = models.PerformerOther
		default:
			continue
		}
		performer, err := s.findOrCreatePerformer(member.Name, perfType, member.ProfilePath)
		if err != nil {
			log.Printf("Apply metadata: create crew %q failed: %v", member.Name, err)
			continue
		}
		role := strings.ToLower(member.Job)
		if err := s.performerRepo.LinkMedia(mediaItemID, performer.ID, role, "", 100+importedCrew); err != nil {
			log.Printf("Apply metadata: link crew %q failed: %v", member.Name, err)
		}
		importedCrew++
	}
}

// findOrCreatePerformer finds an existing performer by name or creates a new one with optional photo.
func (s *Server) findOrCreatePerformer(name string, perfType models.PerformerType, profilePath string) (*models.Performer, error) {
	existing, err := s.performerRepo.FindByName(name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Download photo if missing
		if existing.PhotoPath == nil && profilePath != "" && s.config.Paths.Preview != "" {
			photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
			filename := "performer_" + existing.ID.String() + ".jpg"
			if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.config.Paths.Preview, "posters"), filename); dlErr == nil {
				webPath := "/previews/posters/" + filename
				existing.PhotoPath = &webPath
				_ = s.performerRepo.Update(existing)
			}
		}
		return existing, nil
	}

	p := &models.Performer{
		ID:            uuid.New(),
		Name:          name,
		PerformerType: perfType,
	}

	if profilePath != "" && s.config.Paths.Preview != "" {
		photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
		filename := "performer_" + p.ID.String() + ".jpg"
		if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.config.Paths.Preview, "posters"), filename); dlErr == nil {
			webPath := "/previews/posters/" + filename
			p.PhotoPath = &webPath
		}
	}

	if err := s.performerRepo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Server) handleRefreshMetadata(w http.ResponseWriter, r *http.Request) {
	libID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library id")
		return
	}

	// Enqueue as async job (deduplicated by library ID)
	if s.jobQueue != nil {
		uniqueID := "metadata-refresh:" + libID.String()
		jobID, err := s.jobQueue.EnqueueUnique("metadata:refresh", map[string]string{
			"library_id": libID.String(),
		}, uniqueID, asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]string{"job_id": jobID}})
		return
	}

	s.respondError(w, http.StatusServiceUnavailable, "job queue not available")
}

func (s *Server) handleAutoMatch(w http.ResponseWriter, r *http.Request) {
	libID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library id")
		return
	}

	// Enqueue as async job (deduplicated by library ID)
	if s.jobQueue != nil {
		uniqueID := "metadata:" + libID.String()
		jobID, err := s.jobQueue.EnqueueUnique("metadata:scrape", map[string]string{
			"library_id": libID.String(),
		}, uniqueID, asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]string{"job_id": jobID}})
		return
	}

	s.respondError(w, http.StatusServiceUnavailable, "job queue not available")
}

// ──────────────────── Sort Order Handler ────────────────────

func (s *Server) handleUpdateSortOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntityType string   `json:"entity_type"`
		IDs        []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tableName := ""
	switch req.EntityType {
	case "media_items":
		tableName = "media_items"
	case "performers":
		tableName = "performers"
	case "tags":
		tableName = "tags"
	case "studios":
		tableName = "studios"
	case "edition_items":
		tableName = "edition_items"
	case "collection_items":
		tableName = "collection_items"
	case "tv_shows":
		tableName = "tv_shows"
	case "tv_seasons":
		tableName = "tv_seasons"
	default:
		s.respondError(w, http.StatusBadRequest, "unsupported entity type")
		return
	}

	for i, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		colName := "sort_position"
		if req.EntityType == "edition_items" || req.EntityType == "collection_items" {
			colName = "sort_position"
		}
		query := `UPDATE ` + tableName + ` SET ` + colName + ` = $1 WHERE id = $2`
		s.db.DB.Exec(query, i, id)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── Job Status Handler ────────────────────

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.jobRepo.ListRecent(50)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: jobs})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	job, err := s.jobRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: job})
}

// ──────────────────── Playback Preferences ────────────────────

func (s *Server) handleGetPlaybackPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var pref models.UserPlaybackPreference
	query := `SELECT id, user_id, playback_mode, preferred_quality, auto_play_next,
		subtitle_language, audio_language, created_at, updated_at
		FROM user_playback_preferences WHERE user_id = $1`
	err := s.db.DB.QueryRow(query, userID).Scan(&pref.ID, &pref.UserID, &pref.PlaybackMode,
		&pref.PreferredQuality, &pref.AutoPlayNext, &pref.SubtitleLanguage, &pref.AudioLanguage,
		&pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		// Return defaults
		pref = models.UserPlaybackPreference{
			UserID:           userID,
			PlaybackMode:     models.PlaybackAlwaysAsk,
			PreferredQuality: "1080p",
			AutoPlayNext:     true,
		}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: pref})
}

func (s *Server) handleUpdatePlaybackPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var pref models.UserPlaybackPreference
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	query := `INSERT INTO user_playback_preferences (id, user_id, playback_mode, preferred_quality, auto_play_next,
		subtitle_language, audio_language) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET playback_mode=$3, preferred_quality=$4, auto_play_next=$5,
		subtitle_language=$6, audio_language=$7, updated_at=CURRENT_TIMESTAMP`
	_, err := s.db.DB.Exec(query, uuid.New(), userID, pref.PlaybackMode, pref.PreferredQuality,
		pref.AutoPlayNext, pref.SubtitleLanguage, pref.AudioLanguage)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// ──────────────────── System Settings ────────────────────

func (s *Server) handleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.settingsRepo.GetAll()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Mask sensitive keys for display
	for _, sensitiveKey := range []string{"omdb_api_key"} {
		if val, ok := settings[sensitiveKey]; ok && len(val) > 4 {
			settings[sensitiveKey] = val[:4] + strings.Repeat("*", len(val)-4)
		}
	}
	// For cache_server_api_key, expose only whether it exists (for the "Registered" indicator)
	// but never return the actual value to the frontend
	if val, ok := settings["cache_server_api_key"]; ok && val != "" {
		settings["cache_server_api_key"] = "registered"
	}
	// Never expose the internal cache URL to the frontend
	delete(settings, "cache_server_url")
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: settings})
}

func (s *Server) handleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Sensitive keys that get masked – skip update if the value is still masked
	sensitiveKeys := map[string]bool{"omdb_api_key": true}
	// Internal keys that the frontend should never overwrite
	internalKeys := map[string]bool{"cache_server_api_key": true, "cache_server_url": true}

	for key, value := range req {
		// Never allow the frontend to overwrite internal keys
		if internalKeys[key] {
			continue
		}
		// Don't overwrite real values with masked placeholders
		if sensitiveKeys[key] && strings.Contains(value, "****") {
			continue
		}
		if err := s.settingsRepo.Set(key, value); err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}
