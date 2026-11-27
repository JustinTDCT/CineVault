package scan

import (
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/ebook"
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

func TestParseEbook(t *testing.T) {
	root := "/home/jdube/ebooks"
	dir := root + "/J.R.R. Tolkien/"
	types := []catalog.EditionType{{ID: "original", DisplayName: "Original", IsDefault: true}}
	cases := []struct {
		name   string
		file   string
		meta   ebook.Meta
		title  string
		author string
		series string
		index  float64
	}{
		{"series in the name, no metadata", "J.R.R. Tolkien - [The Lord of the Rings 05] - The Silmarillion.epub", ebook.Meta{},
			"The Silmarillion", "J.R.R. Tolkien", "The Lord of the Rings", 5},
		{"spaced initials match the folder author", "J R R Tolkien - The Story of Kullervo (epub).epub", ebook.Meta{},
			"The Story of Kullervo", "J.R.R. Tolkien", "", 0},
		{"stand-alone isn't a series", "J.R.R. Tolkien - [Stand-Alone 01] - The Legend of Sigurd & Gudrun.epub", ebook.Meta{},
			"The Legend of Sigurd & Gudrun", "J.R.R. Tolkien", "", 0},
		{"half numbers", "J.R.R. Tolkien - [The Lord of the Rings 4.50] - Bilbo's Last Song.epub", ebook.Meta{},
			"Bilbo's Last Song", "J.R.R. Tolkien", "The Lord of the Rings", 4.5},
		{"metadata wins", "J R R Tolkien - Beowulf- A Translation and Commentary (azw3).azw3",
			ebook.Meta{Title: "Beowulf: A Translation and Commentary", Authors: []string{"J.R.R. Tolkien"}},
			"Beowulf: A Translation and Commentary", "J.R.R. Tolkien", "", 0},
		{"no metadata, junk removed", "J R R Tolkien - The Fall of Gondolin Christopher Tolkien (ed) (retail) (azw3).azw3", ebook.Meta{},
			"The Fall of Gondolin Christopher Tolkien", "J.R.R. Tolkien", "", 0},
		{"series from the book's own metadata", "x.epub",
			ebook.Meta{Title: "The Two Towers", Authors: []string{"J.R.R. Tolkien"}, Series: "The Lord of the Rings", SeriesIndex: 2},
			"The Two Towers", "J.R.R. Tolkien", "The Lord of the Rings", 2},
	}
	for _, c := range cases {
		b, ok := ParseEbook(root, dir+c.file, c.meta, types, "original")
		got := []any{b.Book.Title, b.Book.Artist, b.Series, b.SeriesIndex}
		want := []any{c.title, c.author, c.series, c.index}
		for i := range got {
			if got[i] != want[i] || !ok {
				t.Errorf("%s:\n got %v\nwant %v", c.name, got, want)
				break
			}
		}
	}
}
