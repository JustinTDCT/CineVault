package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Performer Handlers ────────────────────

func (s *Server) handleListPerformers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	performers, err := s.performerRepo.List(search, limit, offset)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: performers})
}

func (s *Server) handleCreatePerformer(w http.ResponseWriter, r *http.Request) {
	var p models.Performer
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = uuid.New()
	if err := s.performerRepo.Create(&p); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: p})
}

func (s *Server) handleGetPerformer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}

	performer, err := s.performerRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, err.Error())
		return
	}

	// Get linked media
	media, _ := s.performerRepo.GetPerformerMedia(id)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"performer": performer,
		"media":     media,
	}})
}

func (s *Server) handleUpdatePerformer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}

	var p models.Performer
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id

	if err := s.performerRepo.Update(&p); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: p})
}

func (s *Server) handleDeletePerformer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}
	if err := s.performerRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleLinkPerformer(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	var req struct {
		PerformerID   string `json:"performer_id"`
		Role          string `json:"role"`
		CharacterName string `json:"character_name"`
		SortOrder     int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	performerID, _ := uuid.Parse(req.PerformerID)
	if err := s.performerRepo.LinkMedia(mediaID, performerID, req.Role, req.CharacterName, req.SortOrder); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleUnlinkPerformer(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	performerID, err := uuid.Parse(r.PathValue("performerId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid performer id")
		return
	}
	if err := s.performerRepo.UnlinkMedia(mediaID, performerID); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}
