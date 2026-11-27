package scan

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/catalog"
)

var reTrailingParen = regexp.MustCompile(`\s*\(([^()]*)\)\s*$`)

// ParseMusicVideo reads artist, song and edition from a music video's path:
// "Artist - Song (Edition).mp4", or "Artist/Song.mp4" when the file sits in
// an artist folder. Trailing brackets that name an edition ("(MTV Unplugged)",
// "(Official Video)", "[Live]") or a year are taken off the song title; other
// brackets, such as "(feat. Someone)", stay part of it.
func ParseMusicVideo(root, path string, types []catalog.EditionType, defaultEdition string) ParsedName {
	p := ParsedName{EditionTypeID: defaultEdition, EditionSource: EditionDefault}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	text, explicit, _ := extractTags(base)
	text, brackets := extractBrackets(text)
	text = strings.TrimSpace(reSpaces.ReplaceAllString(text, " "))

	setEdition := func(id, source string) {
		if p.EditionSource == EditionDefault {
			p.EditionTypeID, p.EditionSource = id, source
		}
	}
	if explicit != "" {
		if id, ok := matchAlias(explicit, types); ok {
			setEdition(id, EditionExplicit)
		} else {
			p.UnmatchedLabel = explicit
		}
	}

	artist, song := "", text
	if i := strings.Index(text, " - "); i > 0 {
		artist, song = strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+3:])
	} else if dir := filepath.Dir(path); filepath.Clean(dir) != filepath.Clean(root) {
		artist = filepath.Base(dir)
	}

	rest, id, year := trimEditionSuffixes(song, types)
	song = rest
	if id != "" {
		setEdition(id, EditionAlias)
	}
	if year != 0 {
		p.Year = year
	}
	for _, b := range brackets {
		if id, ok := matchAlias(b, types); ok {
			setEdition(id, EditionAlias)
		}
	}

	p.Title, p.Artist = song, artist
	p.SortName = strings.TrimSpace(SortName(artist) + " " + SortName(song))
	return p
}

// trimEditionSuffixes takes edition names and a year off the end of a song or
// album title: " - Live", "(MTV Unplugged)", "(Deluxe Edition)", "(2002)".
// It returns the rest, the first edition found, and the year.
func trimEditionSuffixes(s string, types []catalog.EditionType) (rest, edition string, year int) {
	for {
		i := strings.LastIndex(s, " - ")
		if i <= 0 {
			break
		}
		id, ok := matchAlias(s[i+3:], types)
		if !ok {
			break
		}
		if edition == "" {
			edition = id
		}
		s = strings.TrimSpace(s[:i])
	}
	for {
		m := reTrailingParen.FindStringSubmatchIndex(s)
		if m == nil || m[0] == 0 {
			break
		}
		inner := strings.TrimSpace(s[m[2]:m[3]])
		if id, ok := matchAlias(inner, types); ok {
			if edition == "" {
				edition = id
			}
		} else if y, err := strconv.Atoi(inner); err == nil && y >= 1900 && y <= 2099 {
			year = y
		} else {
			break
		}
		s = strings.TrimSpace(s[:m[0]])
	}
	return s, edition, year
}
