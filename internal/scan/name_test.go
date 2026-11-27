package scan

import (
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
)

// movieTypes mirrors the movie entries seeded by migration 0001.
var movieTypes = []catalog.EditionType{
	{ID: "theatrical", DisplayName: "Theatrical", Aliases: []string{"theatrical"}},
	{ID: "directors_cut", DisplayName: "Director's Cut", Aliases: []string{"directors cut", "director's cut", "directors.cut", "dc"}},
	{ID: "extended", DisplayName: "Extended", Aliases: []string{"extended", "extended edition", "extended cut"}},
	{ID: "unrated", DisplayName: "Unrated", Aliases: []string{"unrated"}},
	{ID: "uncut", DisplayName: "Uncut", Aliases: []string{"uncut"}},
	{ID: "special_edition", DisplayName: "Special Edition", Aliases: []string{"special edition", "se"}},
	{ID: "remastered", DisplayName: "Remastered", Aliases: []string{"remastered", "remaster"}},
	{ID: "final_cut", DisplayName: "Final/Ultimate Cut", Aliases: []string{"final cut", "ultimate cut", "ultimate edition"}},
	{ID: "imax", DisplayName: "IMAX", Aliases: []string{"imax", "imax enhanced"}},
	{ID: "black_and_white", DisplayName: "Black & White", Aliases: []string{"black and white", "black & white", "b&w", "chrome"}},
}

func TestParseName(t *testing.T) {
	cases := []struct {
		path, title string
		year        int
		edition     string
		source      string
	}{
		// The two files on the test server.
		{"/home/jdube/movies/Alien - Remastered (1979) [Bluray-1080p x265].mkv", "Alien", 1979, "remastered", EditionAlias},
		{"/home/jdube/movies/Alien {edition-Director's Cut} (1979) [1080p-MP4].mkv", "Alien", 1979, "directors_cut", EditionExplicit},

		// Plain theatrical file falls back to the default.
		{"/m/Aliens (1986)/Aliens (1986).mkv", "Aliens", 1986, "theatrical", EditionDefault},
		// Edition after the year and in brackets.
		{"/m/Blade Runner (1982) Final Cut.mkv", "Blade Runner", 1982, "final_cut", EditionAlias},
		{"/m/Mad Max Fury Road (2015) [Black & White] [2160p].mkv", "Mad Max Fury Road", 2015, "black_and_white", EditionAlias},
		{"/m/Kingdom of Heaven (2005) [Directors Cut][1080p].mkv", "Kingdom of Heaven", 2005, "directors_cut", EditionAlias},
		// Dotted scene-style names.
		{"/m/The.Lord.of.the.Rings.The.Two.Towers.2002.EXTENDED.1080p.BluRay.x264.mkv", "The Lord of the Rings The Two Towers", 2002, "extended", EditionAlias},
		// Titles that contain a keyword or " - " must not be cut.
		{"/m/Uncut Gems (2019).mkv", "Uncut Gems", 2019, "theatrical", EditionDefault},
		{"/m/Mission Impossible - Fallout (2018).mkv", "Mission Impossible - Fallout", 2018, "theatrical", EditionDefault},
		// Years inside titles.
		{"/m/1917 (2019).mkv", "1917", 2019, "theatrical", EditionDefault},
		{"/m/2001 A Space Odyssey 1968.mkv", "2001 A Space Odyssey", 1968, "theatrical", EditionDefault},
		// Year only on the folder.
		{"/m/Alien (1979)/Alien Directors Cut.mkv", "Alien", 1979, "directors_cut", EditionAlias},
		// An edition tag that isn't on the list.
		{"/m/Alien {edition-Workprint} (1979).mkv", "Alien", 1979, "theatrical", EditionUnmatched},
		// Multi-word keywords inside brackets.
		{"/m/Dune (2021) [IMAX Enhanced].mkv", "Dune", 2021, "imax", EditionAlias},
	}
	for _, c := range cases {
		p := ParseName(c.path, movieTypes, "theatrical")
		if p.Title != c.title || p.Year != c.year || p.EditionTypeID != c.edition || p.EditionSource != c.source {
			t.Errorf("%s\n  got  title=%q year=%d edition=%s (%s)\n  want title=%q year=%d edition=%s (%s)",
				c.path, p.Title, p.Year, p.EditionTypeID, p.EditionSource, c.title, c.year, c.edition, c.source)
		}
	}
}

func TestParseNameExtras(t *testing.T) {
	p := ParseName("/m/Alien {tmdb-348} {edition-Workprint} (1979).mkv", movieTypes, "theatrical")
	if p.TMDbID != 348 || p.UnmatchedLabel != "Workprint" {
		t.Errorf("got %+v", p)
	}
}

func TestSortName(t *testing.T) {
	for in, want := range map[string]string{"The Thing": "thing", "A Bug's Life": "bug's life", "An American Tail": "american tail", "Alien": "alien", "The": "the"} {
		if got := SortName(in); got != want {
			t.Errorf("SortName(%q) = %q, want %q", in, got, want)
		}
	}
}
