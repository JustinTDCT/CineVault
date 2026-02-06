package api

import (
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
