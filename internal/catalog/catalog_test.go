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

func TestLinkMultiEpisodeFiles(t *testing.T) {
	seasons := []Season{{Number: 1, Episodes: []Episode{
		{Number: 1, Versions: []Version{{ID: 9, LastEpisode: 3}}},
		{Number: 2},
		{Number: 3, Versions: []Version{{ID: 10}}}, // has its own file too
		{Number: 4},
	}}}
	LinkMultiEpisodeFiles(seasons)
	eps := seasons[0].Episodes
	if eps[1].PartOf != 1 || eps[2].PartOf != 0 || eps[3].PartOf != 0 {
		t.Errorf("part_of = %d %d %d", eps[1].PartOf, eps[2].PartOf, eps[3].PartOf)
	}
}
