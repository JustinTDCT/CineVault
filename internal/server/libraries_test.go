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
	"time"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/scan"
)

type fakeLibraries struct {
	libs []catalog.Library
}

func (f *fakeLibraries) CreateLibrary(_ context.Context, l catalog.Library) (catalog.Library, error) {
	for _, x := range f.libs {
		if x.Name == l.Name {
			return l, catalog.ErrConflict
		}
	}
	l.ID = int64(len(f.libs) + 1)
	f.libs = append(f.libs, l)
	return l, nil
}
func (f *fakeLibraries) Libraries(context.Context) ([]catalog.Library, error) { return f.libs, nil }
func (f *fakeLibraries) Library(_ context.Context, id int64) (catalog.Library, error) {
	for _, l := range f.libs {
		if l.ID == id {
			return l, nil
		}
	}
	return catalog.Library{}, catalog.ErrNotFound
}
func (f *fakeLibraries) LibraryTitles(_ context.Context, id int64) ([]catalog.Title, error) {
	return []catalog.Title{{ID: 1, LibraryID: id, Name: "Alien", Year: 1979, Editions: []catalog.Edition{
		{EditionTypeID: "directors_cut"}, {EditionTypeID: "remastered"},
	}}}, nil
}

func libServer(scanFn func(context.Context, catalog.Library) (scan.Result, error)) (*Server, *fakeLibraries) {
	store := &fakeLibraries{}
	if scanFn == nil {
		scanFn = func(context.Context, catalog.Library) (scan.Result, error) {
			return scan.Result{FilesSeen: 2, Added: 2, Errors: []scan.FileError{}}, nil
		}
	}
	return New(Options{
		Libraries: store,
		Scan:      scanFn,
		IsDir:     func(p string) bool { return p == "/home/jdube/movies" },
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}), store
}

func send(t *testing.T, s *Server, method, target, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec.Code, rec.Body.String()
}

func TestCreateLibrary(t *testing.T) {
	s, store := libServer(nil)
	code, body := send(t, s, "POST", "/api/libraries", `{"name":" Movies ","media_type":"movie","paths":["/home/jdube/movies/"]}`)
	if code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", code, body)
	}
	if store.libs[0].Name != "Movies" || store.libs[0].Paths[0] != "/home/jdube/movies" {
		t.Errorf("stored %+v; name should be trimmed and path cleaned", store.libs[0])
	}

	bad := map[string]string{
		`{"name":"Movies","media_type":"movie","paths":["/home/jdube/movies"]}`:  "already exists",
		`{"name":"","media_type":"movie","paths":["/home/jdube/movies"]}`:        "name is required",
		`{"name":"X","media_type":"podcast","paths":["/home/jdube/movies"]}`:     "media_type",
		`{"name":"X","media_type":"movie","paths":[]}`:                           "at least one folder",
		`{"name":"X","media_type":"movie","paths":["movies"]}`:                   "absolute",
		`{"name":"X","media_type":"movie","paths":["/nope"]}`:                    "folder not found",
		`{"name":"X","media_type":"movie","paths":["/home/jdube/movies"],"x":1}`: "must be JSON",
		`not json`: "must be JSON",
	}
	for req, want := range bad {
		code, body := send(t, s, "POST", "/api/libraries", req)
		if code < 400 || !strings.Contains(body, want) {
			t.Errorf("%s: code=%d body=%s (want %q)", req, code, body, want)
		}
	}
	if len(store.libs) != 1 {
		t.Errorf("%d libraries stored, want 1", len(store.libs))
	}
}

func TestLibraryTitlesAndScan(t *testing.T) {
	s, _ := libServer(nil)
	send(t, s, "POST", "/api/libraries", `{"name":"Movies","media_type":"movie","paths":["/home/jdube/movies"]}`)

	code, body := send(t, s, "GET", "/api/libraries", "")
	if code != http.StatusOK || !strings.Contains(body, `"Movies"`) {
		t.Errorf("list: code=%d body=%s", code, body)
	}

	code, body = send(t, s, "POST", "/api/libraries/1/scan", "")
	var res scanResponse
	if err := json.Unmarshal([]byte(body), &res); err != nil || code != http.StatusOK || res.Added != 2 || res.Library.Name != "Movies" {
		t.Errorf("scan: code=%d body=%s", code, body)
	}

	code, body = send(t, s, "GET", "/api/libraries/1/titles", "")
	if code != http.StatusOK || !strings.Contains(body, `"directors_cut"`) {
		t.Errorf("titles: code=%d body=%s", code, body)
	}

	for target, want := range map[string]int{
		"/api/libraries/99/titles": http.StatusNotFound,
		"/api/libraries/x/titles":  http.StatusBadRequest,
		"/api/libraries/0/titles":  http.StatusBadRequest,
	} {
		if code, _ := send(t, s, "GET", target, ""); code != want {
			t.Errorf("%s: code=%d want %d", target, code, want)
		}
	}
}

func TestOnlyOneScanAtATime(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	s, _ := libServer(func(context.Context, catalog.Library) (scan.Result, error) {
		close(started)
		<-release
		return scan.Result{}, nil
	})
	send(t, s, "POST", "/api/libraries", `{"name":"Movies","media_type":"movie","paths":["/home/jdube/movies"]}`)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); send(t, s, "POST", "/api/libraries/1/scan", "") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first scan never started")
	}
	if code, body := send(t, s, "POST", "/api/libraries/1/scan", ""); code != http.StatusConflict {
		t.Errorf("second scan: code=%d body=%s", code, body)
	}
	close(release)
	wg.Wait()
}
