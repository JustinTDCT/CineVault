package scan

import (
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
)

var bookTypes = []catalog.EditionType{
	{ID: "unabridged", DisplayName: "Unabridged", IsDefault: true},
	{ID: "abridged", DisplayName: "Abridged", Aliases: []string{"abridged"}},
	{ID: "dramatized", DisplayName: "Dramatized", Aliases: []string{"dramatized", "full cast"}},
}

func TestParseBook(t *testing.T) {
	root := "/home/jdube/audiobooks"
	withChapters := func(i mediainfo.Info) mediainfo.Info {
		i.Chapters = []mediainfo.Chapter{{Start: 0, End: 10, Name: "Opening"}, {Start: 10, End: 20, Name: "Chapter 1"}}
		return i
	}
	cases := []struct {
		name                     string
		path                     string
		info                     mediainfo.Info
		book, author, series     string
		index                    float64
		year, part               int
		partName, narrator, edit string
	}{
		{"test server, no tags", root + "/The John Matherson Series/One Second After.m4b", tags(),
			"One Second After", "Unknown author", "The John Matherson Series", 0, 0, 1, "One Second After", "", "unabridged"},
		{"test server, with Audible-style tags", root + "/The John Matherson Series/One Second After.m4b",
			tags("album", "One Second After", "artist", "William R. Forstchen", "composer", "Joe Barrett", "date", "2009"),
			"One Second After", "William R. Forstchen", "The John Matherson Series", 0, 2009, 1, "One Second After", "Joe Barrett", "unabridged"},
		{"author/series/book folders of MP3s", root + "/Brandon Sanderson/Mistborn/The Final Empire/03 - Chapter Three.mp3", tags(),
			"The Final Empire", "Brandon Sanderson", "Mistborn", 0, 0, 3, "Chapter Three", "", "unabridged"},
		{"series tag in the name", root + "/J.R.R. Tolkien/J.R.R. Tolkien - [The Lord of the Rings 02] - The Fellowship of the Ring.m4b", tags(),
			"The Fellowship of the Ring", "J.R.R. Tolkien", "The Lord of the Rings", 2, 0, 1, "The Fellowship of the Ring", "", "unabridged"},
		{"abridged and an mp3 with chapters is a whole book", root + "/Stephen King/The Stand (Abridged).mp3", withChapters(tags()),
			"The Stand", "Stephen King", "", 0, 0, 1, "The Stand", "", "abridged"},
	}
	for _, c := range cases {
		b, ok := ParseBook(root, c.path, c.info, bookTypes, "unabridged")
		got := []any{b.Book.Title, b.Book.Artist, b.Series, b.SeriesIndex, b.Book.Year, b.Part, b.PartName, b.Narrator, b.Book.EditionTypeID}
		want := []any{c.book, c.author, c.series, c.index, c.year, c.part, c.partName, c.narrator, c.edit}
		for i := range got {
			if got[i] != want[i] || !ok {
				t.Errorf("%s:\n got %v\nwant %v", c.name, got, want)
				break
			}
		}
	}
}
