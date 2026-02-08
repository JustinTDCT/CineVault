package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListMedia(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	media, err := s.mediaRepo.ListByLibrary(libraryID, 100, 0)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list media")
		return
	}

	count, _ := s.mediaRepo.CountByLibrary(libraryID)

	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"items": media,
			"total": count,
		},
	})
}

func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	media, err := s.mediaRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: media})
}

func (s *Server) handleUpdateMedia(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	var req struct {
		Title         string   `json:"title"`
		SortTitle     *string  `json:"sort_title"`
		OriginalTitle *string  `json:"original_title"`
		Description   *string  `json:"description"`
		Year          *int     `json:"year"`
		ReleaseDate   *string  `json:"release_date"`
		Rating        *float64 `json:"rating"`
		EditionType   *string  `json:"edition_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		s.respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	if err := s.mediaRepo.UpdateMediaFields(id, req.Title, req.SortTitle, req.OriginalTitle, req.Description, req.Year, req.ReleaseDate, req.Rating, req.EditionType); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return updated item
	media, err := s.mediaRepo.GetByID(id)
	if err != nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: media})
}

func (s *Server) handleResetMediaLock(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	if err := s.mediaRepo.ResetMetadataLock(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}

func (s *Server) handleGetMediaEdition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	item, err := s.editionRepo.GetEditionItemByMediaID(id)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if item == nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
			"has_edition": false,
		}})
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"has_edition":    true,
		"edition_type":   item.EditionType,
		"edition_id":     item.ID,
		"edition_group_id": item.EditionGroupID,
		"custom_edition_name": item.CustomEditionName,
	}})
}

func (s *Server) handleSearchMedia(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.respondError(w, http.StatusBadRequest, "missing search query")
		return
	}

	// Get searchable library IDs based on user access and include_in_search setting
	userID := s.getUserID(r)
	role := models.UserRole(r.Header.Get("X-User-Role"))
	searchableIDs, err := s.libRepo.ListSearchableLibraryIDs(userID, role)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "search failed")
		return
	}

	media, err := s.mediaRepo.SearchInLibraries(query, searchableIDs, 50)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "search failed")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: media})
}
