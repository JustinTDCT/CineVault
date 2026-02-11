package api

import (
	"encoding/json"
	"net/http"
)

// ──────────────────── Display Preferences ────────────────────

func (s *Server) handleGetDisplayPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	pref, err := s.displayPrefsRepo.GetByUserID(userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Return overlay_settings as parsed JSON (not escaped string)
	var overlay json.RawMessage
	if err := json.Unmarshal([]byte(pref.OverlaySettings), &overlay); err != nil {
		s.respondError(w, http.StatusInternalServerError, "invalid overlay settings")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"id":               pref.ID,
			"user_id":          pref.UserID,
			"overlay_settings": overlay,
			"created_at":       pref.CreatedAt,
			"updated_at":       pref.UpdatedAt,
		},
	})
}

func (s *Server) handleUpdateDisplayPrefs(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	var req struct {
		OverlaySettings json.RawMessage `json:"overlay_settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.OverlaySettings) == 0 {
		s.respondError(w, http.StatusBadRequest, "overlay_settings is required")
		return
	}
	// Validate it's valid JSON
	var check map[string]interface{}
	if err := json.Unmarshal(req.OverlaySettings, &check); err != nil {
		s.respondError(w, http.StatusBadRequest, "overlay_settings must be valid JSON object")
		return
	}
	if err := s.displayPrefsRepo.Upsert(userID, string(req.OverlaySettings)); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}
