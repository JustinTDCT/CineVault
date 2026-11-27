package editionlist

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestListShape(t *testing.T) {
	all := All()
	if len(all) != 43 {
		t.Fatalf("got %d built-in entries, want 43 (migrations 0001 and 0004)", len(all))
	}
	seen := map[string]bool{}
	for i, e := range all {
		if seen[e.ID] {
			t.Errorf("duplicate ID %q", e.ID)
		}
		seen[e.ID] = true
		if e.DisplayName == "" || e.ShortLabel == "" || len(e.MediaTypes) == 0 {
			t.Errorf("%s: missing display name, short label or media types", e.ID)
		}
		for _, m := range e.MediaTypes {
			if !isMediaType(m) {
				t.Errorf("%s: unknown media type %q", e.ID, m)
			}
		}
		if i > 0 && all[i-1].SortOrder > e.SortOrder {
			t.Errorf("%s: not in sort order", e.ID)
		}
	}
}

func isMediaType(m string) bool {
	for _, x := range MediaTypes {
		if x == m {
			return true
		}
	}
	return false
}

func TestEveryMediaTypeHasOneDefault(t *testing.T) {
	want := map[string]string{
		Movie: "theatrical", TV: "broadcast", Music: "original", MusicVideo: "official_video",
		Audiobook: "unabridged", Book: "original", Comic: "original",
	}
	for _, m := range MediaTypes {
		n := 0
		for _, e := range ForMediaType(m) {
			if e.IsDefault {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s has %d default editions, want 1", m, n)
		}
		d, ok := Default(m)
		if !ok || d.ID != want[m] {
			t.Errorf("Default(%s) = %q, want %q", m, d.ID, want[m])
		}
	}
}

func TestLookupAndPosition(t *testing.T) {
	e, ok := Lookup("directors_cut")
	if !ok || e.DisplayName != "Director's Cut" || e.ShortLabel != "DC" {
		t.Fatalf("Lookup(directors_cut) = %+v, %v", e, ok)
	}
	if !e.AppliesTo(Movie) || !e.AppliesTo(TV) || e.AppliesTo(Music) {
		t.Errorf("directors_cut media types = %v", e.MediaTypes)
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup(nope) found something")
	}
	// Broadcast (5) sorts before Theatrical (10).
	if Position("broadcast") >= Position("theatrical") || Position("nope") != -1 {
		t.Errorf("positions: broadcast %d, theatrical %d, nope %d",
			Position("broadcast"), Position("theatrical"), Position("nope"))
	}
}

func TestResultsAreCopies(t *testing.T) {
	a := All()
	a[0].Aliases[0] = "changed"
	if All()[0].Aliases[0] == "changed" {
		t.Error("All returned shared slices")
	}
}

func TestMentions(t *testing.T) {
	cases := []struct {
		id, text string
		want     bool
	}{
		{"directors_cut", "The Director's Cut runs 117 minutes.", true},
		{"directors_cut", "ALIENS.DIRECTORS.CUT.1986.mkv", true},
		{"special_edition", "The 1991 Special Edition runs 154 minutes.", true},
		{"extended", "Aliens was released in 1986 at 137 minutes.", false},
		{"theatrical", "The theatrical cut is 137 minutes.", true},
		{"three_d", "Shown in 3D at selected cinemas.", true},
		{"black_and_white", "A Black & White version was released.", true},
		{"directors_cut", "Produced by DCU Films.", false}, // "dc" only as a whole word
		{"remastered", "", false},
	}
	for _, c := range cases {
		e, ok := Lookup(c.id)
		if !ok {
			t.Fatalf("no entry %s", c.id)
		}
		if got := e.Mentions(c.text); got != c.want {
			t.Errorf("%s.Mentions(%q) = %v, want %v", c.id, c.text, got, c.want)
		}
	}
}

// TestVersionPinned fails when the built-in list changes without Version
// going up. After changing the list on purpose, bump Version and put the new
// hash here.
func TestVersionPinned(t *testing.T) {
	const want = "bc340dc09c11896fb942900d4b60cb933824da76c0abca127ca6c38d26d698ae"
	b, err := json.Marshal(All())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); Version != 2 || got != want {
		t.Fatalf("Edition List changed (hash %s, Version %d): bump Version and update this test", got, Version)
	}
}
