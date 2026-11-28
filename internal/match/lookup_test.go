package match

import (
	"context"
	"errors"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/comicvine"
	"github.com/JustinTDCT/CineVault/internal/musicbrainz"
	"github.com/JustinTDCT/CineVault/internal/openlibrary"
)

func TestSameArtist(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"J.R.R. Tolkien", "J. R. R. Tolkien", true},
		{"Tolkien", "J.R.R. Tolkien", true},
		{"Queen & David Bowie", "Queen", true},
		{"Ace of Base", "ACE OF BASE", true},
		{"Herbert, Frank", "Frank Herbert", true},
		{"Stephen King", "Stephen Fry", false},
		{"Stephen King", "Carole King", false},
		{"", "", false},
	} {
		if got := sameArtist(c.a, c.b); got != c.want {
			t.Errorf("sameArtist(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestPickReleaseGroup(t *testing.T) {
	rg := func(id, title, artist, primary, date string, score int, secondary ...string) musicbrainz.ReleaseGroup {
		return musicbrainz.ReleaseGroup{ID: id, Title: title, Artist: artist, PrimaryType: primary, SecondaryTypes: secondary, FirstReleaseDate: date, Score: score}
	}
	groups := []musicbrainz.ReleaseGroup{
		rg("single", "The Bridge", "Ace of Base", "Single", "1995-10-01", 100),
		rg("live", "The Bridge", "Ace of Base", "Album", "1996-05-01", 100, "Live"),
		rg("other", "The Bridge", "Billy Joel", "Album", "1986-07-29", 100),
		rg("album", "The Bridge", "Ace of Base", "Album", "1995-11-13", 98),
		rg("weak", "The Bridge", "Ace of Base", "Album", "1990", 40),
	}
	got, ok := PickReleaseGroup(LookupItem{Name: "The Bridge", Artist: "Ace of Base"}, groups)
	if !ok || got.ID != "album" {
		t.Fatalf("picked %+v", got)
	}
	// The files' year wins: a 1996 live tag means the live album.
	if got, _ := PickReleaseGroup(LookupItem{Name: "The Bridge", Artist: "Ace of Base", Year: 1996}, groups); got.ID != "live" {
		t.Errorf("with year: %s", got.ID)
	}
	if _, ok := PickReleaseGroup(LookupItem{Name: "Flowers", Artist: "Ace of Base"}, groups); ok {
		t.Error("matched another album")
	}
}

func TestPickWork(t *testing.T) {
	works := []openlibrary.Work{
		{ID: "OL1W", Title: "Dune", Authors: []string{"SparkNotes"}, EditionCount: 3},
		{ID: "OL2W", Title: "Dune: A Study Guide", Authors: []string{"Frank Herbert"}, EditionCount: 2},
		{ID: "OL893415W", Title: "Dune", Authors: []string{"Frank Herbert"}, EditionCount: 185, FirstPublishYear: 1965},
		{ID: "OL4W", Title: "Dune Messiah", Authors: []string{"Frank Herbert"}, EditionCount: 90},
	}
	got, ok := PickWork(LookupItem{Name: "Dune (Deluxe Edition)", Artist: "Herbert, Frank"}, works)
	if !ok || got.ID != "OL893415W" {
		t.Fatalf("picked %+v %v", got, ok)
	}
	// Accents and "Last, First" authors from the VM test (28 Nov 2026).
	hurin := []openlibrary.Work{{ID: "OL27513W", Title: "The Children of Húrin", Authors: []string{"J.R.R. Tolkien"}, EditionCount: 60}}
	if got, ok := PickWork(LookupItem{Name: "The Children of Hurin", Artist: "J.R.R. Tolkien"}, hurin); !ok || got.ID != "OL27513W" {
		t.Errorf("Húrin: %+v %v", got, ok)
	}
	beowulf := []openlibrary.Work{{ID: "OL1W", Title: "Beowulf", Authors: []string{"J.R.R. Tolkien"}, EditionCount: 12}}
	if _, ok := PickWork(LookupItem{Name: "Beowulf: A Translation and Commentary, together with Sellic Spell", Artist: "Tolkien, J. R. R."}, beowulf); !ok {
		t.Error("Beowulf with its subtitle")
	}
	if got := searchTitles("Five Years After: A John Matherson Novel, Book 4"); len(got) != 2 || got[0] != "Five Years After" {
		t.Errorf("search titles %q", got)
	}
	if got := searchTitles("Dune"); len(got) != 1 {
		t.Errorf("search titles %q", got)
	}
	if _, ok := PickWork(LookupItem{Name: "Dune", Artist: "Brian Herbert"}, works[:2]); ok {
		t.Error("matched without the author")
	}
}

func TestPickVolume(t *testing.T) {
	vols := []comicvine.Volume{
		{ID: 1, Name: "Saga", StartYear: 2012, IssueCount: 66},
		{ID: 2, Name: "Saga", StartYear: 1998, IssueCount: 4},
		{ID: 3, Name: "Saga of the Swamp Thing", StartYear: 1982, IssueCount: 65},
	}
	if got, ok := PickVolume(LookupItem{Name: "Saga"}, vols); !ok || got.ID != 1 {
		t.Errorf("no year: %+v", got)
	}
	if got, ok := PickVolume(LookupItem{Name: "Saga", Year: 1998}, vols); !ok || got.ID != 2 {
		t.Errorf("year 1998: %+v", got)
	}
	if _, ok := PickVolume(LookupItem{Name: "Saga", Year: 2020}, vols); ok {
		t.Error("matched a volume from another year")
	}
}

type fakeLookupStore struct {
	items    []LookupItem
	saved    map[int64]Found
	failures map[int64]string
}

func (f *fakeLookupStore) LookupItems(context.Context, int64, bool) ([]LookupItem, error) {
	return f.items, nil
}
func (f *fakeLookupStore) SaveLookup(_ context.Context, id int64, found Found) error {
	f.saved[id] = found
	return nil
}
func (f *fakeLookupStore) SaveMatchFailure(_ context.Context, id int64, status, _ string) error {
	f.failures[id] = status
	return nil
}

type fakeFinder struct{ needsArtist bool }

func (fakeFinder) Source() string      { return "Fake" }
func (f fakeFinder) NeedsArtist() bool { return f.needsArtist }
func (fakeFinder) Find(_ context.Context, it LookupItem) (Found, bool, error) {
	switch it.Name {
	case "Found":
		return Found{Name: "Found!"}, true, nil
	case "Broken":
		return Found{}, false, errors.New("503")
	case "Bad key":
		return Found{}, false, comicvine.ErrBadKey
	}
	return Found{}, false, nil
}

func TestLookupMatcher(t *testing.T) {
	st := &fakeLookupStore{items: []LookupItem{
		{ID: 1, Name: "Found", Artist: "A"}, {ID: 2, Name: "Missing", Artist: "A"},
		{ID: 3, Name: "Broken", Artist: "A"}, {ID: 4, Name: "Found"},
	}, saved: map[int64]Found{}, failures: map[int64]string{}}
	res, err := LookupMatcher{Store: st, Finder: fakeFinder{needsArtist: true}}.MatchLibrary(context.Background(), 1)
	if err != nil || res.Matched != 1 || res.NotFound != 2 || res.Errors != 1 {
		t.Fatalf("result %+v %v", res, err)
	}
	if st.saved[1].Name != "Found!" || st.failures[2] != "not_found" || st.failures[3] != "error" || st.failures[4] != "not_found" {
		t.Errorf("saved %v failures %v", st.saved, st.failures)
	}
	// A rejected key stops the run.
	st.items = []LookupItem{{ID: 5, Name: "Bad key"}, {ID: 6, Name: "Found"}}
	if _, err := (LookupMatcher{Store: st, Finder: fakeFinder{}}).RematchLibrary(context.Background(), 1); !errors.Is(err, comicvine.ErrBadKey) || st.saved[6].Name != "" {
		t.Errorf("bad key: %v", err)
	}
}
