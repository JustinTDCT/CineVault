package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type UpdateProgressRequest struct {
	ProgressSeconds int  `json:"progress_seconds"`
	DurationSeconds *int `json:"duration_seconds,omitempty"`
	Completed       bool `json:"completed"`
}

func (s *Server) handleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	var req UpdateProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := s.getUserID(r)
	wh := &models.WatchHistory{
		ID:              uuid.New(),
		UserID:          userID,
		MediaItemID:     mediaID,
		ProgressSeconds: req.ProgressSeconds,
		DurationSeconds: req.DurationSeconds,
		Completed:       req.Completed,
	}

	if err := s.watchRepo.Upsert(wh); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update watch progress")
		return
	}

	// Increment play count for music tracks when playback starts
	if req.ProgressSeconds == 0 {
		item, _ := s.mediaRepo.GetByID(mediaID)
		if item != nil && (item.MediaType == models.MediaTypeMusic || item.MediaType == models.MediaTypeMusicVideos) {
			_ = s.mediaRepo.IncrementPlayCount(mediaID)
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: wh})
}

func (s *Server) handleContinueWatching(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	items, err := s.watchRepo.ContinueWatching(userID, 20)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get continue watching")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}
