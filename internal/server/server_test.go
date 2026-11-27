package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/probe"
)

func get(t *testing.T, s *Server) (int, map[string]any, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	return rec.Code, body, rec.Body.String()
}

func TestHealthOK(t *testing.T) {
	s := New("test", "127.0.0.1:5432", Capabilities{
		FFmpeg: probe.FFmpegInfo{Found: true, Encoders: []probe.Encoder{{Name: "hevc_nvenc", Codec: "hevc", Usable: true}}},
		GPUs:   []probe.GPU{{Name: "NVIDIA A2"}},
	})
	s.dial = func(context.Context, string) error { return nil }
	code, body, _ := get(t, s)
	if code != http.StatusOK || body["status"] != "ok" {
		t.Errorf("code=%d status=%v", code, body["status"])
	}
}

func TestHealthDegradedWhenDatabaseDown(t *testing.T) {
	s := New("test", "127.0.0.1:5432", Capabilities{FFmpeg: probe.FFmpegInfo{Found: true}})
	s.dial = func(context.Context, string) error { return errors.New("connection refused") }
	code, body, raw := get(t, s)
	if code != http.StatusServiceUnavailable || body["status"] != "degraded" {
		t.Errorf("code=%d status=%v", code, body["status"])
	}
	if strings.Contains(raw, "refused") {
		t.Error("raw dial errors should not be exposed")
	}
}

func TestHealthDegradedWithoutFFmpeg(t *testing.T) {
	s := New("test", "127.0.0.1:5432", Capabilities{})
	s.dial = func(context.Context, string) error { return nil }
	if code, body, _ := get(t, s); code != http.StatusServiceUnavailable || body["status"] != "degraded" {
		t.Errorf("code=%d status=%v", code, body["status"])
	}
}

func TestOnlyGETAllowed(t *testing.T) {
	s := New("test", "127.0.0.1:5432", Capabilities{})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST code = %d", rec.Code)
	}
}
