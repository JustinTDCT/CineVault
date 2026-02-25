package watchhistory

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
	r.Get("/continue", h.continueWatching)
	r.Get("/history", h.history)
	r.Post("/progress", h.updateProgress)
	r.Get("/position/{itemID}", h.getPosition)
	return r
}

func (h *Handler) continueWatching(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	entries, err := h.repo.ContinueWatching(u.UserID, 20)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to load continue watching")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, entries)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	entries, err := h.repo.History(u.UserID, 50)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to load history")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, entries)
}

func (h *Handler) updateProgress(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	var req WatchEntry
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	req.UserID = u.UserID

	if err := h.repo.Upsert(&req); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to update progress")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, req)
}

func (h *Handler) getPosition(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	itemID := chi.URLParam(r, "itemID")
	entry, err := h.repo.GetByUserAndItem(u.UserID, itemID)
	if err != nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]float64{"position_seconds": 0})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, entry)
}
