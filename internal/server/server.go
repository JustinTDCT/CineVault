// Package server is CineVault's HTTP API.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/probe"
	"github.com/JustinTDCT/CineVault/internal/scan"
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
	// Libraries stores libraries and reads their titles.
	Libraries LibraryStore
	// Scan scans one library.
	Scan func(ctx context.Context, lib catalog.Library) (scan.Result, error)
	// Match matches a library's titles to TMDb; nil when no TMDb token is set.
	Match func(ctx context.Context, libraryID int64) (match.Result, error)
	// IsDir checks that a library folder exists; defaults to os.Stat.
	IsDir func(path string) bool
	// UI serves the web app for every other GET request; nil for none.
	UI http.Handler
	// Playback plays files; nil disables it.
	Playback *Playback
	Log      *slog.Logger
}

// LibraryStore is what the library endpoints need from the database.
type LibraryStore interface {
	CreateLibrary(ctx context.Context, lib catalog.Library) (catalog.Library, error)
	Libraries(ctx context.Context) ([]catalog.Library, error)
	Library(ctx context.Context, id int64) (catalog.Library, error)
	LibraryTitles(ctx context.Context, libraryID int64) ([]catalog.Title, error)
	// TitleEpisodes returns a show's seasons and episodes, or catalog.ErrNotFound.
	TitleEpisodes(ctx context.Context, titleID int64) ([]catalog.Season, error)
	// AlbumTracks returns an album's tracks per edition, or catalog.ErrNotFound.
	AlbumTracks(ctx context.Context, titleID int64) ([]catalog.AlbumEdition, error)
	// Progress and SaveProgress keep where playback of a title stopped.
	Progress(ctx context.Context, titleID int64) (catalog.Progress, bool, error)
	SaveProgress(ctx context.Context, titleID int64, p catalog.Progress) (catalog.Progress, error)
}

// Server serves the CineVault API.
type Server struct {
	opt Options
	// scanning allows one scan at a time.
	scanning sync.Mutex
}

// New creates a Server.
func New(opt Options) *Server {
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	if opt.IsDir == nil {
		opt.IsDir = func(path string) bool {
			st, err := os.Stat(path)
			return err == nil && st.IsDir()
		}
	}
	return &Server{opt: opt}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/edition-types", s.editionTypes)
	mux.HandleFunc("GET /api/libraries", s.listLibraries)
	mux.HandleFunc("POST /api/libraries", s.createLibrary)
	mux.HandleFunc("GET /api/libraries/{id}/titles", s.libraryTitles)
	mux.HandleFunc("POST /api/libraries/{id}/scan", s.scanLibrary)
	mux.HandleFunc("POST /api/libraries/{id}/match", s.matchLibrary)
	mux.HandleFunc("GET /api/titles/{id}/episodes", s.titleEpisodes)
	mux.HandleFunc("POST /api/playback", s.startPlayback)
	mux.HandleFunc("GET /api/playback/{session}/{file}", s.playbackFile)
	mux.HandleFunc("DELETE /api/playback/{session}", s.stopPlayback)
	mux.HandleFunc("GET /api/versions/{id}/file", s.versionFile)
	mux.HandleFunc("GET /api/versions/{id}/thumb", s.versionThumb)
	mux.HandleFunc("GET /api/versions/{id}/cover", s.versionCover)
	mux.HandleFunc("GET /api/titles/{id}/tracks", s.albumTracks)
	mux.HandleFunc("GET /api/titles/{id}/progress", s.getProgress)
	mux.HandleFunc("PUT /api/titles/{id}/progress", s.putProgress)
	if s.opt.UI != nil {
		// GET only, so a wrong method on an API route still gets 405.
		mux.Handle("GET /", s.opt.UI)
	}
	return mux
}

type dbStatus struct {
	Reachable     bool   `json:"reachable"`
	SchemaVersion int    `json:"schema_version"`
	Error         string `json:"error,omitempty"`
}

type playbackStatus struct {
	// Encoder is the H.264 encoder used for conversion, e.g. "h264_nvenc".
	Encoder string `json:"encoder,omitempty"`
	Device  string `json:"device,omitempty"`
	Ready   bool   `json:"ready"`
	Note    string `json:"note,omitempty"`
}

type metadataStatus struct {
	// TMDbConfigured is true when a TMDb token is set, so titles get matched.
	TMDbConfigured bool `json:"tmdb_configured"`
}

type healthResponse struct {
	Status    string         `json:"status"`
	Version   string         `json:"version"`
	Time      time.Time      `json:"time"`
	Database  dbStatus       `json:"database"`
	Metadata  metadataStatus `json:"metadata"`
	Playback  playbackStatus `json:"playback"`
	Transcode Capabilities   `json:"transcode"`
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
		Metadata:  metadataStatus{TMDbConfigured: s.opt.Match != nil},
	}
	if pb := s.opt.Playback; pb != nil {
		resp.Playback = playbackStatus{Ready: pb.EncoderOK, Note: pb.Note}
		if pb.EncoderOK {
			resp.Playback.Encoder, resp.Playback.Device = pb.Encoder.Name, pb.Encoder.DeviceName
		}
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
