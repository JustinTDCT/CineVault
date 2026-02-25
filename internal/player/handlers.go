package player

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/httputil"
	"github.com/JustinTDCT/CineVault/internal/media"
)

type Handler struct {
	mediaRepo  *media.Repository
	transcoder *Transcoder
	cfg        *config.Config
}

func NewHandler(mediaRepo *media.Repository, cfg *config.Config) *Handler {
	tc := NewTranscoder(cfg.FFmpegPath, cfg.DataDir, cfg.HWAccelType, cfg.MaxTranscodes)
	return &Handler{mediaRepo: mediaRepo, transcoder: tc, cfg: cfg}
}

func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Get("/stream/{id}", h.stream)
	r.Post("/transcode/{id}", h.startTranscode)
	r.Delete("/transcode/{sessionID}", h.stopTranscode)
	r.Get("/subtitles/{id}", h.subtitles)
	r.Get("/hls/{sessionID}/*", h.serveHLS)
	return r
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.mediaRepo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "media item not found")
		return
	}
	ServeDirectPlay(w, r, item.FilePath)
}

func (h *Handler) startTranscode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.mediaRepo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "media item not found")
		return
	}

	var req struct {
		Quality string `json:"quality"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil || req.Quality == "" {
		req.Quality = "720p"
	}

	if !h.transcoder.CanStart() {
		httputil.WriteError(w, http.StatusServiceUnavailable, "MAX_TRANSCODES", "all transcode slots in use")
		return
	}

	session, err := h.transcoder.Start(id, item.FilePath, req.Quality)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "TRANSCODE_ERROR", err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusAccepted, map[string]string{
		"session_id": session.ID,
		"playlist":   "/api/player/hls/" + session.ID + "/stream.m3u8",
	})
}

func (h *Handler) stopTranscode(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	h.transcoder.Stop(sessionID)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"stopped": sessionID})
}

func (h *Handler) subtitles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.mediaRepo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "media item not found")
		return
	}

	tracks, err := ListSubtitles(h.cfg.FFprobePath, item.FilePath)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list subtitles")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, tracks)
}

func (h *Handler) serveHLS(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	file := chi.URLParam(r, "*")
	path := filepath.Join(h.cfg.DataDir, "transcode", sessionID, file)
	http.ServeFile(w, r, path)
}
