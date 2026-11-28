package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/cachesync"
	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
)

type fakeCacheService struct {
	mu       sync.Mutex
	settings cachesync.Settings
	token    string
	env      bool
	checks   int
}

func (f *fakeCacheService) Status(context.Context) (cachesync.Status, error) {
	return cachesync.Status{Settings: f.settings, TokenSource: map[bool]string{true: "saved"}[f.token != ""]}, nil
}
func (f *fakeCacheService) LoadSettings(context.Context) (cachesync.Settings, error) {
	return f.settings, nil
}
func (f *fakeCacheService) SaveSettings(_ context.Context, st cachesync.Settings) error {
	f.settings = st
	return nil
}
func (f *fakeCacheService) SetToken(_ context.Context, t string) (string, error) {
	switch {
	case f.env:
		return "", cachesync.ErrTokenFromEnvironment
	case !strings.HasPrefix(t, "cvc_"):
		return "", cachesync.ErrTokenShape
	case strings.Contains(t, "BAD"):
		return "", cachesync.ErrTokenRejected
	}
	f.token = t
	return "", nil
}
func (f *fakeCacheService) ClearToken(context.Context) error {
	if f.env {
		return cachesync.ErrTokenFromEnvironment
	}
	f.token = ""
	return nil
}
func (f *fakeCacheService) Check(context.Context) (cachesync.CheckResult, error) {
	f.mu.Lock()
	f.checks++
	f.mu.Unlock()
	return cachesync.CheckResult{}, nil
}
func (f *fakeCacheService) Report(context.Context, cacheapi.Key, cacheapi.Report) (cacheapi.ReportResponse, error) {
	return cacheapi.ReportResponse{}, nil
}

func cacheServer(f *fakeCacheService) *Server {
	return New(Options{Cache: f, PingDB: func(context.Context) error { return nil }, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func sendCache(s *Server, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestCacheTokenEndpoints(t *testing.T) {
	f := &fakeCacheService{settings: cachesync.Settings{Enabled: true, CheckInterval: "24h", Share: true}}
	s := cacheServer(f)
	for body, want := range map[string]int{
		`{"token":"nope"}`:                  http.StatusBadRequest,
		`{"token":"cvc_BADBADBADBADBADBA"}`: http.StatusUnprocessableEntity,
		`not json`:                          http.StatusBadRequest,
		`{"token":"cvc_GOODGOODGOODGOOD"}`:  http.StatusOK,
	} {
		if rec := sendCache(s, http.MethodPut, "/api/cache/token", body); rec.Code != want {
			t.Errorf("%s: %d, want %d", body, rec.Code, want)
		}
	}
	if f.token != "cvc_GOODGOODGOODGOOD" {
		t.Errorf("saved %q", f.token)
	}
	// GET /api/cache never carries the token.
	if rec := sendCache(s, http.MethodGet, "/api/cache", ""); strings.Contains(rec.Body.String(), "GOOD") {
		t.Errorf("status leaked the token: %s", rec.Body)
	}
	if rec := sendCache(s, http.MethodDelete, "/api/cache/token", ""); rec.Code != http.StatusNoContent || f.token != "" {
		t.Errorf("delete: %d", rec.Code)
	}
	f.env = true
	if rec := sendCache(s, http.MethodPut, "/api/cache/token", `{"token":"cvc_GOODGOODGOODGOOD"}`); rec.Code != http.StatusConflict {
		t.Errorf("over the environment: %d", rec.Code)
	}
}

func TestCacheSettingsKeepsOmittedFields(t *testing.T) {
	f := &fakeCacheService{settings: cachesync.Settings{Enabled: true, CheckInterval: "6h", Share: true}}
	s := cacheServer(f)
	rec := sendCache(s, http.MethodPut, "/api/cache/settings", `{"enabled": false}`)
	var got cachesync.Settings
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusOK || got != (cachesync.Settings{Enabled: false, CheckInterval: "6h", Share: true}) {
		t.Errorf("%d %+v", rec.Code, got)
	}
	if rec := sendCache(s, http.MethodPut, "/api/cache/settings", `{"check_interval": "2h"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("2h: %d", rec.Code)
	}
}
