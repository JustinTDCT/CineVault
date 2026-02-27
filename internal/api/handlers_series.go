package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Movie Series ────────────────────

func (s *Server) handleListSeries(w http.ResponseWriter, r *http.Request) {
	libIDStr := r.URL.Query().Get("library_id")
	if libIDStr == "" {
		s.respondError(w, http.StatusBadRequest, "library_id is required")
		return
	}
	libID, err := uuid.Parse(libIDStr)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library_id")
		return
	}

	list, err := s.seriesRepo.ListByLibrary(libID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list series")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: list})
}

func (s *Server) handleCreateSeries(w http.ResponseWriter, r *http.Request) {
	var series models.MovieSeries
	if err := json.NewDecoder(r.Body).Decode(&series); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if series.Name == "" {
		s.respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if series.LibraryID == uuid.Nil {
		s.respondError(w, http.StatusBadRequest, "library_id is required")
		return
	}
	series.ID = uuid.New()
	if err := s.seriesRepo.Create(&series); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create series: "+err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: series})
}

func (s *Server) handleGetSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}
	series, err := s.seriesRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "series not found")
		return
	}

	// Build rich items: full MediaItem data with edition grouping
	richItems, err := s.seriesRepo.ListItemsRich(id)
	if err == nil && len(richItems) > 0 {
		_ = s.mediaRepo.PopulateEditionCounts(richItems)
	}

	// Return both lightweight items (backward compat) and rich_items for the grid
	out := map[string]interface{}{
		"id":          series.ID,
		"library_id":  series.LibraryID,
		"name":        series.Name,
		"poster_path": series.PosterPath,
		"created_at":  series.CreatedAt,
		"updated_at":  series.UpdatedAt,
		"item_count":  series.ItemCount,
		"items":       series.Items,
		"rich_items":  richItems,
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: out})
}

func (s *Server) handleUpdateSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}
	var series models.MovieSeries
	if err := json.NewDecoder(r.Body).Decode(&series); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	series.ID = id
	if err := s.seriesRepo.Update(&series); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update series")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: series})
}

func (s *Server) handleDeleteSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}
	if err := s.seriesRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete series")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "series deleted"}})
}

// ──────────────────── Series Items ────────────────────

func (s *Server) handleAddSeriesItem(w http.ResponseWriter, r *http.Request) {
	seriesID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}
	var item models.MovieSeriesItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item.ID = uuid.New()
	item.SeriesID = seriesID
	if err := s.seriesRepo.AddItem(&item); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to add series item: "+err.Error())
		return
	}
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: item})
}

func (s *Server) handleRemoveSeriesItem(w http.ResponseWriter, r *http.Request) {
	seriesID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}
	if err := s.seriesRepo.RemoveItem(seriesID, itemID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to remove series item")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "item removed"}})
}

func (s *Server) handleGetMediaSeries(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}
	series, item, err := s.seriesRepo.GetSeriesForMedia(mediaID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get series info")
		return
	}
	if series == nil {
		s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{"in_series": false}})
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"in_series":  true,
		"series":     series,
		"sort_order": item.SortOrder,
		"item_id":    item.ID,
	}})
}
