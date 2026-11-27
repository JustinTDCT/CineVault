package db

import (
	"archive/zip"
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
	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
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
	// 38 approved in 0001, plus 5 music video editions in 0004.
	if len(types) != 43 {
		t.Errorf("got %d edition types, want the 43 built-ins", len(types))
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
	want := map[string]string{"movie": "theatrical", "tv": "broadcast", "music": "original", "audiobook": "unabridged", "book": "original",
		"musicvideo": "official_video", "comic": "original"}
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
	m.Credits = tmdb.Credits{
		Cast: []tmdb.CastMember{{Name: "Sigourney Weaver", Character: "Ripley", ProfilePath: "/sw.jpg"}, {Name: "Tom Skerritt", Character: "Dallas", Order: 1}},
		Crew: []tmdb.CrewMember{{Name: "Ridley Scott", Job: "Director"}},
	}
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

	if len(alien.Cast) != 2 || alien.Cast[0].Name != "Sigourney Weaver" || alien.Cast[0].Character != "Ripley" ||
		alien.Cast[0].ProfileURL != tmdb.ImageBaseURL+"w185/sw.jpg" || alien.Cast[1].ProfileURL != "" ||
		len(alien.Directors) != 1 || alien.Directors[0] != "Ridley Scott" {
		t.Errorf("cast = %+v, directors = %v", alien.Cast, alien.Directors)
	}
	if alien.AddedAt.IsZero() || alien.Progress != nil {
		t.Errorf("added = %v, progress = %+v", alien.AddedAt, alien.Progress)
	}

	// A title matched before cast was stored gets it on the next match.
	if _, err := d.Pool.Exec(ctx, `UPDATE titles SET cast_members = NULL, directors = NULL WHERE id = $1`, alien.ID); err != nil {
		t.Fatal(err)
	}
	if res, err := m.MatchLibrary(ctx, lib.ID); err != nil || res.Credits != 1 {
		t.Errorf("credits run: %+v err=%v", res, err)
	}
	if res, err := m.MatchLibrary(ctx, lib.ID); err != nil || res.Credits != 0 {
		t.Errorf("credits again: %+v err=%v", res, err)
	}

	// Progress shows on the title with the file's length.
	v := alien.Editions[0].Versions[0]
	if _, err := d.Pool.Exec(ctx, `UPDATE versions SET duration_seconds = 7020 WHERE id = $1`, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveProgress(ctx, alien.ID, catalog.Progress{VersionID: v.ID, PositionSeconds: 2460}); err != nil {
		t.Fatal(err)
	}
	titles, _ = d.LibraryTitles(ctx, lib.ID)
	for _, ti := range titles {
		if ti.ID == alien.ID {
			if p := ti.Progress; p == nil || p.VersionID != v.ID || p.PositionSeconds != 2460 || p.DurationSeconds != 7020 || p.UpdatedAt.IsZero() {
				t.Errorf("progress = %+v", p)
			}
			if len(ti.Cast) != 2 {
				t.Errorf("cast after refetch = %+v", ti.Cast)
			}
		}
	}

	// Rescanning must not split the matched title.
	if res, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil || res.Unchanged != 2 {
		t.Errorf("rescan: %+v err=%v", res, err)
	}
	if titles, _ := d.LibraryTitles(ctx, lib.ID); len(titles) != 2 {
		t.Errorf("rescan created duplicate titles: %d", len(titles))
	}
}

func (stubTMDb) SearchTV(_ context.Context, q string, _ int) ([]tmdb.SearchResult, error) {
	if q == "3 Body Problem" {
		return []tmdb.SearchResult{{ID: 108545, Title: "3 Body Problem", OriginalTitle: "3 Body Problem", ReleaseDate: "2024-03-21"}}, nil
	}
	return []tmdb.SearchResult{}, nil
}

func (stubTMDb) TV(_ context.Context, id int) (tmdb.Show, error) {
	return tmdb.Show{ID: id, Name: "3 Body Problem", FirstAirDate: "2024-03-21", PosterPath: "/tbp.jpg",
		Seasons: []tmdb.SeasonSummary{{SeasonNumber: 1, EpisodeCount: 8}}}, nil
}

func (stubTMDb) Season(_ context.Context, _ int, n int) (tmdb.Season, error) {
	if n != 1 {
		return tmdb.Season{}, tmdb.ErrNotFound
	}
	s := tmdb.Season{SeasonNumber: 1, Name: "Season 1", AirDate: "2024-03-21", PosterPath: "/s1.jpg"}
	for i := 1; i <= 8; i++ {
		s.Episodes = append(s.Episodes, tmdb.Episode{ID: 1000 + i, SeasonNumber: 1, EpisodeNumber: i,
			Name: fmt.Sprintf("Episode %d", i), Runtime: 55, StillPath: "/still.jpg", AirDate: "2024-03-21"})
	}
	return s, nil
}

func TestIntegrationTVShow(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	season := filepath.Join(dir, "3 Body Problem", "Season 1")
	if err := os.MkdirAll(season, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"3 Body Problem - S01E01 - Countdown [WEBRip-1080p].mkv",
		"3 Body Problem - S01E02-E03 - Red Coast [WEBRip-1080p].mkv",
		"3 Body Problem - S01E05 - Judgment Day [WEBRip-1080p].mkv",
	} {
		if err := os.WriteFile(filepath.Join(season, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "TV", MediaType: "tv", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	scanner := scan.Scanner{Store: d, Prober: stubProber{}}
	if res, err := scanner.Scan(ctx, lib); err != nil || res.Added != 3 {
		t.Fatalf("scan: %+v err=%v", res, err)
	}
	m := match.Matcher{Store: d, Source: stubTMDb{}}
	if res, err := m.MatchLibrary(ctx, lib.ID); err != nil || res.Matched != 1 || res.Episodes != 8 {
		t.Fatalf("match: %+v err=%v", res, err)
	}

	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil || len(titles) != 1 {
		t.Fatalf("titles: %v %v", titles, err)
	}
	show := titles[0]
	if show.Year != 2024 || show.SeasonCount != 1 || show.EpisodeCount != 3 || len(show.Editions) != 1 || len(show.Editions[0].Versions) != 0 {
		t.Errorf("show = %+v", show)
	}

	seasons, err := d.TitleEpisodes(ctx, show.ID)
	if err != nil || len(seasons) != 1 || len(seasons[0].Episodes) != 8 {
		t.Fatalf("episodes: %+v err=%v", seasons, err)
	}
	eps := seasons[0].Episodes
	if eps[0].Name != "Episode 1" || len(eps[0].Versions) != 1 || eps[0].StillURL == "" || eps[0].RuntimeMinutes != 55 {
		t.Errorf("episode 1 = %+v", eps[0])
	}
	if len(eps[1].Versions) != 1 || eps[1].Versions[0].LastEpisode != 3 || eps[2].PartOf != 2 {
		t.Errorf("multi-episode file: e2=%+v e3=%+v", eps[1], eps[2])
	}
	if len(eps[3].Versions) != 0 || eps[3].PartOf != 0 {
		t.Errorf("episode 4 should be listed without a file: %+v", eps[3])
	}
	if _, err := d.TitleEpisodes(ctx, 999999); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("missing title: %v", err)
	}

	// TMDb filled in the year; a rescan must still find the show by its folder name.
	if err := os.WriteFile(filepath.Join(dir, "3 Body Problem", "3 Body Problem - S02E01 - New.mkv"), []byte("s2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if res, err := scanner.Scan(ctx, lib); err != nil || res.Added != 1 || res.Unchanged != 3 {
		t.Fatalf("rescan: %+v err=%v", res, err)
	}
	if titles, _ := d.LibraryTitles(ctx, lib.ID); len(titles) != 1 {
		t.Errorf("rescan split the show: %d titles", len(titles))
	}
	need, err := d.ShowsNeedingEpisodes(ctx, lib.ID)
	if err != nil || len(need) != 1 || need[0].TMDbID != 108545 || fmt.Sprint(need[0].Seasons) != "[2]" {
		t.Errorf("shows needing episodes = %+v err=%v", need, err)
	}
}

func TestIntegrationMusic(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	album := filepath.Join(dir, "3 Doors Down", "Away From the Sun (2002)")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"3 Doors Down - 01 - When I'm Gone [CD].flac",
		"3 Doors Down - 02 - Away From the Sun [CD].flac",
		"3 Doors Down - 02 - Away From the Sun [CD].mp3", // a second file of the same song
	} {
		if err := os.WriteFile(filepath.Join(album, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Music", MediaType: "music", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil || res.Added != 3 {
		t.Fatalf("scan: %+v err=%v", res, err)
	}
	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil || len(titles) != 1 {
		t.Fatalf("titles: %v %v", titles, err)
	}
	a := titles[0]
	if a.Name != "Away From the Sun" || a.Artist != "3 Doors Down" || a.Year != 2002 || a.TrackCount != 2 || a.CoverVersionID == 0 || len(a.Editions[0].Versions) != 0 {
		t.Errorf("album = %+v", a)
	}
	eds, err := d.AlbumTracks(ctx, a.ID)
	if err != nil || len(eds) != 1 || len(eds[0].Tracks) != 2 {
		t.Fatalf("tracks: %+v err=%v", eds, err)
	}
	if tr := eds[0].Tracks[1]; tr.Number != 2 || tr.Name != "Away From the Sun" || len(tr.Versions) != 2 {
		t.Errorf("track 2 = %+v", tr)
	}
	if _, err := d.AlbumTracks(ctx, 999999); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("missing album: %v", err)
	}
}

func TestIntegrationAudiobook(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	series := filepath.Join(dir, "The John Matherson Series")
	if err := os.MkdirAll(series, 0o755); err != nil {
		t.Fatal(err)
	}
	book := filepath.Join(series, "One Second After.m4b")
	if err := os.WriteFile(book, []byte("m4b"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Audiobooks", MediaType: "audiobook", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil || res.Added != 1 {
		t.Fatalf("scan: %+v err=%v", res, err)
	}
	if err := d.SaveChapters(ctx, book, []mediainfo.Chapter{{Start: 0, End: 30, Name: "Opening Credits"}, {Start: 30, End: 7020.4, Name: "Chapter 1"}}); err != nil {
		t.Fatal(err)
	}
	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil || len(titles) != 1 {
		t.Fatalf("titles: %v %v", titles, err)
	}
	b := titles[0]
	if b.Name != "One Second After" || b.Series != "The John Matherson Series" || b.CoverVersionID == 0 {
		t.Errorf("book = %+v", b)
	}
	eds, err := d.AlbumTracks(ctx, b.ID)
	if err != nil || len(eds) != 1 || len(eds[0].Tracks) != 1 {
		t.Fatalf("parts: %+v err=%v", eds, err)
	}
	v := eds[0].Tracks[0].Versions[0]
	if len(v.Chapters) != 2 || v.Chapters[1].Name != "Chapter 1" || v.Chapters[1].Start != 30 {
		t.Errorf("chapters = %+v", v.Chapters)
	}

	if _, ok, err := d.Progress(ctx, b.ID); err != nil || ok {
		t.Errorf("progress before playing: ok=%v err=%v", ok, err)
	}
	if _, err := d.SaveProgress(ctx, b.ID, catalog.Progress{VersionID: v.ID, PositionSeconds: 812.4}); err != nil {
		t.Fatal(err)
	}
	if p, ok, err := d.Progress(ctx, b.ID); err != nil || !ok || p.PositionSeconds != 812.4 || p.VersionID != v.ID {
		t.Errorf("progress = %+v ok=%v err=%v", p, ok, err)
	}
	if _, err := d.SaveProgress(ctx, b.ID, catalog.Progress{VersionID: v.ID + 1000, PositionSeconds: 1}); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("someone else's file: %v", err)
	}
}

func TestIntegrationEbooks(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "J.R.R. Tolkien")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"J R R Tolkien - The Story of Kullervo (epub).epub", "J R R Tolkien - The Story of Kullervo (azw3).azw3"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Ebooks", MediaType: "book", Paths: []string{filepath.Dir(dir)}})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil || res.Added != 2 {
		t.Fatalf("scan: %+v err=%v", res, err)
	}
	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil || len(titles) != 1 {
		t.Fatalf("titles: %v %v", titles, err)
	}
	b := titles[0]
	if b.Name != "The Story of Kullervo" || b.Artist != "J.R.R. Tolkien" || len(b.Editions[0].Versions) != 2 || b.CoverVersionID == 0 {
		t.Errorf("book = %+v", b)
	}
	v := b.Editions[0].Versions[0]
	if v.Label != "EPUB" && v.Label != "AZW3" {
		t.Errorf("label = %q", v.Label)
	}
	if _, err := d.SaveProgress(ctx, b.ID, catalog.Progress{VersionID: v.ID, PositionSeconds: 0.42, Locator: "epubcfi(/6/14!/4/2/1:0)"}); err != nil {
		t.Fatal(err)
	}
	if p, ok, err := d.Progress(ctx, b.ID); err != nil || !ok || p.Locator != "epubcfi(/6/14!/4/2/1:0)" || p.PositionSeconds != 0.42 {
		t.Errorf("progress = %+v %v %v", p, ok, err)
	}
}

func TestIntegrationComics(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "Aliens - Book One")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Aliens - Book One - 1.cbz", "Aliens - Book One - 2.cbz"} {
		f, err := os.Create(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(f)
		for _, pg := range []string{"01.jpg", "02.jpg"} {
			w, _ := z.Create(pg)
			w.Write([]byte{0xFF, 0xD8, 0xFF})
		}
		z.Close()
		f.Close()
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Comics", MediaType: "comic", Paths: []string{filepath.Dir(dir)}})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil || res.Added != 2 {
		t.Fatalf("scan: %+v err=%v", res, err)
	}
	titles, err := d.LibraryTitles(ctx, lib.ID)
	if err != nil || len(titles) != 1 || titles[0].TrackCount != 2 || titles[0].CoverVersionID == 0 {
		t.Fatalf("titles: %+v %v", titles, err)
	}
	eds, err := d.AlbumTracks(ctx, titles[0].ID)
	if err != nil || len(eds) != 1 || len(eds[0].Tracks) != 2 {
		t.Fatalf("issues: %+v %v", eds, err)
	}
	if iss := eds[0].Tracks[1]; iss.Name != "Issue 2" || iss.Versions[0].PageCount != 2 {
		t.Errorf("issue 2 = %+v", iss)
	}
	v, err := d.Version(ctx, eds[0].Tracks[0].Versions[0].ID)
	if err != nil || v.PageCount != 2 {
		t.Errorf("version = %+v %v", v, err)
	}
}

func TestIntegrationCacheRecords(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Alien {edition-Director's Cut} (1979).mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := d.CreateLibrary(ctx, catalog.Library{Name: "Movies", MediaType: "movie", Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (scan.Scanner{Store: d, Prober: stubProber{}}).Scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	if _, err := (match.Matcher{Store: d, Source: stubTMDb{}}).MatchLibrary(ctx, lib.ID); err != nil {
		t.Fatal(err)
	}

	items, err := d.CacheItems(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %+v, %v", items, err)
	}
	it := items[0]
	if it.Key != (cacheapi.Key{Kind: "movie", TMDbID: 348}) || it.State != "none" ||
		len(it.Owned) != 1 || it.Owned[0] != (cacheapi.ContributeEdition{Edition: "directors_cut", RuntimeMin: 117}) {
		t.Fatalf("alien item = %+v", it)
	}
	if k, ok, err := d.TitleCacheKey(ctx, it.TitleID); !ok || err != nil || k != it.Key {
		t.Errorf("title cache key = %v %v %v", k, ok, err)
	}

	rec := cacheapi.Record{
		Key: it.Key, Status: cacheapi.StatusReady, RecordVersion: 3,
		Editions: []cacheapi.Edition{
			{Edition: "theatrical", Status: cacheapi.EditionPublished, RuntimeMin: 117, RuntimeSources: []string{"s1"}, Confidence: "high"},
			{Edition: "directors_cut", Status: cacheapi.EditionPublished, RuntimeMin: 116, DifferenceShort: "Adds the cocoon scene.",
				DifferenceFull: []cacheapi.Difference{{Change: "added", Text: "The cocoon scene.", Sources: []string{"s1"}}}, Confidence: "high"},
			{Edition: "edition_from_the_future", Status: cacheapi.EditionPublished, RuntimeMin: 120},
		},
		Sources: []cacheapi.Source{{ID: "s1", Name: "TMDb", URL: "https://www.themoviedb.org/movie/348"}},
	}
	if err := d.ApplyRecord(ctx, it.TitleID, rec); err != nil {
		t.Fatal(err)
	}
	alien := func() catalog.Title {
		titles, err := d.LibraryTitles(ctx, lib.ID)
		if err != nil || len(titles) != 1 {
			t.Fatalf("titles = %d, %v", len(titles), err)
		}
		return titles[0]
	}
	got := alien()
	if got.CacheState != "cached" || len(got.CacheSources) == 0 || len(got.Editions) != 2 {
		t.Fatalf("alien after apply = %+v", got)
	}
	theatrical, dc := got.Editions[0], got.Editions[1]
	if theatrical.EditionTypeID != "theatrical" || !theatrical.FromCache || len(theatrical.Versions) != 0 ||
		theatrical.Cache == nil || theatrical.Cache.RuntimeMin != 117 {
		t.Errorf("placeholder = %+v cache %+v", theatrical, theatrical.Cache)
	}
	if dc.FromCache || len(dc.Versions) != 1 || dc.Cache == nil || dc.Cache.DifferenceShort != "Adds the cocoon scene." || len(dc.Cache.DifferenceFull) == 0 {
		t.Errorf("owned edition = %+v cache %+v", dc, dc.Cache)
	}
	// Placeholders aren't shared as owned.
	if items, _ := d.CacheItems(ctx); len(items[0].Owned) != 1 || items[0].Version != 3 || items[0].State != "cached" {
		t.Errorf("items after apply = %+v", items[0])
	}

	// A withdrawn edition stays, marked "No longer verified".
	rec.RecordVersion = 4
	rec.Editions = []cacheapi.Edition{{Edition: "directors_cut", Status: cacheapi.EditionWithdrawn}}
	if err := d.ApplyRecord(ctx, it.TitleID, rec); err != nil {
		t.Fatal(err)
	}
	got = alien()
	if len(got.Editions) != 2 || got.Editions[1].Cache.Status != "withdrawn" || got.Editions[1].Cache.RuntimeMin != 116 {
		t.Errorf("after withdrawal = %+v", got.Editions)
	}

	if err := d.SetCacheState(ctx, it.TitleID, "error", time.Now().Add(time.Hour), "boom"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetContributed(ctx, map[int64]string{it.TitleID: "abc"}); err != nil {
		t.Fatal(err)
	}
	if items, _ := d.CacheItems(ctx); items[0].State != "error" || items[0].ContributedHash != "abc" || !items[0].RetryAt.After(time.Now()) {
		t.Errorf("items = %+v", items[0])
	}

	var share bool
	if ok, err := d.Setting(ctx, "cache.share", &share); ok || err != nil {
		t.Errorf("unset setting: ok %v err %v", ok, err)
	}
	if err := d.SaveSetting(ctx, "cache.share", false); err != nil {
		t.Fatal(err)
	}
	share = true
	if ok, err := d.Setting(ctx, "cache.share", &share); !ok || err != nil || share {
		t.Errorf("saved setting: %v %v %v", ok, err, share)
	}
}
