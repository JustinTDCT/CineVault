package scan

import "testing"

func TestParseEpisode(t *testing.T) {
	cases := []struct {
		root, path      string
		show            string
		year            int
		season, ep, end int
		name            string
	}{
		// The test server's show.
		{"/home/jdube/tvshows", "/home/jdube/tvshows/3 Body Problem/Season 1/3 Body Problem - S01E03 - Destroyer of Worlds [WEBRip-1080p, x265 EAC3 Atmos].mkv",
			"3 Body Problem", 0, 1, 3, 0, "Destroyer of Worlds"},
		// Year and TMDb tag on the show folder.
		{"/tv", "/tv/Doctor Who (2005) {tmdb-57243}/Season 01/Doctor Who (2005) - S01E01 - Rose.mkv", "Doctor Who", 2005, 1, 1, 0, "Rose"},
		// Scene-style names, title cut at the quality words.
		{"/tv", "/tv/The.Expanse/Season 2/The.Expanse.S02E05.Home.1080p.WEB-DL.x264.mkv", "The Expanse", 0, 2, 5, 0, "Home"},
		// Multi-episode files.
		{"/tv", "/tv/Lost/Season 1/Lost - S01E01-E02 - Pilot.mkv", "Lost", 0, 1, 1, 2, "Pilot"},
		{"/tv", "/tv/Lost/Season 1/Lost.S01E01E02.mkv", "Lost", 0, 1, 1, 2, ""},
		{"/tv", "/tv/Lost/Season 1/Lost - S01E01-02 - Pilot.mkv", "Lost", 0, 1, 1, 2, "Pilot"},
		// Not a range: the number after the dash is a resolution.
		{"/tv", "/tv/Lost/Season 1/Lost S01E01-1080p.mkv", "Lost", 0, 1, 1, 0, ""},
		// 1x02 style and a file directly in the library folder.
		{"/tv", "/tv/Friends 1x02 The One with the Sonogram.avi", "Friends", 0, 1, 2, 0, "The One with the Sonogram"},
		// Specials folder means season 0.
		{"/tv", "/tv/Sherlock/Specials/Sherlock - S00E01 - The Abominable Bride.mkv", "Sherlock", 0, 0, 1, 0, "The Abominable Bride"},
		// A show whose name contains a number and an x.
		{"/tv", "/tv/9-1-1/Season 3/9-1-1 - S03E04 - Triggers.mkv", "9-1-1", 0, 3, 4, 0, "Triggers"},
	}
	for _, c := range cases {
		p, ok := ParseEpisode(c.root, c.path)
		if !ok || p.Show.Title != c.show || p.Show.Year != c.year || p.Season != c.season || p.Episode != c.ep || p.Last != c.end || p.Name != c.name {
			t.Errorf("%s:\n got %q %d S%dE%d-%d %q ok=%v\nwant %q %d S%dE%d-%d %q", c.path,
				p.Show.Title, p.Show.Year, p.Season, p.Episode, p.Last, p.Name, ok, c.show, c.year, c.season, c.ep, c.end, c.name)
		}
	}
	if p, _ := ParseEpisode("/tv", "/tv/Doctor Who (2005) {tmdb-57243}/Season 01/x S01E01.mkv"); p.Show.TMDbID != 57243 {
		t.Errorf("tmdb tag: %d", p.Show.TMDbID)
	}
	for _, bad := range []string{"/tv/Show/Season 1/Behind the scenes.mkv", "/tv/Show/1920x1080 test.mkv"} {
		if _, ok := ParseEpisode("/tv", bad); ok {
			t.Errorf("%s: expected no episode", bad)
		}
	}
}
