package settings

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
	r.Put("/", h.update)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	all, err := h.repo.GetAll()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to load settings")
		return
	}

	settingsMap := make(map[string]string)
	for _, s := range all {
		settingsMap[s.Key] = s.Value
	}
	httputil.WriteJSON(w, http.StatusOK, settingsMap)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	var req map[string]string
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	for key, value := range req {
		if err := h.repo.Set(key, value); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to save setting")
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
