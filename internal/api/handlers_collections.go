package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)

	// Optional library_id filter
	var collections []*models.Collection
	var err error
	if libIDStr := r.URL.Query().Get("library_id"); libIDStr != "" {
		libID, parseErr := uuid.Parse(libIDStr)
		if parseErr != nil {
			s.respondError(w, http.StatusBadRequest, "invalid library_id")
			return
		}
		collections, err = s.collectionRepo.ListByUserAndLibrary(userID, libID)
	} else {
		collections, err = s.collectionRepo.ListByUser(userID)
	}
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

	// Validate smart collection rules
	if coll.CollectionType == "smart" {
		if coll.Rules == nil || *coll.Rules == "" {
			s.respondError(w, http.StatusBadRequest, "smart collections require rules")
			return
		}
		var rules models.SmartCollectionRules
		if err := json.Unmarshal([]byte(*coll.Rules), &rules); err != nil {
			s.respondError(w, http.StatusBadRequest, "invalid rules format")
			return
		}
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

// handleEvaluateSmartCollection evaluates a smart collection's rules and returns matching items.
func (s *Server) handleEvaluateSmartCollection(w http.ResponseWriter, r *http.Request) {
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

	if coll.CollectionType != "smart" || coll.Rules == nil {
		s.respondError(w, http.StatusBadRequest, "collection is not a smart collection")
		return
	}

	items, err := s.collectionRepo.EvaluateSmartCollection(*coll.Rules, coll.LibraryID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to evaluate smart collection: "+err.Error())
		return
	}

	if items == nil {
		items = []*models.MediaItem{}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

// handleUpdateCollection updates a collection's metadata and/or rules.
func (s *Server) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	existing, err := s.collectionRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	// Verify ownership
	userID := s.getUserID(r)
	if existing.UserID != userID {
		s.respondError(w, http.StatusForbidden, "not your collection")
		return
	}

	var update models.Collection
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Apply updates
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Description != nil {
		existing.Description = update.Description
	}
	if update.PosterPath != nil {
		existing.PosterPath = update.PosterPath
	}
	if update.Visibility != "" {
		existing.Visibility = update.Visibility
	}
	if update.ItemSortMode != "" {
		existing.ItemSortMode = update.ItemSortMode
	}
	if update.Rules != nil {
		existing.Rules = update.Rules
	}

	existing.ID = id
	if err := s.collectionRepo.Update(existing); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update collection")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: existing})
}
