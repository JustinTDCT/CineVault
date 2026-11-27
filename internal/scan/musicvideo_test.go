package scan

import (
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
)

var mvTypes = []catalog.EditionType{
	{ID: "official_video", DisplayName: "Official Video", Aliases: []string{"official video", "official music video", "official"}, IsDefault: true},
	{ID: "acoustic", DisplayName: "Acoustic/Unplugged", Aliases: []string{"acoustic", "unplugged", "mtv unplugged"}},
	{ID: "live", DisplayName: "Live", Aliases: []string{"live"}},
	{ID: "alternate_version", DisplayName: "Alternate Version", Aliases: []string{"alternate version", "us version", "u s version"}},
	{ID: "lyric_video", DisplayName: "Lyric Video", Aliases: []string{"lyric video", "lyrics"}},
}

func TestParseMusicVideo(t *testing.T) {
	cases := []struct {
		path, artist, song, edition string
		year                        int
	}{
		// The test server's files.
		{"/mv/311 - Come Original.mp4", "311", "Come Original", "official_video", 0},
		{"/mv/Alice In Chains - No Excuses (MTV Unplugged).mp4", "Alice In Chains", "No Excuses", "acoustic", 0},
		{"/mv/Ace of Base - Cruel Summer (U.S. Version).mp4", "Ace of Base", "Cruel Summer", "alternate_version", 0},
		{"/mv/3 Doors Down - It's Not My Time.mp4", "3 Doors Down", "It's Not My Time", "official_video", 0},
		// Other common shapes.
		{"/mv/Queen - Bohemian Rhapsody (Official Video) (1975).mp4", "Queen", "Bohemian Rhapsody", "official_video", 1975},
		{"/mv/Nirvana - Lithium - Live.mkv", "Nirvana", "Lithium", "live", 0},
		{"/mv/Daft Punk - Get Lucky (feat. Pharrell Williams) [Lyric Video].mp4", "Daft Punk", "Get Lucky (feat. Pharrell Williams)", "lyric_video", 0},
		{"/mv/Radiohead/Karma Police.mp4", "Radiohead", "Karma Police", "official_video", 0},
		{"/mv/Loose Song.mp4", "", "Loose Song", "official_video", 0},
		// A song title that starts with brackets keeps them.
		{"/mv/The Rolling Stones - (I Can't Get No) Satisfaction.mp4", "The Rolling Stones", "(I Can't Get No) Satisfaction", "official_video", 0},
	}
	for _, c := range cases {
		p := ParseMusicVideo("/mv", c.path, mvTypes, "official_video")
		if p.Artist != c.artist || p.Title != c.song || p.EditionTypeID != c.edition || p.Year != c.year {
			t.Errorf("%s:\n got %q / %q / %s / %d\nwant %q / %q / %s / %d", c.path, p.Artist, p.Title, p.EditionTypeID, p.Year, c.artist, c.song, c.edition, c.year)
		}
	}
	if p := ParseMusicVideo("/mv", "/mv/Alice Cooper - Poison.mp4", mvTypes, "official_video"); p.SortName != "alice cooper poison" {
		t.Errorf("sort name = %q", p.SortName)
	}
}
