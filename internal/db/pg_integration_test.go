package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/internal/tmdb"
)

// testDatabase creates a fresh, empty database for one test and drops it
// afterwards. It needs CINEVAULT_TEST_DATABASE_URL (run `make test-db`); the
// user in that URL must be allowed to create databases.
func testDatabase(t *testing.T) *DB {
	t.Helper()
	base := os.Getenv("CINEVAULT_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set CINEVAULT_TEST_DATABASE_URL to run database tests (make test-db)")
	}
	ctx := context.Background()

	admin, err := Open(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("cinevault_test_%d", time.Now().UnixNano())
	_, err = admin.Pool.Exec(ctx, "CREATE DATABASE "+name)
	admin.Close()
	if err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	d, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		d.Close()
		a, err := Open(context.Background(), base)
		if err != nil {
			t.Logf("could not drop %s: %v", name, err)
			return
		}
		defer a.Close()
		if _, err := a.Pool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("could not drop %s: %v", name, err)
		}
	})
	return d
}

func TestIntegrationMigrateConcurrentAndIdempotent(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	ms, err := LoadMigrations()
	if err != nil {
		t.Fatal(err)
	}

	// Two servers starting at the same moment must not collide.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = d.Migrate(ctx)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent migrate %d: %v", i, err)
		}
	}

	res, err := d.Migrate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 0 || res.Current != len(ms) {
		t.Errorf("third run should change nothing: %+v", res)
	}
	var recorded int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != len(ms) {
		t.Errorf("schema_migrations has %d rows, want %d", recorded, len(ms))
	}
}

func TestIntegrationEditionList(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	types, err := d.EditionTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 38 {
		t.Errorf("got %d edition types, want the 38 approved built-ins", len(types))
	}

	defaults := map[string][]string{}
	var tv []string
	for _, e := range types {
		if !e.Builtin || e.Hidden || len(e.ShortLabel) == 0 {
			t.Errorf("unexpected built-in entry: %+v", e)
		}
		for _, m := range e.MediaTypes {
			if e.IsDefault {
				defaults[m] = append(defaults[m], e.ID)
			}
			if m == "tv" {
				tv = append(tv, e.ID)
			}
		}
	}
	want := map[string]string{"movie": "theatrical", "tv": "broadcast", "music": "original", "audiobook": "unabridged", "book": "original"}
	for media, id := range want {
		if len(defaults[media]) != 1 || defaults[media][0] != id {
			t.Errorf("default for %s = %v, want [%s]", media, defaults[media], id)
		}
	}
	if len(tv) == 0 || tv[0] != "broadcast" {
		t.Errorf("TV editions should start with broadcast: %v", tv)
	}
}

func TestIntegrationAliensExample(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	var libID, titleID, theatrical, special int64
	q := func(dest *int64, sql string, args ...any) {
		t.Helper()
		if err := d.Pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	q(&libID, `INSERT INTO libraries (name, media_type, paths) VALUES ('Movies', 'movie', '{/media/movies}') RETURNING id`)
	q(&titleID, `INSERT INTO titles (library_id, media_type, name, file_title, sort_name, year, tmdb_id)
	             VALUES ($1, 'movie', 'Aliens', 'Aliens', 'aliens', 1986, 679) RETURNING id`, libID)
	q(&theatrical, `INSERT INTO editions (title_id, edition_type_id, runtime_seconds) VALUES ($1, 'theatrical', 8220) RETURNING id`, titleID)
	q(&special, `INSERT INTO editions (title_id, edition_type_id, runtime_seconds) VALUES ($1, 'special_edition', 9240) RETURNING id`, titleID)

	for _, v := range []struct {
		edition int64
		path    string
		label   string
	}{
		{theatrical, "/media/movies/Aliens (1986)/Aliens 2160p.mkv", "4K · HDR10 · Atmos"},
		{theatrical, "/media/movies/Aliens (1986)/Aliens 1080p.mkv", "1080p · DTS-HD MA"},
		{special, "/media/movies/Aliens (1986)/Aliens {edition-Special Edition} 1080p.mkv", "1080p · DTS-HD MA"},
	} {
		if _, err := d.Pool.Exec(ctx, `INSERT INTO versions (edition_id, path, size_bytes, label) VALUES ($1, $2, 1, $3)`,
			v.edition, v.path, v.label); err != nil {
			t.Fatal(err)
		}
	}

	var editions, versions int64
	q(&editions, `SELECT count(*) FROM editions WHERE title_id = $1`, titleID)
	q(&versions, `SELECT count(*) FROM versions v JOIN editions e ON e.id = v.edition_id WHERE e.title_id = $1`, titleID)
	if editions != 2 || versions != 3 {
		t.Errorf("editions=%d versions=%d, want 2 and 3", editions, versions)
	}

	// The same edition type can't be added twice to one title.
	if _, err := d.Pool.Exec(ctx, `INSERT INTO editions (title_id, edition_type_id) VALUES ($1, 'theatrical')`, titleID); err == nil {
		t.Error("duplicate edition was accepted")
	}

	// Deleting the title removes its editions and versions.
	if _, err := d.Pool.Exec(ctx, `DELETE FROM titles WHERE id = $1`, titleID); err != nil {
		t.Fatal(err)
	}
	q(&versions, `SELECT count(*) FROM versions`)
	if versions != 0 {
		t.Errorf("%d versions left after deleting the title", versions)
	}
}

type stubProber struct{}

func (stubProber) Probe(_ context.Context, path string) (mediainfo.Info, error) {
	return mediainfo.Info{Container: "mkv", VideoCodec: "hevc", Width: 1920, Height: 800,
		AudioCodec: "eac3", AudioChannels: 6, DurationSeconds: 7020.4, Bitrate: 2220000}, nil
}

func TestIntegrationScanAlien(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	remastered := filepath.Join(dir, "Alien - Remastered (1979) [Bluray-1080p x265].mkv")
	dc := filepath.Join(dir, "Alien {edition-Director's Cut} (1979) [1080p-MP4].mkv")
	for _, p := range []string{remastered, dc} {
		if err := os.WriteFile(p, []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Movies", MediaType: "movie", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateLibrary(ctx, catalog.Library{Name: "Movies", MediaType: "movie", Paths: []string{dir}}); !errors.Is(err, catalog.ErrConflict) {
		t.Errorf("duplicate library name: err = %v, want ErrConflict", err)
	}
	if _, err := d.Library(ctx, lib.ID+1000); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("unknown library: err = %v, want ErrNotFound", err)
	}

	s := scan.Scanner{Store: d, Prober: stubProber{}}
	res, err := s.Scan(ctx, lib)
	if err != nil || res.Added != 2 || len(res.Errors) != 0 {
		t.Fatalf("scan: %+v err=%v", res, err)
	}

	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(titles) != 1 || titles[0].Name != "Alien" || titles[0].Year != 1979 || len(titles[0].Editions) != 2 {
		t.Fatalf("titles = %+v", titles)
	}
	eds := titles[0].Editions
	if eds[0].EditionTypeID != "directors_cut" || eds[1].EditionTypeID != "remastered" {
		t.Errorf("edition order = %s, %s", eds[0].EditionTypeID, eds[1].EditionTypeID)
	}
	v := eds[1].Versions[0]
	if v.Path != remastered || v.Label != "1080p · HEVC · E-AC-3 5.1" || v.DurationSeconds != 7020.4 || eds[1].RuntimeSeconds != 7020 {
		t.Errorf("version = %+v runtime=%d", v, eds[1].RuntimeSeconds)
	}

	res, err = s.Scan(ctx, lib)
	if err != nil || res.Unchanged != 2 {
		t.Errorf("rescan: %+v err=%v", res, err)
	}

	if err := os.Remove(dc); err != nil {
		t.Fatal(err)
	}
	if res, err = s.Scan(ctx, lib); err != nil || res.Missing != 1 {
		t.Fatalf("after delete: %+v err=%v", res, err)
	}
	titles, _ = d.LibraryTitles(ctx, lib.ID)
	if !titles[0].Editions[0].Versions[0].Missing || titles[0].Editions[1].Versions[0].Missing {
		t.Errorf("missing flags wrong: %+v", titles[0].Editions)
	}
}

type stubTMDb struct{}

func (stubTMDb) SearchMovie(_ context.Context, q string, _ int) ([]tmdb.SearchResult, error) {
	if q == "Alien" {
		return []tmdb.SearchResult{{ID: 348, Title: "Alien", OriginalTitle: "Alien", ReleaseDate: "1979-05-25"}}, nil
	}
	return []tmdb.SearchResult{}, nil
}

func (stubTMDb) Movie(_ context.Context, id int) (tmdb.Movie, error) {
	m := tmdb.Movie{ID: id, IMDbID: "tt0078748", Title: "Alien", OriginalTitle: "Alien",
		Overview: "During its return to the earth...", ReleaseDate: "1979-05-25", Runtime: 117,
		PosterPath: "/poster.jpg", VoteAverage: 8.1}
	m.Genres = append(m.Genres, struct {
		Name string `json:"name"`
	}{Name: "Horror"})
	return m, nil
}

func TestIntegrationMatchAlien(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"Alien {edition-Director's Cut} (1979).mkv", "Nonexistent Film (2001).mkv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Movies", MediaType: "movie", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil {
		t.Fatal(err)
	}

	m := match.Matcher{Store: d, Source: stubTMDb{}}
	res, err := m.MatchLibrary(ctx, lib.ID)
	if err != nil || res.Matched != 1 || res.NotFound != 1 {
		t.Fatalf("match: %+v err=%v", res, err)
	}
	// A second run only retries errors, not titles TMDb didn't know.
	if res, err = m.MatchLibrary(ctx, lib.ID); err != nil || res.Matched+res.NotFound+res.Errors != 0 {
		t.Errorf("second run: %+v err=%v", res, err)
	}

	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	var alien catalog.Title
	for _, ti := range titles {
		if ti.TMDbID == 348 {
			alien = ti
		}
	}
	if alien.IMDbID != "tt0078748" || alien.RuntimeMinutes != 117 || alien.ReleaseDate != "1979-05-25" ||
		alien.PosterURL != tmdb.ImageBaseURL+"w500/poster.jpg" || len(alien.Genres) != 1 || alien.MatchStatus != "matched" {
		t.Errorf("alien = %+v", alien)
	}

	// Rescanning must not split the matched title.
	if res, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil || res.Unchanged != 2 {
		t.Errorf("rescan: %+v err=%v", res, err)
	}
	if titles, _ := d.LibraryTitles(ctx, lib.ID); len(titles) != 2 {
		t.Errorf("rescan created duplicate titles: %d", len(titles))
	}
}
