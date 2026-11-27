package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/probe"
	"github.com/JustinTDCT/CineVault/internal/thumb"
	"github.com/JustinTDCT/CineVault/internal/transcode"
)

// VersionStore looks up files.
type VersionStore interface {
	Version(ctx context.Context, id int64) (catalog.Version, error)
}

// Streamer runs HLS sessions (transcode.Manager in production).
type Streamer interface {
	Start(src transcode.Source, plan transcode.Plan) (string, error)
	Playlist(id string) (string, error)
	Segment(ctx context.Context, id string, n int) (string, error)
	Stop(id string)
}

// Playback configures playing files.
type Playback struct {
	Versions VersionStore
	Streamer Streamer
	// Encoder is the H.264 encoder chosen at startup; EncoderOK is false when
	// none works, and Note explains a fallback or failure.
	Encoder   probe.Encoder
	EncoderOK bool
	Note      string
	// MaxHeight caps the converted picture height; 0 means no cap.
	MaxHeight int
	// Thumbs makes still images from videos; nil disables them.
	Thumbs *thumb.Maker
}

type playRequest struct {
	VersionID int64                `json:"version_id"`
	Caps      transcode.ClientCaps `json:"caps"`
}

type playResponse struct {
	Mode      string   `json:"mode"` // direct or hls
	URL       string   `json:"url"`
	SessionID string   `json:"session_id,omitempty"`
	Reasons   []string `json:"reasons"`
	// Device and Encoder say what's converting, e.g. "NVIDIA A2" and "h264_nvenc".
	Device  string `json:"device,omitempty"`
	Encoder string `json:"encoder,omitempty"`
}

// startPlayback decides how to play a file and, if it needs converting, opens
// an HLS session. Body: {"version_id": 2, "caps": {"containers": [...], "video": [...], "audio": [...]}}.
func (s *Server) startPlayback(w http.ResponseWriter, r *http.Request) {
	pb := s.opt.Playback
	if pb == nil {
		writeError(w, http.StatusServiceUnavailable, "playback isn't set up on this server")
		return
	}
	var req playRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil || req.VersionID <= 0 {
		writeError(w, http.StatusBadRequest, "request body must be JSON with version_id and caps")
		return
	}
	v, err := pb.Versions.Version(r.Context(), req.VersionID)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		s.internalError(w, "load file", err)
		return
	}
	if v.Missing {
		writeError(w, http.StatusConflict, "this file is missing from disk; check the drive or folder and rescan")
		return
	}

	plan := transcode.Decide(v, req.Caps, pb.Encoder, pb.MaxHeight)
	if plan.Mode == "direct" {
		writeJSON(w, http.StatusOK, playResponse{Mode: "direct", URL: "/api/versions/" + strconv.FormatInt(v.ID, 10) + "/file", Reasons: []string{}})
		return
	}
	if !pb.EncoderOK {
		writeError(w, http.StatusServiceUnavailable, "this file needs converting for this player, but no working video encoder was found: "+pb.Note)
		return
	}
	id, err := pb.Streamer.Start(transcode.Source{
		Path: v.Path, VideoCodec: v.VideoCodec, Height: v.Height, DurationSeconds: v.DurationSeconds,
	}, plan)
	if errors.Is(err, transcode.ErrBusy) {
		writeError(w, http.StatusServiceUnavailable, err.Error()+"; try again when another video stops")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start playback: "+err.Error())
		return
	}
	s.opt.Log.Info("playback started", "session", id, "file", filepath.Base(v.Path),
		"device", plan.Encoder.DeviceName, "reasons", strings.Join(plan.Reasons, "; "))
	writeJSON(w, http.StatusOK, playResponse{
		Mode: "hls", URL: "/api/playback/" + id + "/index.m3u8", SessionID: id,
		Reasons: plan.Reasons, Device: plan.Encoder.DeviceName, Encoder: plan.Encoder.Name,
	})
}

// playbackFile serves a session's playlist (index.m3u8) or a segment (segN.ts).
func (s *Server) playbackFile(w http.ResponseWriter, r *http.Request) {
	pb := s.opt.Playback
	if pb == nil {
		http.NotFound(w, r)
		return
	}
	id, file := r.PathValue("session"), r.PathValue("file")
	if file == "index.m3u8" {
		pl, err := pb.Streamer.Playlist(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "playback session not found; start playback again")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(pl))
		return
	}
	num, ok := strings.CutPrefix(file, "seg")
	num, ok2 := strings.CutSuffix(num, ".ts")
	n, err := strconv.Atoi(num)
	if !ok || !ok2 || err != nil {
		http.NotFound(w, r)
		return
	}
	path, err := pb.Streamer.Segment(r.Context(), id, n)
	switch {
	case errors.Is(err, transcode.ErrNoSession), errors.Is(err, transcode.ErrSegment):
		writeError(w, http.StatusNotFound, err.Error())
		return
	case r.Context().Err() != nil:
		return // the player gave up on this request
	case err != nil:
		s.opt.Log.Error("segment failed", "session", id, "segment", n, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, path)
}

func (s *Server) stopPlayback(w http.ResponseWriter, r *http.Request) {
	if s.opt.Playback != nil {
		s.opt.Playback.Streamer.Stop(r.PathValue("session"))
	}
	w.WriteHeader(http.StatusNoContent)
}

var directTypes = map[string]string{"mp4": "video/mp4", "m4v": "video/mp4", "mov": "video/mp4", "webm": "video/webm"}

// versionFile sends a file as-is, with range requests so players can seek.
// versionThumb serves a still from a video file, made on first request.
// Used for music videos and anything else without artwork.
func (s *Server) versionThumb(w http.ResponseWriter, r *http.Request) {
	pb := s.opt.Playback
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if pb == nil || pb.Thumbs == nil || err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	v, err := pb.Versions.Version(r.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		s.internalError(w, "load file", err)
		return
	}
	path, err := pb.Thumbs.Get(r.Context(), thumb.Source{VersionID: v.ID, Path: v.Path, SizeBytes: v.SizeBytes, DurationSeconds: v.DurationSeconds})
	if err != nil {
		if r.Context().Err() == nil {
			s.opt.Log.Warn("thumbnail failed", "file", v.Path, "error", err)
		}
		writeError(w, http.StatusNotFound, "no thumbnail for this file")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, path)
}

func (s *Server) versionFile(w http.ResponseWriter, r *http.Request) {
	pb := s.opt.Playback
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if pb == nil || err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	v, err := pb.Versions.Version(r.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		s.internalError(w, "load file", err)
		return
	}
	f, err := os.Open(v.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, "this file is missing from disk")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		s.internalError(w, "read file", err)
		return
	}
	if ct, ok := directTypes[v.Container]; ok {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, filepath.Base(v.Path), st.ModTime(), f)
}
