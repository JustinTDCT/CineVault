package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/cachesync"
	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
)

// cacheStatus is the CineVault Cache section of Settings.
func (s *Server) cacheStatus(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	st, err := s.opt.Cache.Status(r.Context())
	if err != nil {
		s.internalError(w, "cache status", err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// cacheSettings saves {"check_interval": "24h", "share": true}.
func (s *Server) cacheSettings(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	var st cachesync.Settings
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&st); err != nil {
		writeError(w, http.StatusBadRequest, `send {"check_interval": "24h", "share": true}`)
		return
	}
	if err := s.opt.Cache.SaveSettings(r.Context(), st); err != nil {
		if _, ok := cachesync.Intervals[st.CheckInterval]; !ok {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.internalError(w, "save cache settings", err)
		return
	}
	out, err := s.opt.Cache.LoadSettings(r.Context())
	if err != nil {
		s.internalError(w, "load cache settings", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// cacheCheck starts "Check now". It runs in the background: a first check
// of a big library makes one lookup per title. GET /api/cache shows when it
// has finished.
func (s *Server) cacheCheck(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := s.opt.Cache.Check(ctx); err != nil && !errors.Is(err, cachesync.ErrBusy) {
			s.opt.Log.Warn("cache check failed", "error", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

type reportRequest struct {
	Edition string `json:"edition"` // Edition List ID; empty for the whole title
	Field   string `json:"field"`
	Message string `json:"message"`
}

// cacheReport sends "Report a mistake" about a title's cache information.
func (s *Server) cacheReport(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil || s.opt.TitleCacheKey == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "title id must be a positive number")
		return
	}
	var req reportRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, `send {"edition": "directors_cut", "field": "runtime", "message": "..."}`)
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	key, ok, err := s.opt.TitleCacheKey(r.Context(), id)
	if err != nil {
		s.internalError(w, "title cache key", err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "this title has no CineVault Cache information to report on")
		return
	}
	res, err := s.opt.Cache.Report(r.Context(), key, cacheapi.Report{Edition: req.Edition, Field: req.Field, Message: req.Message})
	var apiErr *cacheapi.Error
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, res)
	case errors.As(err, &apiErr):
		code := http.StatusBadGateway
		if apiErr.StatusCode == http.StatusTooManyRequests {
			code = http.StatusTooManyRequests
		}
		writeError(w, code, "the CineVault Cache didn't take the report: "+apiErr.Message)
	case cacheapi.IsRetryable(err):
		writeError(w, http.StatusBadGateway, "can't reach the CineVault Cache; try again later")
	default: // a bad report, or no subscription
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
