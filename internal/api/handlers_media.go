package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
)

// parseMediaFilter extracts filter/sort params from the request query string.
func parseMediaFilter(r *http.Request) *repository.MediaFilter {
	q := r.URL.Query()
	f := &repository.MediaFilter{
		Genre:         q.Get("genre"),
		Folder:        q.Get("folder"),
		ContentRating: q.Get("content_rating"),
		Edition:       q.Get("edition"),
		Sort:          q.Get("sort"),
		Order:         q.Get("order"),
	}
	// Only return a filter if at least one field is set
	if f.Genre == "" && f.Folder == "" && f.ContentRating == "" && f.Edition == "" && f.Sort == "" && f.Order == "" {
		return nil
	}
	return f
}

func (s *Server) handleListMedia(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	limit := 200
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	f := parseMediaFilter(r)

	media, err := s.mediaRepo.ListByLibraryFiltered(libraryID, limit, offset, f)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list media")
		return
	}

	// Enrich items with edition group info (edition_count, edition_group_id)
	_ = s.mediaRepo.PopulateEditionCounts(media)

	count, _ := s.mediaRepo.CountByLibraryFiltered(libraryID, f)

	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"items":  media,
			"total":  count,
			"limit":  limit,
			"offset": offset,
		},
	})
}

func (s *Server) handleMediaLetterIndex(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	f := parseMediaFilter(r)

	index, err := s.mediaRepo.LetterIndexFiltered(libraryID, f)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to build letter index")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: index})
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

	// Enrich with edition info
	_ = s.mediaRepo.PopulateEditionCounts([]*models.MediaItem{media})

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

// handleSetEditionParent links a media item as a child edition of a parent item.
func (s *Server) handleSetEditionParent(w http.ResponseWriter, r *http.Request) {
	childID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	var req struct {
		ParentID    string `json:"parent_id"`
		EditionType string `json:"edition_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	parentID, err := uuid.Parse(req.ParentID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid parent_id")
		return
	}

	if childID == parentID {
		s.respondError(w, http.StatusBadRequest, "cannot set item as its own parent")
		return
	}

	userID := s.getUserID(r)
	if err := s.editionRepo.SetParent(childID, parentID, req.EditionType, userID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to set edition parent: "+err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "edition parent set"}})
}

// handleRemoveEditionParent removes a media item from its edition group.
func (s *Server) handleRemoveEditionParent(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	if err := s.editionRepo.RemoveFromGroup(mediaID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to remove from edition group: "+err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "removed from edition group"}})
}
