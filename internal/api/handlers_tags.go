package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Tag Handlers ────────────────────

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	tree := r.URL.Query().Get("tree") == "true"

	if tree {
		tags, err := s.tagRepo.BuildTree(category)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: tags})
		return
	}

	tags, err := s.tagRepo.List(category)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: tags})
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var t models.Tag
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t.ID = uuid.New()
	if err := s.tagRepo.Create(&t); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: t})
}

func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid tag id")
		return
	}
	var t models.Tag
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t.ID = id
	if err := s.tagRepo.Update(&t); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: t})
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid tag id")
		return
	}
	if err := s.tagRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleGetMediaTags(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	tags, err := s.tagRepo.GetMediaTags(mediaID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: tags})
}

func (s *Server) handleAssignTags(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	var req struct {
		TagIDs []string `json:"tag_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	for _, idStr := range req.TagIDs {
		tagID, _ := uuid.Parse(idStr)
		s.tagRepo.AssignToMedia(mediaID, tagID)
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleRemoveTag(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	tagID, err := uuid.Parse(r.PathValue("tagId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid tag id")
		return
	}
	if err := s.tagRepo.RemoveFromMedia(mediaID, tagID); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true})
}
