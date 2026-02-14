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
		Source:        q.Get("source"),
		DynamicRange:  q.Get("dynamic_range"),
		Codec:         q.Get("codec"),
		HDRFormat:     q.Get("hdr_format"),
		Resolution:    q.Get("resolution"),
		AudioCodec:    q.Get("audio_codec"),
		BitrateRange:  q.Get("bitrate_range"),
		Country:       q.Get("country"),
		DurationRange: q.Get("duration_range"),
		WatchStatus:   q.Get("watch_status"),
		AddedDays:     q.Get("added_days"),
		YearFrom:      q.Get("year_from"),
		YearTo:        q.Get("year_to"),
		MinRating:     q.Get("min_rating"),
		Sort:          q.Get("sort"),
		Order:         q.Get("order"),
	}
	// Only return a filter if at least one field is set
	if f.Genre == "" && f.Folder == "" && f.ContentRating == "" && f.Edition == "" &&
		f.Source == "" && f.DynamicRange == "" && f.Codec == "" && f.HDRFormat == "" &&
		f.Resolution == "" && f.AudioCodec == "" && f.BitrateRange == "" &&
		f.Country == "" && f.DurationRange == "" && f.WatchStatus == "" && f.AddedDays == "" &&
		f.YearFrom == "" && f.YearTo == "" && f.MinRating == "" &&
		f.Sort == "" && f.Order == "" {
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

func (s *Server) handleGetMediaExtras(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	extras, err := s.mediaRepo.GetExtras(id)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to fetch extras")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: extras})
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
		CustomNotes   *string  `json:"custom_notes"`
		CustomTags    *string  `json:"custom_tags"`
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

	// Update custom notes if provided
	if req.CustomNotes != nil {
		_ = s.mediaRepo.UpdateCustomNotes(id, req.CustomNotes)
	}
	// Update custom tags if provided
	if req.CustomTags != nil {
		_ = s.mediaRepo.UpdateCustomTags(id, *req.CustomTags)
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

	// Enrich items with edition group info (edition_count, edition_group_id)
	_ = s.mediaRepo.PopulateEditionCounts(media)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: media})
}

// handleBulkUpdateMedia updates specific fields across multiple media items.
func (s *Server) handleBulkUpdateMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []uuid.UUID            `json:"ids"`
		Fields map[string]interface{} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.IDs) == 0 {
		s.respondError(w, http.StatusBadRequest, "no IDs provided")
		return
	}
	if len(req.Fields) == 0 {
		s.respondError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	// Handle tag add/remove modes that require per-item processing
	tagMode, _ := req.Fields["tag_mode"].(string)
	tagsValue, hasTags := req.Fields["custom_tags"]
	notesMode, _ := req.Fields["notes_mode"].(string)
	notesValue, hasNotes := req.Fields["custom_notes"]

	// Remove mode keys from the direct update fields
	delete(req.Fields, "tag_mode")
	delete(req.Fields, "notes_mode")

	// For tag add/remove, we need per-item processing
	if hasTags && (tagMode == "add" || tagMode == "remove") {
		delete(req.Fields, "custom_tags")
		tagsStr, _ := tagsValue.(string)
		newTags := splitTags(tagsStr)

		existing, err := s.mediaRepo.BulkGetCustomTags(req.IDs)
		if err == nil {
			for _, id := range req.IDs {
				current := parseTagsJSON(existing[id])
				if tagMode == "add" {
					current = mergeTags(current, newTags)
				} else {
					current = removeTags(current, newTags)
				}
				tagJSON := buildTagsJSON(current)
				_ = s.mediaRepo.UpdateCustomTags(id, tagJSON)
			}
		}
	} else if hasTags && tagMode == "replace" {
		// Convert tags string to JSON format for direct bulk update
		tagsStr, _ := tagsValue.(string)
		tags := splitTags(tagsStr)
		req.Fields["custom_tags"] = buildTagsJSON(tags)
	}

	// For notes append mode, process per-item
	if hasNotes && notesMode == "append" {
		delete(req.Fields, "custom_notes")
		notesStr, _ := notesValue.(string)
		if notesStr != "" {
			existing, err := s.mediaRepo.BulkGetCustomNotes(req.IDs)
			if err == nil {
				for _, id := range req.IDs {
					current := existing[id]
					if current != "" {
						current = current + "\n" + notesStr
					} else {
						current = notesStr
					}
					_ = s.mediaRepo.UpdateCustomNotes(id, &current)
				}
			}
		}
	}
	// For notes replace, leave in fields for direct update

	// Do direct bulk update for remaining simple fields
	if len(req.Fields) > 0 {
		if err := s.mediaRepo.BulkUpdateFields(req.IDs, req.Fields); err != nil {
			s.respondError(w, http.StatusInternalServerError, "bulk update failed: "+err.Error())
			return
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"message": "bulk update complete",
		"count":   len(req.IDs),
	}})
}

// handleBulkAction performs bulk actions on multiple media items.
func (s *Server) handleBulkAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []uuid.UUID            `json:"ids"`
		Action string                 `json:"action"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.IDs) == 0 {
		s.respondError(w, http.StatusBadRequest, "no IDs provided")
		return
	}

	userID := s.getUserID(r)

	switch req.Action {
	case "mark_played":
		for _, id := range req.IDs {
			wh := &models.WatchHistory{
				ID:          uuid.New(),
				UserID:      userID,
				MediaItemID: id,
				Completed:   true,
			}
			_ = s.watchRepo.Upsert(wh)
		}
	case "mark_unplayed":
		for _, id := range req.IDs {
			wh := &models.WatchHistory{
				ID:              uuid.New(),
				UserID:          userID,
				MediaItemID:     id,
				ProgressSeconds: 0,
				Completed:       false,
			}
			_ = s.watchRepo.Upsert(wh)
		}
	case "refresh_metadata":
		for _, id := range req.IDs {
			_ = s.mediaRepo.ResetMetadataLock(id)
		}
	case "delete":
		count, err := s.mediaRepo.BulkDelete(req.IDs)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "delete failed: "+err.Error())
			return
		}
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
			"message": "items deleted",
			"count":   count,
		}})
		return
	default:
		s.respondError(w, http.StatusBadRequest, "unknown action: "+req.Action)
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"message": req.Action + " complete",
		"count":   len(req.IDs),
	}})
}

// Tag helper functions for bulk operations
func splitTags(s string) []string {
	parts := []string{}
	for _, p := range splitComma(s) {
		t := trimSpace(p)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return parts
}

func splitComma(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	result = append(result, current)
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s) - 1
	for start <= end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end >= start && (s[end] == ' ' || s[end] == '\t' || s[end] == '\n') {
		end--
	}
	if start > end {
		return ""
	}
	return s[start : end+1]
}

func parseTagsJSON(s string) []string {
	if s == "" || s == "{}" {
		return []string{}
	}
	var obj struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return []string{}
	}
	return obj.Tags
}

func buildTagsJSON(tags []string) string {
	if len(tags) == 0 {
		return `{"tags":[]}`
	}
	b, _ := json.Marshal(map[string][]string{"tags": tags})
	return string(b)
}

func mergeTags(existing, add []string) []string {
	set := map[string]bool{}
	for _, t := range existing {
		set[t] = true
	}
	for _, t := range add {
		set[t] = true
	}
	result := make([]string, 0, len(set))
	for t := range set {
		result = append(result, t)
	}
	return result
}

func removeTags(existing, remove []string) []string {
	removeSet := map[string]bool{}
	for _, t := range remove {
		removeSet[t] = true
	}
	result := []string{}
	for _, t := range existing {
		if !removeSet[t] {
			result = append(result, t)
		}
	}
	return result
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
