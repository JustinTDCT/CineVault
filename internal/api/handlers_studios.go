package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Studio Handlers ────────────────────

func (s *Server) handleListStudios(w http.ResponseWriter, r *http.Request) {
	studioType := r.URL.Query().Get("type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	studios, err := s.studioRepo.List(studioType, limit, offset)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: studios})
}

func (s *Server) handleCreateStudio(w http.ResponseWriter, r *http.Request) {
	var st models.Studio
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	st.ID = uuid.New()
	if err := s.studioRepo.Create(&st); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: st})
}

func (s *Server) handleGetStudio(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid studio id")
		return
	}
	studio, err := s.studioRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: studio})
}

func (s *Server) handleUpdateStudio(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid studio id")
		return
	}
	var st models.Studio
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	st.ID = id
	if err := s.studioRepo.Update(&st); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: st})
}

func (s *Server) handleDeleteStudio(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid studio id")
		return
	}
	if err := s.studioRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleLinkStudio(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	var req struct {
		StudioID string `json:"studio_id"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	studioID, _ := uuid.Parse(req.StudioID)
	if err := s.studioRepo.LinkMedia(mediaID, studioID, req.Role); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleUnlinkStudio(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	studioID, err := uuid.Parse(r.PathValue("studioId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid studio id")
		return
	}
	if err := s.studioRepo.UnlinkMedia(mediaID, studioID); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}
