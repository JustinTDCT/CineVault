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

	// "Song - Live" and other " - Edition" suffixes.
	for {
		i := strings.LastIndex(song, " - ")
		if i <= 0 {
			break
		}
		id, ok := matchAlias(song[i+3:], types)
		if !ok {
			break
		}
		setEdition(id, EditionAlias)
		song = strings.TrimSpace(song[:i])
	}
	// Trailing "(...)" groups.
	for {
		m := reTrailingParen.FindStringSubmatchIndex(song)
		if m == nil || m[0] == 0 {
			break
		}
		inner := strings.TrimSpace(song[m[2]:m[3]])
		if id, ok := matchAlias(inner, types); ok {
			setEdition(id, EditionAlias)
		} else if y, err := strconv.Atoi(inner); err == nil && y >= 1900 && y <= 2099 {
			p.Year = y
		} else {
			break
		}
		song = strings.TrimSpace(song[:m[0]])
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
