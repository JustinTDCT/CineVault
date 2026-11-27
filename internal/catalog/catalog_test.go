package catalog

import "testing"

func TestAppliesTo(t *testing.T) {
	dc := EditionType{ID: "directors_cut", MediaTypes: []string{"movie", "tv"}}
	if !dc.AppliesTo("movie") || !dc.AppliesTo("tv") || dc.AppliesTo("music") {
		t.Errorf("AppliesTo wrong for %+v", dc)
	}
}

func TestValidMediaType(t *testing.T) {
	for _, m := range []string{"movie", "tv", "music", "audiobook", "book"} {
		if !ValidMediaType(m) {
			t.Errorf("%s should be valid", m)
		}
	}
	for _, m := range []string{"", "podcast", "Movie"} {
		if ValidMediaType(m) {
			t.Errorf("%q should be invalid", m)
		}
	}
}
