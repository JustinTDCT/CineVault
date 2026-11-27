package scan

import (
	"testing"

	"github.com/JustinTDCT/CineVault/internal/comic"
)

func TestParseComic(t *testing.T) {
	root := "/home/jdube/comicbooks"
	cases := []struct {
		path   string
		info   *comic.Info
		series string
		year   int
		number float64
		issue  string
	}{
		{root + "/Aliens - Book One/Aliens - Book One - 2.cbr", nil, "Aliens - Book One", 0, 2, "Issue 2"},
		{root + "/Aliens - Book One/Aliens - Book One - 6.cbr", nil, "Aliens - Book One", 0, 6, "Issue 6"},
		{root + "/Saga (2012)/Saga #054 (2018) (Digital) (Zone-Empire).cbz", nil, "Saga", 2012, 54, "Issue 54"},
		{root + "/Saga (2012)/Saga 001.cbz", &comic.Info{Series: "Saga", Number: "1", Title: "Chapter One", Summary: "Two soldiers."}, "Saga", 2012, 1, "Chapter One"},
		{root + "/Watchmen 005 (1987).cbz", nil, "Watchmen", 1987, 5, "Issue 5"},
		{root + "/loose.cbz", &comic.Info{Series: "Batman: Year One", Number: "2.5"}, "Batman: Year One", 0, 2.5, "Issue 2.5"},
		{root + "/Maus/Maus.cbz", nil, "Maus", 0, 0, "Maus"},
	}
	for _, c := range cases {
		p := ParseComic(root, c.path, c.info)
		if p.Series.Title != c.series || p.Series.Year != c.year || p.Number != c.number || p.Issue != c.issue {
			t.Errorf("%s:\n got %q %d #%g %q\nwant %q %d #%g %q", c.path, p.Series.Title, p.Series.Year, p.Number, p.Issue, c.series, c.year, c.number, c.issue)
		}
	}
	if issueTrack(2.5) != 250 || issueTrack(2) != 200 {
		t.Error("issueTrack")
	}
}
