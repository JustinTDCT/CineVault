package metadata

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/auth"
	"github.com/JustinTDCT/CineVault/internal/httputil"
	"github.com/JustinTDCT/CineVault/internal/libraries"
	"github.com/JustinTDCT/CineVault/internal/media"
)

type Handler struct {
	matcher   *Matcher
	mediaRepo *media.Repository
	libRepo   *libraries.Repository
}

func NewHandler(matcher *Matcher, mediaRepo *media.Repository, libRepo *libraries.Repository) *Handler {
	return &Handler{matcher: matcher, mediaRepo: mediaRepo, libRepo: libRepo}
}

func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Get("/search", h.search)
	r.Post("/match/{itemID}", h.applyMatch)
	r.Post("/refresh/{itemID}", h.refresh)
	return r
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	libType := libraries.LibraryType(r.URL.Query().Get("type"))
	yearStr := r.URL.Query().Get("year")

	if title == "" || !libType.Valid() {
		httputil.WriteError(w, http.StatusBadRequest, "MISSING_PARAMS", "title and type required")
		return
	}

	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	}

	results, err := h.matcher.ManualSearch(title, libType, year)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "metadata search failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, results)
}

func (h *Handler) applyMatch(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	itemID := chi.URLParam(r, "itemID")
	var req struct {
		CacheID string `json:"cache_id"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if err := h.matcher.ApplyMatch(itemID, req.CacheID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to apply match")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "matched"})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	itemID := chi.URLParam(r, "itemID")
	item, err := h.mediaRepo.GetByID(itemID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "media item not found")
		return
	}

	if item.CacheID == nil {
		httputil.WriteError(w, http.StatusBadRequest, "NO_CACHE_ID", "item has no cache server link")
		return
	}

	if err := h.matcher.cacheClient.RefreshRecord(*item.CacheID); err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "refresh request failed")
		return
	}

	httputil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "refresh queued"})
}
