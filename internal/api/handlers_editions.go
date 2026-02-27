package api

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// handleGetMediaEditions returns all editions for the edition group containing the given media item.
// Also includes cache server edition metadata (AI-discovered) when available.
func (s *Server) handleGetMediaEditions(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	// Find the edition group for this media item
	groupID, err := s.editionRepo.GetGroupByMediaID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch cache server edition metadata if we have a TMDB ID
	var cacheEditions interface{}
	if tmdbID := s.extractTMDBIDForMedia(mediaID); tmdbID > 0 {
		if cacheClient := s.getCacheClient(); cacheClient != nil {
			editions, err := cacheClient.ListEditions(tmdbID)
			if err == nil && len(editions) > 0 {
				cacheEditions = editions
			}
		}
	}

	if groupID == nil {
		data := map[string]interface{}{
			"has_editions": false,
			"editions":     []interface{}{},
		}
		if cacheEditions != nil {
			data["cache_editions"] = cacheEditions
			data["cache_editions_source"] = "ai"
		}
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: data})
		return
	}

	// Get all editions with media details
	editions, err := s.editionRepo.ListItemsWithMedia(*groupID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	data := map[string]interface{}{
		"has_editions":     true,
		"edition_group_id": groupID,
		"editions":         editions,
	}
	if cacheEditions != nil {
		data["cache_editions"] = cacheEditions
		data["cache_editions_source"] = "ai"
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: data})
}

// extractTMDBIDForMedia reads a media item's external_ids JSON and extracts the TMDB ID.
func (s *Server) extractTMDBIDForMedia(mediaID uuid.UUID) int {
	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil || media == nil || media.ExternalIDs == nil {
		return 0
	}
	var externalIDs map[string]interface{}
	if err := json.Unmarshal([]byte(*media.ExternalIDs), &externalIDs); err != nil {
		return 0
	}
	if tmdbVal, ok := externalIDs["tmdb_id"]; ok {
		switch v := tmdbVal.(type) {
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				return id
			}
		case float64:
			return int(v)
		}
	}
	return 0
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
