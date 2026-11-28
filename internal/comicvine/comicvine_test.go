package comicvine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearchVolumes(t *testing.T) {
	var key, resources string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, resources = r.URL.Query().Get("api_key"), r.URL.Query().Get("resources")
		if key == "bad" {
			w.Write([]byte(`{"error":"Invalid API Key","status_code":100,"results":[]}`))
			return
		}
		w.Write([]byte(`{"error":"OK","status_code":1,"results":[{"id":49901,"name":"Saga","start_year":"2012",
			"publisher":{"name":"Image"},"deck":"","description":"<p>An epic space opera &amp; fantasy.</p><h2>Collected Editions</h2><ul><li>Vol 1</li></ul>",
			"image":{"medium_url":"https://comicvine.gamespot.com/a/uploads/scale_medium/saga.jpg"},"count_of_issues":66}]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, APIKey: "k", UserAgent: "CineVault/test", Every: time.Millisecond}
	vols, err := c.SearchVolumes(context.Background(), "Saga")
	if err != nil || len(vols) != 1 {
		t.Fatalf("%+v %v", vols, err)
	}
	v := vols[0]
	if v.ID != 49901 || v.StartYear != 2012 || v.Publisher != "Image" || v.Description != "An epic space opera & fantasy." ||
		!strings.HasSuffix(v.ImageURL, "saga.jpg") || v.IssueCount != 66 || key != "k" || resources != "volume" {
		t.Errorf("%+v key %q", v, key)
	}
	c.APIKey = "bad"
	if _, err := c.SearchVolumes(context.Background(), "Saga"); !errors.Is(err, ErrBadKey) {
		t.Errorf("bad key: %v", err)
	}
}

func TestErrorsDontLeakTheKey(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:1", APIKey: "secret-key", UserAgent: "t", Every: time.Millisecond}
	_, err := c.SearchVolumes(context.Background(), "x")
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Errorf("error %v", err)
	}
}
