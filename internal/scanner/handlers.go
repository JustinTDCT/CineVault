package scanner

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/auth"
	"github.com/JustinTDCT/CineVault/internal/httputil"
	"github.com/JustinTDCT/CineVault/internal/libraries"
)

type Handler struct {
	scanner *Scanner
	libRepo *libraries.Repository
}

func NewHandler(s *Scanner, libRepo *libraries.Repository) *Handler {
	return &Handler{scanner: s, libRepo: libRepo}
}

func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/library/{id}", h.scanLibrary)
	r.Get("/status/{id}", h.scanStatus)
	return r
}

func (h *Handler) scanLibrary(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || !u.IsAdmin {
		httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
		return
	}

	id := chi.URLParam(r, "id")
	lib, err := h.libRepo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
		return
	}

	go func() {
		h.scanner.ScanLibrary(lib.ID, lib.LibraryType, lib.Folders)
	}()

	httputil.WriteJSON(w, http.StatusAccepted, map[string]string{
		"status":     "scanning",
		"library_id": lib.ID,
	})
}

func (h *Handler) scanStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var state libraries.ScanState
	err := h.scanner.db.QueryRow(`
		SELECT id, library_id, last_scan_started, last_scan_completed,
		       files_scanned, files_added, files_removed, status
		FROM scan_state WHERE library_id=$1`, id,
	).Scan(&state.ID, &state.LibraryID, &state.LastScanStarted, &state.LastScanCompleted,
		&state.FilesScanned, &state.FilesAdded, &state.FilesRemoved, &state.Status)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "no scan state found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, state)
}
