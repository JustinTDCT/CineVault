package scan

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/ebook"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
)

// ParsedBook is what an audiobook file's tags and path say about it.
type ParsedBook struct {
	// Book is the book as a title: Title, Artist (the author), Year and the
	// edition (Unabridged, Abridged, Dramatized...).
	Book        ParsedName
	Series      string
	SeriesIndex float64 // 0 when unknown
	Narrator    string
	Overview    string
	// Part and PartName place a file within a book made of several files
	// (a folder of MP3s). A whole-book file (.m4b, or any file with
	// chapters) is part 1, named after the book.
	Part     int
	PartName string
}

var (
	// "[The Lord of the Rings 05]", "[Stand-Alone 01]", "[History of Middle-earth 4.50]"
	reSeriesTag = regexp.MustCompile(`\[([^\[\]]*?)\s+(\d{1,3}(?:\.\d{1,2})?)\]`)
	// "Book 2", "Book Two" is left alone; "#3"
	reBookNumber = regexp.MustCompile(`(?i)(?:\bbook\s+|#)(\d{1,3}(?:\.\d{1,2})?)\b`)
	// A folder named for a series: "The John Matherson Series", "Dune Saga".
	reSeriesDir = regexp.MustCompile(`(?i)\b(series|saga|trilogy|chronicles|cycle|quartet|duology)$`)
	// Download leftovers: "(retail)", "(epub)", "(azw3)".
	reFileJunk   = regexp.MustCompile(`(?i)\s*\((?:retail|epub|azw3?|mobi|pdf|ebook|kindle|ed)\)`)
	reStandAlone = regexp.MustCompile(`(?i)^(?:stand[- ]?alones?|standalones?|novels?|books?)$`)
	// " - - " left behind when a bracketed tag is taken out of "A - [x] - B".
	reEmptyDash = regexp.MustCompile(`(?:\s+-)+\s+-\s+`)
)

// ParseBook works out the book, author, series and part from an audiobook
// file. Tags come first, then the folders: Author/[Series/]Book/part files,
// or Author-or-Series/Book.m4b.
func ParseBook(root, path string, info mediainfo.Info, types []catalog.EditionType, defaultEdition string) (ParsedBook, bool) {
	whole := strings.EqualFold(filepath.Ext(path), ".m4b") || len(info.Chapters) > 1
	return parseBook(root, path, info, types, defaultEdition, whole)
}

// ParseEbook works out an ebook's title, author and series from what the file
// says about itself (meta) and its path, the same way as audiobooks.
func ParseEbook(root, path string, meta ebook.Meta, types []catalog.EditionType, defaultEdition string) (ParsedBook, bool) {
	info := mediainfo.Info{Tags: map[string]string{
		"title": meta.Title, "artist": strings.Join(meta.Authors, " & "), "description": meta.Description,
		"date": meta.Date, "series": meta.Series,
	}}
	if meta.SeriesIndex > 0 {
		info.Tags["series_index"] = strconv.FormatFloat(meta.SeriesIndex, 'f', -1, 64)
	}
	return parseBook(root, path, info, types, defaultEdition, true)
}

func parseBook(root, path string, info mediainfo.Info, types []catalog.EditionType, defaultEdition string, whole bool) (ParsedBook, bool) {
	var b ParsedBook
	b.Book = ParsedName{EditionTypeID: defaultEdition, EditionSource: EditionDefault}

	var dirs []string
	if rel, err := filepath.Rel(root, filepath.Dir(path)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		dirs = strings.Split(filepath.ToSlash(rel), "/")
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	whole = whole || len(dirs) == 0

	// The folders above the book: the nearest may be a series, the next the author.
	var above []string
	title := ""
	if whole {
		title = info.Tag("album", "title")
		if title == "" {
			title = base
		}
		above = dirs
	} else {
		title = info.Tag("album")
		if title == "" && len(dirs) > 0 {
			title = dirs[len(dirs)-1]
		}
		if len(dirs) > 0 {
			above = dirs[:len(dirs)-1]
		}
	}

	// Series from "[Series 02]" in the title or file name.
	for _, s := range []string{title, base} {
		if m := reSeriesTag.FindStringSubmatch(s); m != nil {
			b.Series = strings.TrimSpace(m[1])
			b.SeriesIndex, _ = strconv.ParseFloat(m[2], 64)
			break
		}
	}
	title = reSeriesTag.ReplaceAllString(title, "")
	title, _ = extractBrackets(title)
	title = reFileJunk.ReplaceAllString(title, "")
	title = reEmptyDash.ReplaceAllString(reSpaces.ReplaceAllString(title, " "), " - ")
	title = strings.Trim(strings.TrimSpace(title), "- ")

	// Series and author from the folders above the book: the top one is the
	// author and the one nearest the book a series ("Author/Series/Book").
	// A single folder is a series only if it's named like one.
	folderAuthor := ""
	switch len(above) {
	case 0:
	case 1:
		if d := strings.TrimSpace(above[0]); reSeriesDir.MatchString(d) {
			if b.Series == "" {
				b.Series = d
			}
		} else {
			folderAuthor = d
		}
	default:
		folderAuthor = strings.TrimSpace(above[0])
		if b.Series == "" {
			b.Series = strings.TrimSpace(above[len(above)-1])
		}
	}

	// The author: tags, then the folder, then "Author - Title" in the name.
	author := info.Tag("album_artist", "artist", "author")
	if author == "" {
		author = folderAuthor
	}
	if i := strings.Index(title, " - "); i > 0 {
		left, right := strings.TrimSpace(title[:i]), strings.TrimSpace(title[i+3:])
		if author == "" || normalizeKey(left) == normalizeKey(author) || normalizeKey(left) == normalizeKey(folderAuthor) {
			if author == "" {
				author = left
			}
			title = right
		}
	}
	title, edition, year := trimEditionSuffixes(title, types)
	if edition != "" {
		b.Book.EditionTypeID, b.Book.EditionSource = edition, EditionAlias
	}
	if author == "" {
		author = "Unknown author"
	}

	if b.Series == "" {
		b.Series = info.Tag("series", "mvnm", "movementname", "grouping")
	}
	if reStandAlone.MatchString(b.Series) { // "[Stand-Alone 01]" isn't a series
		b.Series, b.SeriesIndex = "", 0
	}
	if b.SeriesIndex == 0 {
		if n, err := strconv.ParseFloat(strings.TrimSpace(info.Tag("series-part", "series_index", "mvin", "movement")), 64); err == nil {
			b.SeriesIndex = n
		} else if m := reBookNumber.FindStringSubmatch(base); m != nil {
			b.SeriesIndex, _ = strconv.ParseFloat(m[1], 64)
		}
	}
	if m := reYearPrefix.FindString(info.Tag("date", "year")); m != "" && year == 0 {
		year, _ = strconv.Atoi(m)
	}
	b.Narrator = info.Tag("narrator", "narratedby", "composer", "performer")
	b.Overview = info.Tag("description", "synopsis", "comment", "desc")

	b.Book.Title, b.Book.Artist, b.Book.Year = title, author, year
	b.Book.SortName = strings.Join([]string{SortName(author), SortName(b.Series), indexKey(b.SeriesIndex), yearKey(year), SortName(title)}, " ")

	if whole {
		b.Part, b.PartName = 1, title
	} else {
		b.Part = leadingInt(info.Tag("track"))
		_, n, name := splitTrackFileName(base, author)
		if b.Part == 0 {
			b.Part = n
		}
		b.PartName = info.Tag("title")
		if b.PartName == "" {
			b.PartName = name
		}
	}
	return b, title != ""
}

// indexKey sorts series books by number; books without one go after.
func indexKey(n float64) string {
	if n <= 0 {
		return "9999"
	}
	return strconv.FormatFloat(10000+n, 'f', 2, 64)
}
