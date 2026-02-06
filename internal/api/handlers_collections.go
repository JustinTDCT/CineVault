package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	collections, err := s.collectionRepo.ListByUser(userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list collections")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: collections})
}

func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var coll models.Collection
	if err := json.NewDecoder(r.Body).Decode(&coll); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	coll.ID = uuid.New()
	coll.UserID = s.getUserID(r)
	if coll.CollectionType == "" {
		coll.CollectionType = "manual"
	}
	if coll.Visibility == "" {
		coll.Visibility = "private"
	}
	if coll.ItemSortMode == "" {
		coll.ItemSortMode = "custom"
	}

	if err := s.collectionRepo.Create(&coll); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create collection")
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: coll})
}

func (s *Server) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	coll, err := s.collectionRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: coll})
}

func (s *Server) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	if err := s.collectionRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete collection")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "collection deleted"}})
}

func (s *Server) handleAddCollectionItem(w http.ResponseWriter, r *http.Request) {
	collectionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	var item models.CollectionItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	item.ID = uuid.New()
	item.CollectionID = collectionID
	userID := s.getUserID(r)
	item.AddedBy = &userID

	if err := s.collectionRepo.AddItem(&item); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to add collection item")
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: item})
}

func (s *Server) handleRemoveCollectionItem(w http.ResponseWriter, r *http.Request) {
	collectionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	if err := s.collectionRepo.RemoveItem(collectionID, itemID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to remove collection item")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "item removed"}})
}
