package match

import (
	"context"
	"errors"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/musicbrainz"
)

func rec(id, title, artist string, score int, rels ...musicbrainz.Release) musicbrainz.Recording {
	return musicbrainz.Recording{ID: id, Title: title, Artist: artist, Score: score, Releases: rels}
}

var (
	studio = musicbrainz.Release{Title: "Hunting High and Low", Status: "Official", PrimaryType: "Album", GroupDate: "1985-06-01"}
	single = musicbrainz.Release{Title: "Take On Me", Status: "Official", PrimaryType: "Single", GroupDate: "1984-10-19"}
	comp   = musicbrainz.Release{Title: "Headlines and Deadlines", Status: "Official", PrimaryType: "Album", SecondaryTypes: []string{"Compilation"}, GroupDate: "1991-11-04"}
)

func TestPickRecording(t *testing.T) {
	v := MusicVideo{Artist: "a-ha", Song: "Take On Me"}
	got, ok := PickRecording(v, []musicbrainz.Recording{
		rec("live", "Take On Me (live)", "a-ha", 100, studio), // another title
		rec("cover", "Take On Me", "Weezer", 100, studio),     // another artist
		rec("low", "Take On Me", "a-ha", 60, studio),          // weak score
		rec("single", "Take On Me", "a-ha", 100, single),      // only a single
		rec("album", "Take on Me", "a-ha", 95, comp, studio),  // on the studio album
	})
	if !ok || got.ID != "album" {
		t.Fatalf("picked %+v", got)
	}
	if a, _ := got.Album(); a.Title != "Hunting High and Low" {
		t.Errorf("album %+v", a)
	}
	if _, ok := PickRecording(v, []musicbrainz.Recording{rec("x", "Something Else", "a-ha", 100)}); ok {
		t.Error("matched another song")
	}
	// "feat." credits still name the artist.
	if got, ok := PickRecording(MusicVideo{Artist: "Queen", Song: "Under Pressure"},
		[]musicbrainz.Recording{rec("up", "Under Pressure", "Queen & David Bowie", 100, studio)}); !ok || got.ID != "up" {
		t.Errorf("joint credit: %+v %v", got, ok)
	}
	// No artist on either side is never a match: song titles repeat.
	if _, ok := PickRecording(MusicVideo{Song: "Take On Me"}, []musicbrainz.Recording{rec("a", "Take On Me", "a-ha", 100, studio)}); ok {
		t.Error("matched without an artist")
	}
	if _, ok := PickRecording(v, []musicbrainz.Recording{rec("b", "Take On Me", "", 100, studio)}); ok {
		t.Error("matched a recording with no artist credit")
	}
}

type fakeMV struct {
	videos   []MusicVideo
	matches  map[int64]MusicVideoMatch
	failures map[int64]string
}

func (f *fakeMV) MusicVideosToMatch(context.Context, int64, bool) ([]MusicVideo, error) {
	return f.videos, nil
}
func (f *fakeMV) SaveMusicVideoMatch(_ context.Context, id int64, m MusicVideoMatch) error {
	f.matches[id] = m
	return nil
}
func (f *fakeMV) SaveMatchFailure(_ context.Context, id int64, status, _ string) error {
	f.failures[id] = status
	return nil
}

type fakeMB struct{}

func (fakeMB) SearchRecordings(_ context.Context, artist, title string) ([]musicbrainz.Recording, error) {
	switch title {
	case "Take On Me":
		return []musicbrainz.Recording{rec("4a1b6c7e-2d3f-4e5a-9b8c-7d6e5f4a3b2c", "Take On Me", "a-ha", 100, single, studio)}, nil
	case "Broken":
		return nil, errors.New("MusicBrainz answered 503")
	}
	return nil, nil
}

func TestMusicVideoMatcher(t *testing.T) {
	st := &fakeMV{videos: []MusicVideo{{ID: 1, Artist: "a-ha", Song: "Take On Me"}, {ID: 2, Artist: "Nobody", Song: "Unknown Song"}, {ID: 3, Artist: "x", Song: "Broken"}, {ID: 4, Song: "Take On Me"}},
		matches: map[int64]MusicVideoMatch{}, failures: map[int64]string{}}
	res, err := MusicVideoMatcher{Store: st, Source: fakeMB{}}.MatchLibrary(context.Background(), 1)
	if err != nil || res.Matched != 1 || res.NotFound != 2 || res.Errors != 1 {
		t.Fatalf("result %+v %v", res, err)
	}
	m := st.matches[1]
	if m.RecordingID != "4a1b6c7e-2d3f-4e5a-9b8c-7d6e5f4a3b2c" || m.Album != "Hunting High and Low" || m.Year != 1985 || m.Artist != "a-ha" {
		t.Errorf("match %+v", m)
	}
	if st.failures[2] != "not_found" || st.failures[3] != "error" || st.failures[4] != "not_found" {
		t.Errorf("failures %v", st.failures)
	}
}

// From MusicBrainz's answer for Ace of Base "Beautiful Life" (2026-11-28):
// every compilation's copy is its own recording, all scoring 100, and the
// album recording isn't first.
func TestPickRecordingAmongCompilations(t *testing.T) {
	va := func(id, title, date string, secondary ...string) musicbrainz.Recording {
		return rec(id, "Beautiful Life", "Ace of Base", 100, musicbrainz.Release{Title: title, Status: "Official",
			PrimaryType: "Album", SecondaryTypes: secondary, Date: date, Artist: "Various Artists"})
	}
	recs := []musicbrainz.Recording{
		rec("top", "Beautiful Life", "Ace of Base", 100, musicbrainz.Release{Title: "Top Surprise 3", PrimaryType: "Album", Artist: "Various Artists"}),
		va("dm8", "Dance Machine 8", "1996-03", "Compilation", "DJ-mix"),
		va("postens", "Postens landeplager: Brennmanetsommer 2", "1996"), // not marked a compilation
		va("roxbury", "A Night at the Roxbury", "1998", "Compilation", "Soundtrack"),
		rec("bridge", "Beautiful Life", "Ace of Base", 100,
			musicbrainz.Release{Title: "The Bridge", Status: "Official", PrimaryType: "Album", Date: "1995", Artist: "Ace of Base"},
			musicbrainz.Release{Title: "The Bridge", Status: "Official", PrimaryType: "Album", Artist: "Ace of Base"}),
	}
	got, ok := PickRecording(MusicVideo{Artist: "Ace of Base", Song: "Beautiful Life"}, recs)
	if !ok || got.ID != "bridge" {
		t.Fatalf("picked %+v", got)
	}
	if a, _ := got.Album(); a.Title != "The Bridge" {
		t.Errorf("album %+v", a)
	}
}
