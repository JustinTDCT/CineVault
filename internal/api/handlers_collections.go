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
	if update.ParentCollectionID != nil {
		existing.ParentCollectionID = update.ParentCollectionID
	}

	existing.ID = id
	if err := s.collectionRepo.Update(existing); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update collection")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: existing})
}

// handleListCollectionChildren returns child collections of a given parent.
func (s *Server) handleListCollectionChildren(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	children, err := s.collectionRepo.ListChildren(id)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list children")
		return
	}
	if children == nil {
		children = []*models.Collection{}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: children})
}

// handleGetCollectionStats returns aggregate statistics for a collection.
func (s *Server) handleGetCollectionStats(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	stats, err := s.collectionRepo.GetStats(id)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get collection stats")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: stats})
}

// handleBulkAddCollectionItems adds multiple items to a collection at once.
func (s *Server) handleBulkAddCollectionItems(w http.ResponseWriter, r *http.Request) {
	collectionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	var items []models.CollectionItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body: expected array of items")
		return
	}

	if len(items) == 0 {
		s.respondError(w, http.StatusBadRequest, "no items provided")
		return
	}

	userID := s.getUserID(r)
	if err := s.collectionRepo.BulkAddItems(collectionID, items, &userID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to bulk add items: "+err.Error())
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{
		"message": "items added",
		"count":   len(items),
	}})
}

// handleCreateCollectionTemplates creates default smart collection presets for the user.
func (s *Server) handleCreateCollectionTemplates(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)

	templates := []struct {
		Name  string
		Rules string
	}{
		{
			Name:  "Recently Added",
			Rules: `{"added_within":30,"sort_by":"added","sort_order":"desc","max_results":50}`,
		},
		{
			Name:  "Top Rated",
			Rules: `{"min_rating":8.0,"sort_by":"rating","sort_order":"desc","max_results":50}`,
		},
		{
			Name:  "Short Films",
			Rules: `{"max_duration":60,"sort_by":"duration","sort_order":"asc","max_results":50}`,
		},
		{
			Name:  "Classic Cinema",
			Rules: `{"year_from":1920,"year_to":1979,"min_rating":7.0,"sort_by":"year","sort_order":"asc","max_results":100}`,
		},
	}

	var created []*models.Collection
	for _, t := range templates {
		rules := t.Rules
		coll := &models.Collection{
			ID:             uuid.New(),
			UserID:         userID,
			Name:           t.Name,
			CollectionType: "smart",
			Visibility:     "private",
			ItemSortMode:   "custom",
			Rules:          &rules,
		}
		if err := s.collectionRepo.Create(coll); err != nil {
			// Skip duplicates / errors silently
			continue
		}
		created = append(created, coll)
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: map[string]interface{}{
		"message": "templates created",
		"count":   len(created),
		"collections": created,
	}})
}
