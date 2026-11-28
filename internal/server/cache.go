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

// cacheSettings saves {"enabled": true, "check_interval": "24h", "share": true}.
// A field left out keeps its current value.
func (s *Server) cacheSettings(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	cur, err := s.opt.Cache.LoadSettings(r.Context())
	if err != nil {
		s.internalError(w, "load cache settings", err)
		return
	}
	var req struct {
		Enabled       *bool   `json:"enabled"`
		CheckInterval *string `json:"check_interval"`
		Share         *bool   `json:"share"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, `send {"enabled": true, "check_interval": "24h", "share": true}`)
		return
	}
	st := cur
	if req.Enabled != nil {
		st.Enabled = *req.Enabled
	}
	if req.CheckInterval != nil {
		st.CheckInterval = *req.CheckInterval
	}
	if req.Share != nil {
		st.Share = *req.Share
	}
	if _, ok := cachesync.Intervals[st.CheckInterval]; !ok {
		writeError(w, http.StatusBadRequest, "check_interval must be one of 1h, 6h, 24h, 168h or manual")
		return
	}
	if err := s.opt.Cache.SaveSettings(r.Context(), st); err != nil {
		s.internalError(w, "save cache settings", err)
		return
	}
	if st.Enabled && !cur.Enabled {
		s.startCacheCheck() // turned back on: catch up
	}
	out, err := s.opt.Cache.LoadSettings(r.Context())
	if err != nil {
		s.internalError(w, "load cache settings", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// cacheSetToken checks and saves a subscriber token: {"token": "cvc_…"}. The
// token is never sent back or logged. Answers {"note": "…"} when it was saved
// without being checked (the cache couldn't be reached).
func (s *Server) cacheSetToken(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, `send {"token": "cvc_…"}`)
		return
	}
	note, err := s.opt.Cache.SetToken(r.Context(), req.Token)
	switch {
	case errors.Is(err, cachesync.ErrTokenFromEnvironment):
		writeError(w, http.StatusConflict, "The token is set by CINEVAULT_CACHE_TOKEN. Remove it there to manage it in Settings.")
		return
	case errors.Is(err, cachesync.ErrTokenRejected):
		writeError(w, http.StatusUnprocessableEntity, "The CineVault Cache didn't accept this token. Check it was copied whole, and that it hasn't been revoked.")
		return
	case errors.Is(err, cachesync.ErrTokenShape):
		writeError(w, http.StatusBadRequest, "That doesn't look like a CineVault Cache token: it starts with cvc_ and is about 56 characters long.")
		return
	case err != nil:
		s.internalError(w, "save cache token", err)
		return
	}
	s.opt.Log.Info("cache token saved in Settings")
	s.startCacheCheck() // look titles up with the new subscription
	writeJSON(w, http.StatusOK, map[string]string{"note": note})
}

// cacheClearToken removes a token saved in Settings.
func (s *Server) cacheClearToken(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	if err := s.opt.Cache.ClearToken(r.Context()); err != nil {
		if errors.Is(err, cachesync.ErrTokenFromEnvironment) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		s.internalError(w, "remove cache token", err)
		return
	}
	s.opt.Log.Info("cache token removed in Settings")
	w.WriteHeader(http.StatusNoContent)
}

// cacheCheck starts "Check now". It runs in the background: a first check
// of a big library makes one lookup per title. GET /api/cache shows when it
// has finished.
func (s *Server) cacheCheck(w http.ResponseWriter, r *http.Request) {
	if s.opt.Cache == nil {
		writeError(w, http.StatusNotFound, "the CineVault Cache isn't set up on this server")
		return
	}
	s.startCacheCheck()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

// startCacheCheck runs a check in the background.
func (s *Server) startCacheCheck() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := s.opt.Cache.Check(ctx); err != nil && !errors.Is(err, cachesync.ErrBusy) {
			s.opt.Log.Warn("cache check failed", "error", err)
		}
	}()
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
