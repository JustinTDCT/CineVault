package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/JustinTDCT/CineVault/internal/jobs"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// ──────────────────── Media Segments ────────────────────

// handleGetSegments returns all skip segments for a media item.
func (s *Server) handleGetSegments(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	segments, err := s.segmentRepo.GetByMediaID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get segments")
		return
	}
	if segments == nil {
		segments = []*models.MediaSegment{}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: segments})
}

// handleUpsertSegment creates or updates a skip segment (manual entry by admin).
func (s *Server) handleUpsertSegment(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	var req struct {
		SegmentType  string  `json:"segment_type"`
		StartSeconds float64 `json:"start_seconds"`
		EndSeconds   float64 `json:"end_seconds"`
		Verified     bool    `json:"verified"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate segment type
	switch models.SegmentType(req.SegmentType) {
	case models.SegmentIntro, models.SegmentCredits, models.SegmentRecap, models.SegmentPreview:
		// valid
	default:
		s.respondError(w, http.StatusBadRequest, "invalid segment type (must be intro, credits, recap, or preview)")
		return
	}

	if req.StartSeconds < 0 || req.EndSeconds <= req.StartSeconds {
		s.respondError(w, http.StatusBadRequest, "invalid time range")
		return
	}

	seg := &models.MediaSegment{
		ID:           uuid.New(),
		MediaItemID:  mediaID,
		SegmentType:  models.SegmentType(req.SegmentType),
		StartSeconds: req.StartSeconds,
		EndSeconds:   req.EndSeconds,
		Confidence:   1.0, // Manual entry = full confidence
		Source:       models.SegmentSourceManual,
		Verified:     req.Verified,
	}

	if err := s.segmentRepo.Upsert(seg); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to save segment")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: seg})
}

// handleDeleteSegment removes a segment by media ID and type.
func (s *Server) handleDeleteSegment(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}
	segType := r.PathValue("type")
	if segType == "" {
		s.respondError(w, http.StatusBadRequest, "segment type required")
		return
	}

	if err := s.segmentRepo.Delete(mediaID, segType); err != nil {
		s.respondError(w, http.StatusNotFound, "segment not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

// handleDetectSegments triggers background segment detection for a library.
func (s *Server) handleDetectSegments(w http.ResponseWriter, r *http.Request) {
	libID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	payload := jobs.DetectSegmentsPayload{LibraryID: libID.String()}
	uniqueID := "detect:" + libID.String()
	_, err = s.jobQueue.EnqueueUnique(jobs.TaskDetectSegments, payload, uniqueID,
		asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to enqueue detection job")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{
		"status": "queued",
	}})
}

// ──────────────────── User Skip Preferences ────────────────────

// handleGetSkipPrefs returns the current user's skip preferences.
func (s *Server) handleGetSkipPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	prefs, err := s.segmentRepo.GetSkipPrefs(userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get skip preferences")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: prefs})
}

// handleUpdateSkipPrefs updates the current user's skip preferences.
func (s *Server) handleUpdateSkipPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var pref models.UserSkipPreference
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pref.UserID = userID
	if err := s.segmentRepo.UpsertSkipPrefs(&pref); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update skip preferences")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}
