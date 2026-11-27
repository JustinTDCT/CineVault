package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/probe"
)

var sampleTypes = []catalog.EditionType{
	{ID: "broadcast", DisplayName: "Broadcast", MediaTypes: []string{"tv"}, IsDefault: true},
	{ID: "theatrical", DisplayName: "Theatrical", MediaTypes: []string{"movie"}, IsDefault: true},
	{ID: "directors_cut", DisplayName: "Director's Cut", MediaTypes: []string{"movie", "tv"}},
	{ID: "fan_edit", DisplayName: "Fan Edit", MediaTypes: []string{"movie"}, Hidden: true},
}

func newServer(pingErr error, typesErr error, ffmpeg bool) *Server {
	return New(Options{
		Version:       "test",
		DBAddr:        "127.0.0.1:5432",
		SchemaVersion: 1,
		Caps:          Capabilities{FFmpeg: probe.FFmpegInfo{Found: ffmpeg}},
		PingDB:        func(context.Context) error { return pingErr },
		EditionTypes: func(context.Context) ([]catalog.EditionType, error) {
			return sampleTypes, typesErr
		},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func do(t *testing.T, s *Server, method, target string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec.Code, rec.Body.String()
}

func TestHealthOK(t *testing.T) {
	code, body := do(t, newServer(nil, nil, true), "GET", "/api/health")
	var h healthResponse
	if err := json.Unmarshal([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK || h.Status != "ok" || !h.Database.Reachable || h.Database.SchemaVersion != 1 || h.Metadata.TMDbConfigured {
		t.Errorf("code=%d health=%+v", code, h)
	}
}

func TestHealthDegradedWhenDatabaseDown(t *testing.T) {
	code, body := do(t, newServer(errors.New("connection refused"), nil, true), "GET", "/api/health")
	if code != http.StatusServiceUnavailable || !strings.Contains(body, `"degraded"`) {
		t.Errorf("code=%d body=%s", code, body)
	}
	if strings.Contains(body, "refused") {
		t.Error("raw database errors should not be exposed")
	}
}

func TestHealthDegradedWithoutFFmpeg(t *testing.T) {
	if code, _ := do(t, newServer(nil, nil, false), "GET", "/api/health"); code != http.StatusServiceUnavailable {
		t.Errorf("code=%d", code)
	}
}

func ids(t *testing.T, body string) string {
	t.Helper()
	var list []catalog.EditionType
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	var out []string
	for _, e := range list {
		out = append(out, e.ID)
	}
	return strings.Join(out, ",")
}

func TestEditionTypes(t *testing.T) {
	s := newServer(nil, nil, true)
	cases := map[string]string{
		"/api/edition-types":                                      "broadcast,theatrical,directors_cut",
		"/api/edition-types?media_type=tv":                        "broadcast,directors_cut",
		"/api/edition-types?media_type=movie":                     "theatrical,directors_cut",
		"/api/edition-types?media_type=movie&include_hidden=true": "theatrical,directors_cut,fan_edit",
		"/api/edition-types?media_type=book":                      "",
	}
	for target, want := range cases {
		code, body := do(t, s, "GET", target)
		if code != http.StatusOK || ids(t, body) != want {
			t.Errorf("%s: code=%d ids=%q want %q", target, code, ids(t, body), want)
		}
	}
}

func TestEditionTypesErrors(t *testing.T) {
	if code, body := do(t, newServer(nil, nil, true), "GET", "/api/edition-types?media_type=podcast"); code != http.StatusBadRequest || !strings.Contains(body, "media_type") {
		t.Errorf("bad media type: code=%d body=%s", code, body)
	}
	code, body := do(t, newServer(nil, errors.New("pq: relation missing"), true), "GET", "/api/edition-types")
	if code != http.StatusInternalServerError || strings.Contains(body, "relation") {
		t.Errorf("db error: code=%d body=%s", code, body)
	}
}

func TestOnlyGETAllowed(t *testing.T) {
	if code, _ := do(t, newServer(nil, nil, true), "POST", "/api/health"); code != http.StatusMethodNotAllowed {
		t.Errorf("POST code = %d", code)
	}
}
