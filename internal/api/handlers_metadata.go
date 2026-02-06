package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

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

	// Search across all configured scrapers
	var allMatches []*models.MetadataMatch
	for _, scraper := range s.scrapers {
		matches, err := scraper.Search(media.Title, media.MediaType)
		if err != nil {
			continue
		}
		allMatches = append(allMatches, matches...)
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

	// Apply metadata to media item
	query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
		updated_at = CURRENT_TIMESTAMP WHERE id = $5`
	_, err = s.db.DB.Exec(query, req.Title, req.Year, req.Description, req.Rating, mediaID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
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
