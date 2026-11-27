package match

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/tmdb"
)

func r(id int, title, date string) tmdb.SearchResult {
	return tmdb.SearchResult{ID: id, Title: title, OriginalTitle: title, ReleaseDate: date}
}

func TestPick(t *testing.T) {
	alienResults := []tmdb.SearchResult{
		r(126889, "Alien: Covenant", "2017-05-09"),
		r(348, "Alien", "1979-05-25"),
		r(9999, "Alien", "2019-01-01"),
	}
	cases := []struct {
		name    string
		title   Title
		results []tmdb.SearchResult
		want    int // 0 = no match
	}{
		{"exact title and year", Title{Name: "Alien", Year: 1979}, alienResults, 348},
		{"year off by one", Title{Name: "Alien", Year: 1980}, alienResults, 348},
		{"year too far", Title{Name: "Alien", Year: 1990}, alienResults, 0},
		{"no year: first acceptable in TMDb order", Title{Name: "Alien"}, alienResults, 348},
		{"punctuation", Title{Name: "Mission Impossible - Fallout", Year: 2018}, []tmdb.SearchResult{r(353081, "Mission: Impossible - Fallout", "2018-07-13")}, 353081},
		{"leading The and ampersand", Title{Name: "Fast and Furious", Year: 2009}, []tmdb.SearchResult{r(13804, "Fast & Furious", "2009-04-02")}, 13804},
		{"original title", Title{Name: "Amelie", Year: 2001}, []tmdb.SearchResult{{ID: 194, Title: "Amélie", OriginalTitle: "Amelie", ReleaseDate: "2001-04-25"}}, 194},
		{"partial title rejected", Title{Name: "Alien", Year: 2017}, []tmdb.SearchResult{r(126889, "Alien: Covenant", "2017-05-09")}, 0},
		{"exact year beats off-by-one", Title{Name: "Dune", Year: 2021}, []tmdb.SearchResult{r(1, "Dune", "2020-12-31"), r(438631, "Dune", "2021-09-15")}, 438631},
	}
	for _, c := range cases {
		got, ok := Pick(c.title, c.results)
		if (c.want == 0 && ok) || (c.want != 0 && (!ok || got.ID != c.want)) {
			t.Errorf("%s: got %d (ok=%v), want %d", c.name, got.ID, ok, c.want)
		}
	}
}

type fakeSource struct {
	searches []string
	err      error
}

func (f *fakeSource) SearchMovie(_ context.Context, q string, year int) ([]tmdb.SearchResult, error) {
	f.searches = append(f.searches, q)
	if f.err != nil {
		return nil, f.err
	}
	switch {
	case q == "Alien":
		return []tmdb.SearchResult{r(348, "Alien", "1979-05-25")}, nil
	case q == "Blade Runner" && year == 0:
		return []tmdb.SearchResult{r(78, "Blade Runner", "1982-06-25")}, nil
	}
	return []tmdb.SearchResult{}, nil
}

func (f *fakeSource) Movie(_ context.Context, id int) (tmdb.Movie, error) {
	if id == 404 {
		return tmdb.Movie{}, tmdb.ErrNotFound
	}
	return tmdb.Movie{ID: id, Title: "Movie " + string(rune('A'+id%26))}, nil
}

func (f *fakeSource) SearchTV(_ context.Context, q string, year int) ([]tmdb.SearchResult, error) {
	f.searches = append(f.searches, "tv:"+q)
	if q == "3 Body Problem" {
		return []tmdb.SearchResult{r(108545, "3 Body Problem", "2024-03-21")}, nil
	}
	return []tmdb.SearchResult{}, nil
}

func (f *fakeSource) TV(_ context.Context, id int) (tmdb.Show, error) {
	return tmdb.Show{ID: id, Name: "3 Body Problem", FirstAirDate: "2024-03-21",
		Seasons: []tmdb.SeasonSummary{{SeasonNumber: 0}, {SeasonNumber: 1}, {SeasonNumber: 2}}}, nil
}

func (f *fakeSource) Season(_ context.Context, show, n int) (tmdb.Season, error) {
	f.searches = append(f.searches, fmt.Sprintf("season:%d/%d", show, n))
	switch n {
	case 1:
		return tmdb.Season{SeasonNumber: 1, Episodes: make([]tmdb.Episode, 8)}, nil
	case 2:
		return tmdb.Season{}, errors.New("TMDb returned HTTP 500")
	}
	return tmdb.Season{}, tmdb.ErrNotFound
}

type fakeStore struct {
	titles   []Title
	matched  map[int64]int
	failures map[int64]string
	seasons  map[int64][]int
	pending  []ShowSeasons
}

func (f *fakeStore) SaveSeason(_ context.Context, id int64, s tmdb.Season) error {
	if f.seasons == nil {
		f.seasons = map[int64][]int{}
	}
	f.seasons[id] = append(f.seasons[id], s.SeasonNumber)
	return nil
}
func (f *fakeStore) ShowsNeedingEpisodes(context.Context, int64) ([]ShowSeasons, error) {
	return f.pending, nil
}

func (f *fakeStore) TitlesToMatch(context.Context, int64) ([]Title, error) { return f.titles, nil }
func (f *fakeStore) SaveMatch(_ context.Context, id int64, m tmdb.Movie) error {
	f.matched[id] = m.ID
	return nil
}
func (f *fakeStore) SaveMatchFailure(_ context.Context, id int64, status, _ string) error {
	f.failures[id] = status
	return nil
}

func TestMatchLibrary(t *testing.T) {
	store := &fakeStore{
		titles: []Title{
			{ID: 1, Name: "Alien", Year: 1979},
			{ID: 2, Name: "Blade Runner", Year: 1983}, // year search finds nothing; the retry without a year finds 1982, within one year
			{ID: 3, Name: "Not A Real Movie", Year: 2001},
			{ID: 4, Name: "Tagged", TMDbID: 603},
			{ID: 5, Name: "Bad Tag", TMDbID: 404},
		},
		matched: map[int64]int{}, failures: map[int64]string{},
	}
	src := &fakeSource{}
	res, err := Matcher{Store: store, Source: src}.MatchLibrary(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 3 || res.NotFound != 2 || res.Errors != 0 {
		t.Errorf("result = %+v", res)
	}
	if store.matched[1] != 348 || store.matched[2] != 78 || store.matched[4] != 603 {
		t.Errorf("matched = %v", store.matched)
	}
	if store.failures[3] != "not_found" || store.failures[5] != "not_found" {
		t.Errorf("failures = %v", store.failures)
	}
}

func TestMatchStopsOnBadToken(t *testing.T) {
	store := &fakeStore{titles: []Title{{ID: 1, Name: "Alien"}, {ID: 2, Name: "Aliens"}}, matched: map[int64]int{}, failures: map[int64]string{}}
	src := &fakeSource{err: tmdb.ErrUnauthorized}
	if _, err := (Matcher{Store: store, Source: src}).MatchLibrary(context.Background(), 1); !errors.Is(err, tmdb.ErrUnauthorized) {
		t.Errorf("err = %v", err)
	}
	if len(src.searches) != 1 || len(store.failures) != 0 {
		t.Errorf("should stop at the first title without recording failures: searches=%v failures=%v", src.searches, store.failures)
	}
}

func TestMatchRecordsNetworkErrors(t *testing.T) {
	store := &fakeStore{titles: []Title{{ID: 1, Name: "Alien"}}, matched: map[int64]int{}, failures: map[int64]string{}}
	src := &fakeSource{err: errors.New("TMDb request failed: timeout")}
	res, err := Matcher{Store: store, Source: src}.MatchLibrary(context.Background(), 1)
	if err != nil || res.Errors != 1 || store.failures[1] != "error" {
		t.Errorf("res=%+v err=%v failures=%v", res, err, store.failures)
	}
}

func TestMatchShows(t *testing.T) {
	store := &fakeStore{
		titles:  []Title{{ID: 7, MediaType: "tv", Name: "3 Body Problem"}, {ID: 8, MediaType: "tv", Name: "Unknown Show"}},
		matched: map[int64]int{}, failures: map[int64]string{},
		pending: []ShowSeasons{{TitleID: 9, Name: "Older Show", TMDbID: 555, Seasons: []int{1}}},
	}
	src := &fakeSource{}
	res, err := Matcher{Store: store, Source: src}.MatchLibrary(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if store.matched[7] != 108545 || store.failures[8] != "not_found" {
		t.Errorf("matched=%v failures=%v", store.matched, store.failures)
	}
	// Season 0 isn't on TMDb (skipped), season 1 saved, season 2 failed but didn't stop the run;
	// the older show's new season was fetched too.
	if fmt.Sprint(store.seasons[7]) != "[1]" || fmt.Sprint(store.seasons[9]) != "[1]" {
		t.Errorf("seasons = %v", store.seasons)
	}
	if res.Matched != 1 || res.NotFound != 1 || res.Errors != 1 || res.Episodes != 16 {
		t.Errorf("result = %+v", res)
	}
	if src.searches[0] != "tv:3 Body Problem" {
		t.Errorf("searches = %v", src.searches)
	}
}
