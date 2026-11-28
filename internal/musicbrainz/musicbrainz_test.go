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

func TestAlbumRule(t *testing.T) {
	// An EP counts like an album, and an exact date beats the same year given
	// alone ("Jar of Flies" before the "Jar of Flies / Sap" set).
	r := Recording{Artist: "Alice in Chains", Releases: []Release{
		{Title: "Jar of Flies / Sap", Status: "Official", PrimaryType: "Album", Date: "1994", Artist: "Alice in Chains"},
		{Title: "Jar of Flies", Status: "Official", PrimaryType: "EP", GroupDate: "1994-01-25", Artist: "Alice in Chains"},
		{Title: "No Excuses", Status: "Official", PrimaryType: "Single", GroupDate: "1994-01-10", Artist: "Alice in Chains"},
		{Title: "Greatest Hits", Status: "Official", PrimaryType: "Album", SecondaryTypes: []string{"Compilation"}, Date: "1993", Artist: "Alice in Chains"},
		{Title: "Grunge Hits", Status: "Official", PrimaryType: "Album", Date: "1993-05-01", Artist: "Various Artists"},
	}}
	if a, _ := r.Album(); a.Title != "Jar of Flies" {
		t.Errorf("album %q", a.Title)
	}
	for in, want := range map[string]string{"1994": "1994-99-99", "1994-01": "1994-01-99", "1994-01-25": "1994-01-25", "": "9999"} {
		if got := AlbumDate(Release{Date: in}); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
	if !SameArtist("Queen & David Bowie", "queen") || SameArtist("Various Artists", "Queen") || SameArtist("", "") {
		t.Error("SameArtist")
	}
}

func TestSearchReadsReleaseArtists(t *testing.T) {
	var limit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit = r.URL.Query().Get("limit")
		w.Write([]byte(`{"recordings":[{"id":"x","title":"Beautiful Life","score":100,"artist-credit":[{"name":"Ace of Base"}],
			"releases":[{"title":"Dance Machine 8","status":"Official","date":"1996-03","artist-credit":[{"name":"Various Artists"}],
			  "release-group":{"primary-type":"Album","secondary-types":["Compilation","DJ-mix"]}}]}]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, UserAgent: "t", Every: time.Millisecond}
	recs, err := c.SearchRecordings(context.Background(), "Ace of Base", "Beautiful Life")
	if err != nil || len(recs) != 1 || recs[0].Releases[0].Artist != "Various Artists" || limit != "100" {
		t.Fatalf("%+v %v limit %s", recs, err, limit)
	}
}
