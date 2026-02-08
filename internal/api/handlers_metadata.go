package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
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

	scrapers := metadata.ScrapersForMediaType(s.scrapers, media.MediaType)
	if len(scrapers) == 0 {
		scrapers = s.scrapers // fallback to all if no type-specific match
	}

	var allMatches []*models.MetadataMatch
	for _, scraper := range scrapers {
		matches, err := scraper.Search(query, media.MediaType)
		if err != nil {
			continue
		}
		allMatches = append(allMatches, matches...)
	}

	// Extract year from the filename (not the DB) to avoid using a previously bad match
	fileYear := yearFromFilename(media.FileName)
	if fileYear == nil {
		// Try full file path if filename didn't have it
		fileYear = yearFromFilename(media.FilePath)
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

	var req struct {
		Source     string  `json:"source"`
		ExternalID string  `json:"external_id"`
		Title      string  `json:"title"`
		Year       *int    `json:"year"`
		Description *string `json:"description"`
		PosterURL  *string `json:"poster_url"`
		Rating     *float64 `json:"rating"`
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
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
			log.Printf("Apply metadata: poster saved to %s", saved)
		}
	}

	// Apply metadata to media item and lock to prevent auto-overwrite
	if posterPath != nil {
		query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
			poster_path = $5, metadata_locked = true, updated_at = CURRENT_TIMESTAMP WHERE id = $6`
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

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleAutoMatch(w http.ResponseWriter, r *http.Request) {
	libID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library id")
		return
	}

	// Enqueue as async job
	if s.jobQueue != nil {
		jobID, err := s.jobQueue.Enqueue("metadata:scrape", map[string]string{
			"library_id": libID.String(),
		})
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
