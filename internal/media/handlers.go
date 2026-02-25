package media

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/httputil"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{id}", h.getByID)
	r.Get("/{id}/children", h.children)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	libID := r.URL.Query().Get("library_id")
	if libID == "" {
		httputil.WriteError(w, http.StatusBadRequest, "MISSING_PARAM", "library_id required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	params := ListParams{
		LibraryID: libID,
		Limit:     limit,
		SortBy:    r.URL.Query().Get("sort"),
		SortDir:   r.URL.Query().Get("dir"),
		Cursor:    r.URL.Query().Get("cursor"),
	}

	items, err := h.repo.ListByLibrary(params)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list media")
		return
	}

	count, _ := h.repo.CountByLibrary(libID)
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": count,
	})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "media item not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) children(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	items, err := h.repo.ListChildren(id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list children")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.repo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "media item not found")
		return
	}

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		ReleaseYear *int    `json:"release_year"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.Title != nil {
		item.Title = req.Title
	}

	httputil.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete media")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}
