package musicbrainz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchRecordings(t *testing.T) {
	var query, agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query, agent = r.URL.Query().Get("query"), r.Header.Get("User-Agent")
		if r.URL.Path != "/recording" || r.URL.Query().Get("fmt") != "json" {
			t.Errorf("request %s", r.URL)
		}
		w.Write([]byte(`{"recordings":[{"id":"4a1b6c7e-2d3f-4e5a-9b8c-7d6e5f4a3b2c","title":"Take On Me","score":100,
			"first-release-date":"1984-10-19","artist-credit":[{"name":"a-ha","joinphrase":""}],
			"releases":[{"title":"Hunting High and Low","status":"Official","date":"1985-06-01",
			  "release-group":{"primary-type":"Album","first-release-date":"1985-06-01"}}]}]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, UserAgent: "CineVault/test", Every: time.Millisecond}
	recs, err := c.SearchRecordings(context.Background(), `a-ha`, `Take On "Me"`)
	if err != nil || len(recs) != 1 {
		t.Fatalf("%+v %v", recs, err)
	}
	if query != `recording:"Take On \"Me\"" AND artist:"a-ha"` || agent != "CineVault/test" {
		t.Errorf("query %q agent %q", query, agent)
	}
	r := recs[0]
	if r.Artist != "a-ha" || r.Score != 100 || len(r.Releases) != 1 || r.Releases[0].PrimaryType != "Album" {
		t.Errorf("recording %+v", r)
	}
}

func TestLimiterSpacesRequests(t *testing.T) {
	c := &Client{Every: 30 * time.Millisecond}
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := c.wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 55*time.Millisecond {
		t.Errorf("three requests in %v", d)
	}
}

func TestRetries503(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			w.Header().Set("Retry-After", "nonsense")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, UserAgent: "t", Every: time.Millisecond, Backoff: time.Millisecond}
	if _, err := c.SearchRecordings(context.Background(), "a", "b"); err != nil || calls != 3 {
		t.Fatalf("calls %d err %v", calls, err)
	}
	calls = -100 // always 503
	if _, err := c.SearchRecordings(context.Background(), "a", "b"); err == nil || calls != -100+retries+1 {
		t.Fatalf("calls %d err %v", calls+100, err)
	}
	if retryAfter("5") != 5*time.Second || retryAfter("600") != 30*time.Second || retryAfter("") != -1 {
		t.Error("retryAfter")
	}
}
