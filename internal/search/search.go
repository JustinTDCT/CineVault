package search

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/httputil"
	"github.com/JustinTDCT/CineVault/internal/media"
)

type Handler struct {
	db        *sql.DB
	mediaRepo *media.Repository
}

func NewHandler(db *sql.DB, mediaRepo *media.Repository) *Handler {
	return &Handler{db: db, mediaRepo: mediaRepo}
}

func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.search)
	return r
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		httputil.WriteError(w, http.StatusBadRequest, "MISSING_QUERY", "q parameter required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 25
	}

	results, err := h.mediaRepo.Search(query, limit)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "search failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}
