package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Trimmed copies of real TMDb responses.
const searchAlien = `{"page":1,"results":[
  {"id":348,"title":"Alien","original_title":"Alien","release_date":"1979-05-25","popularity":80.1},
  {"id":126889,"title":"Alien: Covenant","original_title":"Alien: Covenant","release_date":"2017-05-09","popularity":60.2}
],"total_results":2}`

const movieAlien = `{"id":348,"credits":{"cast":[
  {"id":10205,"name":"Sigourney Weaver","character":"Ripley","profile_path":"/w.jpg","order":1},
  {"id":4139,"name":"Tom Skerritt","character":"Dallas","profile_path":null,"order":0}],
  "crew":[{"name":"Ridley Scott","job":"Director"},{"name":"Ridley Scott","job":"Director"},{"name":"Dan O'Bannon","job":"Screenplay"}]},"imdb_id":"tt0078748","title":"Alien","original_title":"Alien",
 "overview":"During its return to the earth, commercial spaceship Nostromo intercepts a distress signal...",
 "tagline":"In space no one can hear you scream.","release_date":"1979-05-25","runtime":117,
 "poster_path":"/vfrQk5IPloGg1v9Rzbh2Eg3VGyM.jpg","backdrop_path":"/AmR3JG1VQVxU8TfAvljUhfSFUOx.jpg",
 "vote_average":8.1,"genres":[{"id":27,"name":"Horror"},{"id":878,"name":"Science Fiction"}],
 "belongs_to_collection":{"id":8091,"name":"Alien Collection"}}`

func fake(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("test-token")
	c.BaseURL = srv.URL + "/3"
	c.MinInterval = 0
	return c
}

func TestSearchAndDetails(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/3/search/movie":
			q := r.URL.Query()
			if q.Get("query") != "Alien" || q.Get("primary_release_year") != "1979" || q.Get("language") != "en-US" {
				t.Errorf("query = %v", q)
			}
			w.Write([]byte(searchAlien))
		case "/3/movie/348":
			if r.URL.Query().Get("append_to_response") != "credits" {
				t.Errorf("details query = %s", r.URL.RawQuery)
			}
			w.Write([]byte(movieAlien))
		default:
			http.NotFound(w, r)
		}
	})

	res, err := c.SearchMovie(context.Background(), "Alien", 1979)
	if err != nil || len(res) != 2 || res[0].ID != 348 || res[0].Year() != 1979 {
		t.Fatalf("search: %+v err=%v", res, err)
	}
	m, err := c.Movie(context.Background(), 348)
	if err != nil {
		t.Fatal(err)
	}
	if m.IMDbID != "tt0078748" || m.Runtime != 117 || m.Year() != 1979 || m.Collection == nil || m.Collection.Name != "Alien Collection" {
		t.Errorf("movie = %+v", m)
	}
	if g := m.GenreNames(); len(g) != 2 || g[1] != "Science Fiction" {
		t.Errorf("genres = %v", g)
	}
	if c := m.Credits.TopCast(); len(c) != 2 || c[0].Name != "Tom Skerritt" || c[1].Character != "Ripley" || c[1].ProfilePath != "/w.jpg" {
		t.Errorf("cast = %+v", c)
	}
	if d := m.Credits.Directors(); len(d) != 1 || d[0] != "Ridley Scott" {
		t.Errorf("directors = %v", d)
	}
}

func TestErrors(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/movie/1":
			w.WriteHeader(http.StatusUnauthorized)
		case "/3/movie/2":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusBadGateway)
		}
	})
	if _, err := c.Movie(context.Background(), 1); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("401: %v", err)
	}
	if _, err := c.Movie(context.Background(), 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	if _, err := c.Movie(context.Background(), 3); err == nil {
		t.Error("502 should be an error")
	}
}

func TestRetriesAfterRateLimit(t *testing.T) {
	var calls atomic.Int32
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(movieAlien))
	})
	if m, err := c.Movie(context.Background(), 348); err != nil || m.ID != 348 || calls.Load() != 2 {
		t.Errorf("m=%d err=%v calls=%d", m.ID, err, calls.Load())
	}
}

func TestEmptySearch(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("primary_release_year") {
			t.Error("year 0 should not be sent")
		}
		w.Write([]byte(`{"page":1,"results":[]}`))
	})
	res, err := c.SearchMovie(context.Background(), "Nothing", 0)
	if err != nil || res == nil || len(res) != 0 {
		t.Errorf("res=%v err=%v", res, err)
	}
}
