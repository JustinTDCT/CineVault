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
	"sync"
	"testing"
	"time"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/match"
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

func (f *fakeLibraries) TitleEpisodes(_ context.Context, id int64) ([]catalog.Season, error) {
	if id != 7 {
		return nil, catalog.ErrNotFound
	}
	return []catalog.Season{{Number: 1, Name: "Season 1", Episodes: []catalog.Episode{
		{ID: 70, Season: 1, Number: 1, Name: "Countdown", Versions: []catalog.Version{{ID: 700}}},
		{ID: 71, Season: 1, Number: 2, Name: "Red Coast", Versions: []catalog.Version{}},
	}}}, nil
}

func (f *fakeLibraries) AlbumTracks(_ context.Context, id int64) ([]catalog.AlbumEdition, error) {
	if id != 7 {
		return nil, catalog.ErrNotFound
	}
	return []catalog.AlbumEdition{{EditionID: 1, Tracks: []catalog.Track{{ID: 5, Disc: 1, Number: 1, Name: "When I'm Gone", Versions: []catalog.Version{{ID: 50}}}}}}, nil
}

func (f *fakeLibraries) Progress(_ context.Context, id int64) (catalog.Progress, bool, error) {
	if id == 7 {
		return catalog.Progress{VersionID: 50, PositionSeconds: 812.5}, true, nil
	}
	return catalog.Progress{}, false, nil
}

func (f *fakeLibraries) SaveProgress(_ context.Context, id int64, p catalog.Progress) (catalog.Progress, error) {
	if id != 7 || p.VersionID != 50 {
		return p, catalog.ErrNotFound
	}
	return p, nil
}

func TestProgress(t *testing.T) {
	s, _ := libServer(nil)
	if code, body := send(t, s, "GET", "/api/titles/7/progress", ""); code != 200 || !strings.Contains(body, `"position_seconds": 812.5`) {
		t.Errorf("get: %d %s", code, body)
	}
	if code, body := send(t, s, "GET", "/api/titles/8/progress", ""); code != 200 || !strings.Contains(body, `"version_id": 0`) {
		t.Errorf("never played: %d %s", code, body)
	}
	for body, want := range map[string]int{
		`{"version_id":50,"position_seconds":100}`: 200,
		`{"version_id":51,"position_seconds":100}`: 404,
		`{"version_id":50,"position_seconds":-1}`:  400,
		`nope`: 400,
	} {
		if code, _ := send(t, s, "PUT", "/api/titles/7/progress", body); code != want {
			t.Errorf("put %s: %d, want %d", body, code, want)
		}
	}
}

func TestAlbumTracks(t *testing.T) {
	s, _ := libServer(nil)
	code, body := send(t, s, "GET", "/api/titles/7/tracks", "")
	if code != http.StatusOK || !strings.Contains(body, `"When I'm Gone"`) || !strings.Contains(body, `"edition_id": 1`) {
		t.Fatalf("code=%d body=%s", code, body)
	}
	if code, _ := send(t, s, "GET", "/api/titles/8/tracks", ""); code != http.StatusNotFound {
		t.Errorf("missing: %d", code)
	}
}

func TestTitleEpisodes(t *testing.T) {
	s, _ := libServer(nil)
	code, body := send(t, s, "GET", "/api/titles/7/episodes", "")
	var seasons []catalog.Season
	if err := json.Unmarshal([]byte(body), &seasons); code != http.StatusOK || err != nil || len(seasons) != 1 || seasons[0].Episodes[0].Name != "Countdown" {
		t.Fatalf("code=%d body=%s", code, body)
	}
	if !strings.Contains(body, `"versions": []`) {
		t.Errorf("an episode without files should list no versions as [], got %s", body)
	}
	for target, want := range map[string]int{"/api/titles/8/episodes": 404, "/api/titles/x/episodes": 400} {
		if code, _ := send(t, s, "GET", target, ""); code != want {
			t.Errorf("%s: %d, want %d", target, code, want)
		}
	}
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

func TestScanRunsMatchWhenConfigured(t *testing.T) {
	s, _ := libServer(nil)
	s.opt.Match = func(context.Context, int64) (match.Result, error) {
		return match.Result{Matched: 1, Messages: []string{}}, nil
	}
	send(t, s, "POST", "/api/libraries", `{"name":"Movies","media_type":"movie","paths":["/home/jdube/movies"]}`)

	code, body := send(t, s, "POST", "/api/libraries/1/scan", "")
	var res scanResponse
	if err := json.Unmarshal([]byte(body), &res); err != nil || code != http.StatusOK || res.Match == nil || res.Match.Matched != 1 {
		t.Errorf("scan: code=%d body=%s", code, body)
	}
	code, body = send(t, s, "POST", "/api/libraries/1/match", "")
	if code != http.StatusOK || !strings.Contains(body, `"matched": 1`) {
		t.Errorf("match: code=%d body=%s", code, body)
	}
}

func TestMatchErrorsAreReportedNotFatal(t *testing.T) {
	s, _ := libServer(nil)
	s.opt.Match = func(context.Context, int64) (match.Result, error) {
		return match.Result{}, errors.New("TMDb rejected the API token")
	}
	send(t, s, "POST", "/api/libraries", `{"name":"Movies","media_type":"movie","paths":["/home/jdube/movies"]}`)
	code, body := send(t, s, "POST", "/api/libraries/1/scan", "")
	if code != http.StatusOK || !strings.Contains(body, "rejected the API token") || !strings.Contains(body, `"added": 2`) {
		t.Errorf("scan should succeed and report the match error: code=%d body=%s", code, body)
	}
}

func TestMatchWithoutTMDb(t *testing.T) {
	s, _ := libServer(nil)
	send(t, s, "POST", "/api/libraries", `{"name":"Movies","media_type":"movie","paths":["/home/jdube/movies"]}`)
	if code, body := send(t, s, "POST", "/api/libraries/1/match", ""); code != http.StatusServiceUnavailable || !strings.Contains(body, "CINEVAULT_TMDB_TOKEN") {
		t.Errorf("code=%d body=%s", code, body)
	}
	code, body := send(t, s, "POST", "/api/libraries/1/scan", "")
	if code != http.StatusOK || !strings.Contains(body, `"match": null`) {
		t.Errorf("scan without TMDb: code=%d body=%s", code, body)
	}
}
