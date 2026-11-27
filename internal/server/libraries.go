package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/scan"
)

type createLibraryRequest struct {
	Name      string   `json:"name"`
	MediaType string   `json:"media_type"`
	Paths     []string `json:"paths"`
}

func (s *Server) listLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := s.opt.Libraries.Libraries(r.Context())
	if err != nil {
		s.internalError(w, "list libraries", err)
		return
	}
	writeJSON(w, http.StatusOK, libs)
}

// createLibrary adds a library. Body: {"name", "media_type", "paths": [...]}.
func (s *Server) createLibrary(w http.ResponseWriter, r *http.Request) {
	var req createLibraryRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be JSON with name, media_type and paths")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	switch {
	case req.Name == "":
		writeError(w, http.StatusBadRequest, "name is required")
		return
	case !catalog.ValidMediaType(req.MediaType):
		writeError(w, http.StatusBadRequest, "media_type must be one of movie, tv, music, audiobook, book")
		return
	case len(req.Paths) == 0:
		writeError(w, http.StatusBadRequest, "paths needs at least one folder")
		return
	}
	paths := make([]string, 0, len(req.Paths))
	for _, p := range req.Paths {
		if !filepath.IsAbs(p) {
			writeError(w, http.StatusBadRequest, "folder paths must be absolute: "+p)
			return
		}
		p = filepath.Clean(p)
		if !s.opt.IsDir(p) {
			writeError(w, http.StatusBadRequest, "folder not found on the server: "+p)
			return
		}
		paths = append(paths, p)
	}

	lib, err := s.opt.Libraries.CreateLibrary(r.Context(), catalog.Library{Name: req.Name, MediaType: req.MediaType, Paths: paths})
	if errors.Is(err, catalog.ErrConflict) {
		writeError(w, http.StatusConflict, "a library named "+strconv.Quote(req.Name)+" already exists")
		return
	}
	if err != nil {
		s.internalError(w, "create library", err)
		return
	}
	writeJSON(w, http.StatusCreated, lib)
}

// library loads the {id} in the path, writing the error response if it fails.
func (s *Server) library(w http.ResponseWriter, r *http.Request) (catalog.Library, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "library id must be a positive number")
		return catalog.Library{}, false
	}
	lib, err := s.opt.Libraries.Library(r.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "library not found")
		return lib, false
	}
	if err != nil {
		s.internalError(w, "load library", err)
		return lib, false
	}
	return lib, true
}

func (s *Server) libraryTitles(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.library(w, r)
	if !ok {
		return
	}
	titles, err := s.opt.Libraries.LibraryTitles(r.Context(), lib.ID)
	if err != nil {
		s.internalError(w, "list titles", err)
		return
	}
	writeJSON(w, http.StatusOK, titles)
}

// titleEpisodes lists a show's seasons and episodes, including episodes the
// library has no file for. A movie has none.
func (s *Server) titleEpisodes(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "title id must be a positive number")
		return
	}
	seasons, err := s.opt.Libraries.TitleEpisodes(r.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "title not found")
		return
	}
	if err != nil {
		s.internalError(w, "list episodes", err)
		return
	}
	writeJSON(w, http.StatusOK, seasons)
}

// albumTracks lists an album's tracks and their files, per edition.
func (s *Server) albumTracks(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "title id must be a positive number")
		return
	}
	eds, err := s.opt.Libraries.AlbumTracks(r.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "title not found")
		return
	}
	if err != nil {
		s.internalError(w, "list tracks", err)
		return
	}
	writeJSON(w, http.StatusOK, eds)
}

// getProgress returns where playback of a title stopped. A title that
// hasn't been played answers with version_id 0.
func (s *Server) getProgress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "title id must be a positive number")
		return
	}
	p, _, err := s.opt.Libraries.Progress(r.Context(), id)
	if err != nil {
		s.internalError(w, "load progress", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// putProgress saves where playback stopped: {"version_id": 5, "position_seconds": 812.4}.
func (s *Server) putProgress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "title id must be a positive number")
		return
	}
	var p catalog.Progress
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&p); err != nil || p.VersionID <= 0 || p.PositionSeconds < 0 {
		writeError(w, http.StatusBadRequest, `send {"version_id": <file id>, "position_seconds": <seconds>}`)
		return
	}
	saved, err := s.opt.Libraries.SaveProgress(r.Context(), id, p)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "that file isn't part of this title")
		return
	}
	if err != nil {
		s.internalError(w, "save progress", err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type scanResponse struct {
	Library    catalog.Library `json:"library"`
	DurationMS int64           `json:"duration_ms"`
	scan.Result
	// Match is the TMDb matching run after the scan; nil when TMDb isn't configured.
	Match *matchSummary `json:"match"`
}

type matchSummary struct {
	match.Result
	// Error is set when matching stopped early, e.g. a rejected TMDb token.
	Error string `json:"error,omitempty"`
}

// scanLibrary scans a library now and returns the summary. Only one scan runs
// at a time; a second request gets 409 Conflict.
func (s *Server) scanLibrary(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.library(w, r)
	if !ok {
		return
	}
	if !s.scanning.TryLock() {
		writeError(w, http.StatusConflict, "a scan is already running; try again when it finishes")
		return
	}
	defer s.scanning.Unlock()

	start := time.Now()
	res, err := s.opt.Scan(r.Context(), lib)
	if err != nil {
		s.opt.Log.Error("scan failed", "library", lib.Name, "error", err)
		writeError(w, http.StatusInternalServerError, "scan failed: "+err.Error())
		return
	}
	s.opt.Log.Info("scan finished", "library", lib.Name, "seen", res.FilesSeen, "added", res.Added,
		"updated", res.Updated, "unchanged", res.Unchanged, "missing", res.Missing, "errors", len(res.Errors))

	resp := scanResponse{Library: lib, Result: res}
	if s.opt.Match != nil && usesTMDb(lib.MediaType) {
		resp.Match = s.runMatch(r, lib)
	}
	resp.DurationMS = time.Since(start).Milliseconds()
	writeJSON(w, http.StatusOK, resp)
}

// matchLibrary matches unmatched titles to TMDb without scanning files.
func (s *Server) matchLibrary(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.library(w, r)
	if !ok {
		return
	}
	if s.opt.Match == nil {
		writeError(w, http.StatusServiceUnavailable, "TMDb isn't configured; set CINEVAULT_TMDB_TOKEN")
		return
	}
	if !usesTMDb(lib.MediaType) {
		writeError(w, http.StatusBadRequest, "only movie and TV libraries are matched to TMDb")
		return
	}
	if !s.scanning.TryLock() {
		writeError(w, http.StatusConflict, "a scan is already running; try again when it finishes")
		return
	}
	defer s.scanning.Unlock()
	writeJSON(w, http.StatusOK, s.runMatch(r, lib))
}

// usesTMDb reports whether a library's titles are matched to TMDb.
func usesTMDb(mediaType string) bool { return mediaType == "movie" || mediaType == "tv" }

func (s *Server) runMatch(r *http.Request, lib catalog.Library) *matchSummary {
	res, err := s.opt.Match(r.Context(), lib.ID)
	out := &matchSummary{Result: res}
	if out.Messages == nil {
		out.Messages = []string{}
	}
	if err != nil {
		out.Error = err.Error()
		s.opt.Log.Error("match failed", "library", lib.Name, "error", err)
	} else {
		s.opt.Log.Info("match finished", "library", lib.Name, "matched", res.Matched, "not_found", res.NotFound, "errors", res.Errors)
	}
	return out
}

func (s *Server) internalError(w http.ResponseWriter, what string, err error) {
	s.opt.Log.Error(what, "error", err)
	writeError(w, http.StatusInternalServerError, "could not "+what)
}
