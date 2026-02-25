package users

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/httputil"
	"github.com/JustinTDCT/CineVault/internal/auth"
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
	r.Get("/me", h.me)
	r.Get("/me/profile", h.getProfile)
	r.Put("/me/profile", h.updateProfile)
	r.Get("/pin-users", h.pinUsers)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}
	users, err := h.repo.List()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list users")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, users)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	user, err := h.repo.GetByID(u.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.repo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	caller := auth.UserFromContext(r.Context())
	if caller == nil || (caller.UserID != id && !caller.IsAdmin) {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "cannot edit this user")
		return
	}

	var req struct {
		FullName string  `json:"full_name"`
		Email    string  `json:"email"`
		PIN      *string `json:"pin"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	user, err := h.repo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.Email != "" {
		user.Email = auth.NormalizeEmail(req.Email)
	}
	if req.PIN != nil {
		user.PIN = req.PIN
	}

	if err := h.repo.Update(user); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to update user")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	caller := auth.UserFromContext(r.Context())
	if caller == nil || !caller.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete user")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	profile, err := h.repo.GetProfile(u.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, profile)
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	profile, err := h.repo.GetProfile(u.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
		return
	}

	if err := httputil.ReadJSON(r, profile); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if err := h.repo.UpdateProfile(profile); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to update profile")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, profile)
}

func (h *Handler) pinUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListForPINSwitch()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list users")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, users)
}
