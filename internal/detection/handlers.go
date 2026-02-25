package detection

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/JustinTDCT/CineVault/internal/auth"
	"github.com/JustinTDCT/CineVault/internal/httputil"
	"github.com/JustinTDCT/CineVault/internal/media"
)

type Handler struct {
	detector  *Detector
	mediaRepo *media.Repository
}

func NewHandler(detector *Detector, mediaRepo *media.Repository) *Handler {
	return &Handler{detector: detector, mediaRepo: mediaRepo}
}

func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Get("/{id}", h.getSegments)
	r.Post("/{id}/detect", h.detect)
	r.Delete("/{id}", h.deleteAll)
	r.Delete("/segment/{segID}", h.deleteOne)
	return r
}

func (h *Handler) getSegments(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	info, err := h.detector.BuildSkipInfo(id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to get segments")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, info)
}

func (h *Handler) detect(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	id := chi.URLParam(r, "id")
	item, err := h.mediaRepo.GetByID(id)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "NOT_FOUND", "media item not found")
		return
	}

	var req struct {
		Types []SegmentType `json:"types"`
	}
	httputil.ReadJSON(r, &req)
	if len(req.Types) == 0 {
		req.Types = []SegmentType{SegmentIntro, SegmentCredits}
	}

	go func() {
		h.detector.DetectSegments(item.ID, item.FilePath, req.Types)
	}()

	httputil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "detection started"})
}

func (h *Handler) deleteAll(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.detector.DeleteSegments(id)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"deleted": "all segments"})
}

func (h *Handler) deleteOne(w http.ResponseWriter, r *http.Request) {
	segID := chi.URLParam(r, "segID")
	h.detector.DeleteSegment(segID)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"deleted": segID})
}
