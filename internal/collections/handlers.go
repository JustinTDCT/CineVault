package collections

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/auth"
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
	r.Post("/", h.create)
	r.Delete("/{id}", h.delete)
	r.Get("/{id}/items", h.getItems)
	r.Post("/{id}/items", h.addItem)
	r.Delete("/{id}/items/{itemID}", h.removeItem)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	cols, err := h.repo.ListByUser(u.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list collections")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, cols)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var col Collection
	if err := httputil.ReadJSON(r, &col); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	col.UserID = u.UserID
	if err := h.repo.Create(&col); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to create collection")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, col)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete collection")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *Handler) getItems(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	items, err := h.repo.GetItems(id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list items")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) addItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		MediaItemID string `json:"media_item_id"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	if err := h.repo.AddItem(id, req.MediaItemID, 0); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to add item")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, map[string]string{"added": req.MediaItemID})
}

func (h *Handler) removeItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemID")
	h.repo.RemoveItem(id, itemID)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"removed": itemID})
}
