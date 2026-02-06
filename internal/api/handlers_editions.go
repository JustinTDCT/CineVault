package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListEditions(w http.ResponseWriter, r *http.Request) {
	groups, err := s.editionRepo.ListGroups(100, 0)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list edition groups")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: groups})
}

func (s *Server) handleCreateEdition(w http.ResponseWriter, r *http.Request) {
	var group models.EditionGroup
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group.ID = uuid.New()
	if err := s.editionRepo.CreateGroup(&group); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create edition group")
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: group})
}

func (s *Server) handleGetEdition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid edition group ID")
		return
	}

	group, err := s.editionRepo.GetGroupByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "edition group not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: group})
}

func (s *Server) handleUpdateEdition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid edition group ID")
		return
	}

	var group models.EditionGroup
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group.ID = id
	if err := s.editionRepo.UpdateGroup(&group); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update edition group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: group})
}

func (s *Server) handleDeleteEdition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid edition group ID")
		return
	}

	if err := s.editionRepo.DeleteGroup(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete edition group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "edition group deleted"}})
}

func (s *Server) handleAddEditionItem(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid edition group ID")
		return
	}

	var item models.EditionItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	item.ID = uuid.New()
	item.EditionGroupID = groupID
	userID := s.getUserID(r)
	item.AddedBy = &userID

	if err := s.editionRepo.AddItem(&item); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to add edition item")
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: item})
}

func (s *Server) handleRemoveEditionItem(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid edition group ID")
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	if err := s.editionRepo.RemoveItem(groupID, itemID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to remove edition item")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "item removed"}})
}
