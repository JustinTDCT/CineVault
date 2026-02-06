package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListSisters(w http.ResponseWriter, r *http.Request) {
	groups, err := s.sisterRepo.List(100, 0)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list sister groups")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: groups})
}

func (s *Server) handleCreateSister(w http.ResponseWriter, r *http.Request) {
	var group models.SisterGroup
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group.ID = uuid.New()
	userID := s.getUserID(r)
	group.CreatedBy = &userID

	if err := s.sisterRepo.Create(&group); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create sister group")
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: group})
}

func (s *Server) handleGetSister(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}

	group, err := s.sisterRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "sister group not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: group})
}

func (s *Server) handleDeleteSister(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}

	if err := s.sisterRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete sister group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "sister group deleted"}})
}

type AddSisterItemRequest struct {
	MediaItemID string `json:"media_item_id"`
}

func (s *Server) handleAddSisterItem(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}

	var req AddSisterItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mediaItemID, err := uuid.Parse(req.MediaItemID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media item ID")
		return
	}

	if err := s.sisterRepo.AddMember(groupID, mediaItemID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to add item to sister group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "item added"}})
}

func (s *Server) handleRemoveSisterItem(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	if err := s.sisterRepo.RemoveMember(groupID, itemID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to remove item from sister group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "item removed"}})
}
