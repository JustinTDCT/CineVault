// Package server is CineVault's HTTP API.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/probe"
)

// Capabilities is what the host can do for transcoding, probed at startup.
type Capabilities struct {
	FFmpeg probe.FFmpegInfo `json:"ffmpeg"`
	GPUs   []probe.GPU      `json:"gpus"`
}

// Options configures a Server. The function fields keep this package free of
// any database driver, so it can be tested with fakes.
type Options struct {
	Version string
	// DBAddr is shown in health errors; it never includes credentials.
	DBAddr string
	Caps   Capabilities
	// SchemaVersion is the database schema version after startup migrations.
	SchemaVersion int
	// PingDB checks the database is reachable.
	PingDB func(ctx context.Context) error
	// EditionTypes returns the whole Edition List.
	EditionTypes func(ctx context.Context) ([]catalog.EditionType, error)
	Log          *slog.Logger
}

// Server serves the CineVault API.
type Server struct {
	opt Options
}

// New creates a Server.
func New(opt Options) *Server {
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	return &Server{opt: opt}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/edition-types", s.editionTypes)
	return mux
}

type dbStatus struct {
	Reachable     bool   `json:"reachable"`
	SchemaVersion int    `json:"schema_version"`
	Error         string `json:"error,omitempty"`
}

type healthResponse struct {
	Status    string       `json:"status"`
	Version   string       `json:"version"`
	Time      time.Time    `json:"time"`
	Database  dbStatus     `json:"database"`
	Transcode Capabilities `json:"transcode"`
}

// health reports "ok" when the database is reachable and FFmpeg was found,
// and "degraded" otherwise. It never exposes credentials.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	resp := healthResponse{
		Status:    "ok",
		Version:   s.opt.Version,
		Time:      time.Now().UTC(),
		Transcode: s.opt.Caps,
		Database:  dbStatus{SchemaVersion: s.opt.SchemaVersion},
	}
	if err := s.opt.PingDB(ctx); err != nil {
		resp.Database.Error = "cannot connect to " + s.opt.DBAddr
		resp.Status = "degraded"
	} else {
		resp.Database.Reachable = true
	}
	if !s.opt.Caps.FFmpeg.Found {
		resp.Status = "degraded"
	}

	code := http.StatusOK
	if resp.Status != "ok" {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, resp)
}

// editionTypes lists the Edition List. Optional query parameters:
// media_type (movie, tv, music, audiobook, book) and include_hidden=true.
func (s *Server) editionTypes(w http.ResponseWriter, r *http.Request) {
	media := r.URL.Query().Get("media_type")
	if media != "" && !catalog.ValidMediaType(media) {
		writeError(w, http.StatusBadRequest, "media_type must be one of movie, tv, music, audiobook, book")
		return
	}
	includeHidden := r.URL.Query().Get("include_hidden") == "true"

	all, err := s.opt.EditionTypes(r.Context())
	if err != nil {
		s.opt.Log.Error("list edition types", "error", err)
		writeError(w, http.StatusInternalServerError, "could not read the Edition List")
		return
	}
	out := []catalog.EditionType{}
	for _, e := range all {
		if (media == "" || e.AppliesTo(media)) && (includeHidden || !e.Hidden) {
			out = append(out, e)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
