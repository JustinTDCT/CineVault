package scan

import (
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
)

var musicTypes = []catalog.EditionType{
	{ID: "original", DisplayName: "Original", IsDefault: true},
	{ID: "deluxe", DisplayName: "Deluxe/Expanded", Aliases: []string{"deluxe", "deluxe edition", "expanded edition"}},
	{ID: "remaster", DisplayName: "Remaster", Aliases: []string{"remaster", "remastered"}},
}

func tags(kv ...string) mediainfo.Info {
	i := mediainfo.Info{Tags: map[string]string{}}
	for n := 0; n < len(kv); n += 2 {
		i.Tags[kv[n]] = kv[n+1]
	}
	return i
}

func TestParseTrack(t *testing.T) {
	root := "/home/jdube/music"
	cases := []struct {
		name                   string
		path                   string
		info                   mediainfo.Info
		album, artist, edition string
		year, disc, num        int
		song, trackArtist      string
	}{
		{"test server file, no tags", root + "/3 Doors Down/Away From the Sun (2002)/3 Doors Down - 08 - Dangerous Game [CD].flac", tags(),
			"Away From the Sun", "3 Doors Down", "original", 2002, 1, 8, "Dangerous Game", ""},
		{"tags win", root + "/3 Doors Down/Away From the Sun (2002)/x.flac",
			tags("album", "Away From the Sun", "album_artist", "3 Doors Down", "artist", "3 Doors Down", "title", "When I'm Gone", "track", "1/12", "date", "2002-11-12"),
			"Away From the Sun", "3 Doors Down", "original", 2002, 1, 1, "When I'm Gone", ""},
		{"brackets in the song stay", root + "/3 Doors Down/Time of My Life (2011)/3 Doors Down - 13 - When You’re Young (acoustic) [CD].flac", tags(),
			"Time of My Life", "3 Doors Down", "original", 2011, 1, 13, "When You’re Young (acoustic)", ""},
		{"deluxe edition album", root + "/Adele/25 (Deluxe Edition) (2015)/01 Hello.flac", tags(),
			"25", "Adele", "deluxe", 2015, 1, 1, "Hello", ""},
		{"disc folders", root + "/Pink Floyd/The Wall (1979)/CD2/01 - Hey You.mp3", tags(),
			"The Wall", "Pink Floyd", "original", 1979, 2, 1, "Hey You", ""},
		{"disc-track file names", root + "/Pink Floyd/The Wall/2-03 Nobody Home.mp3", tags(),
			"The Wall", "Pink Floyd", "original", 0, 2, 3, "Nobody Home", ""},
		{"folder edition wins over a plain album tag", root + "/3 Doors Down/Time of My Life (Deluxe Edition) (2011)/x.flac",
			tags("album", "Time of My Life", "title", "Believer", "track", "12", "date", "2011"),
			"Time of My Life", "3 Doors Down", "deluxe", 2011, 1, 12, "Believer", ""},
		{"compilation keeps track artists", root + "/Various Artists/Now 1 (1983)/05 x.mp3",
			tags("album_artist", "Various Artists", "artist", "Duran Duran", "title", "Is There Something I Should Know"),
			"Now 1", "Various Artists", "original", 1983, 1, 5, "Is There Something I Should Know", "Duran Duran"},
	}
	for _, c := range cases {
		p, ok := ParseTrack(root, c.path, c.info, musicTypes, "original")
		got := []any{p.Album.Title, p.Album.Artist, p.Album.EditionTypeID, p.Album.Year, p.Disc, p.Number, p.Name, p.Artist}
		want := []any{c.album, c.artist, c.edition, c.year, c.disc, c.num, c.song, c.trackArtist}
		for i := range got {
			if got[i] != want[i] || !ok {
				t.Errorf("%s:\n got %v\nwant %v", c.name, got, want)
				break
			}
		}
	}
	p, _ := ParseTrack(root, root+"/3 Doors Down/Away From the Sun (2002)/3 Doors Down - 08 - Dangerous Game [CD].flac", tags(), musicTypes, "original")
	if p.Album.SortName != "3 doors down 2002 away from the sun" {
		t.Errorf("sort name = %q", p.Album.SortName)
	}
}
