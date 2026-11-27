package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestServesAppAndFallsBackToIndex(t *testing.T) {
	h := handler(fstest.MapFS{
		"index.html":         {Data: []byte("<div id=root></div>")},
		"assets/index-a1.js": {Data: []byte("console.log(1)")},
	})

	for _, p := range []string{"/", "/libraries/1", "/libraries/1/titles/2", "/setup"} {
		rec := get(h, p)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "root") || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: code=%d cache=%q body=%q", p, rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
		}
	}

	rec := get(h, "/assets/index-a1.js")
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset: code=%d cache=%q", rec.Code, rec.Header().Get("Cache-Control"))
	}

	rec = get(h, "/api/nope")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "no such API endpoint") {
		t.Errorf("api 404: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNotBuilt(t *testing.T) {
	rec := get(handler(fstest.MapFS{".gitkeep": {}}), "/")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "make web") {
		t.Errorf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEmbeddedFSLoads(t *testing.T) {
	_ = Handler() // must not panic, whether or not the app is built
}
