package libraries

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"

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
	r.Get("/types", h.listTypes)
	r.Get("/browse", h.browseFolders)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Get("/{id}/permissions", h.getPermissions)
	r.Put("/{id}/permissions", h.setPermissions)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	libs, err := h.repo.ListForUser(u.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list libraries")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, libs)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	var lib Library
	if err := httputil.ReadJSON(r, &lib); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if lib.Name == "" || !lib.LibraryType.Valid() {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_PARAMS", "name and valid library_type required")
		return
	}

	if err := h.repo.Create(&lib); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to create library")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, lib)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	lib, err := h.repo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, lib)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	id := chi.URLParam(r, "id")
	lib, err := h.repo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
		return
	}

	if err := httputil.ReadJSON(r, lib); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	lib.ID = id

	if err := h.repo.Update(lib); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to update library")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, lib)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete library")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *Handler) listTypes(w http.ResponseWriter, r *http.Request) {
	types := make([]map[string]interface{}, len(AllTypes))
	for i, t := range AllTypes {
		types[i] = map[string]interface{}{
			"value":     t,
			"label":     t.Label(),
			"is_video":  t.IsVideo(),
			"has_audio": t.HasAudio(),
			"has_metadata": t.HasMetadata(),
		}
	}
	httputil.WriteJSON(w, http.StatusOK, types)
}

func (h *Handler) getPermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	perms, err := h.repo.GetPermissions(id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to get permissions")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, perms)
}

func (h *Handler) setPermissions(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	id := chi.URLParam(r, "id")
	var req []struct {
		UserID     string          `json:"user_id"`
		Permission PermissionLevel `json:"permission"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	for _, p := range req {
		if err := h.repo.SetPermission(id, p.UserID, p.Permission); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to set permission")
			return
		}
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) browseFolders(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "INVALID_PATH", "cannot read directory")
		return
	}

	type folderEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
	}

	var folders []folderEntry
	for _, e := range entries {
		if e.IsDir() && !isHidden(e.Name()) {
			folders = append(folders, folderEntry{
				Name:  e.Name(),
				Path:  filepath.Join(path, e.Name()),
				IsDir: true,
			})
		}
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"current": path,
		"folders": folders,
	})
}

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
