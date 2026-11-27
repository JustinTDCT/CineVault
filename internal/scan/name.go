package scan

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/JustinTDCT/CineVault/internal/catalog"
)

// EditionSource says how a file's edition was decided.
const (
	EditionExplicit  = "explicit"  // {edition-…} tag matched an Edition List entry
	EditionAlias     = "alias"     // a keyword in the name matched an alias
	EditionDefault   = "default"   // nothing found; the media type's default edition
	EditionUnmatched = "unmatched" // an {edition-…} tag named something not in the list
)

// ParsedName is what a file's path says about the title it belongs to.
type ParsedName struct {
	Title          string
	SortName       string
	Year           int // 0 when unknown
	TMDbID         int // from a {tmdb-123} tag, 0 when absent
	EditionTypeID  string
	EditionSource  string
	UnmatchedLabel string // the {edition-…} text when it matched nothing
}

var (
	reTag       = regexp.MustCompile(`\{([A-Za-z]+)-([^}]*)\}`)
	reBracket   = regexp.MustCompile(`\[([^\]]*)\]`)
	reYearParen = regexp.MustCompile(`\(((?:19|20)\d{2})\)`)
	reYearBare  = regexp.MustCompile(`(?:^|[\s(])((?:19|20)\d{2})(?:$|[\s)])`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// ParseName works out title, year and edition from a media file's path.
// types are the Edition List entries for the library's media type, and
// defaultEdition is that media type's default (for example "theatrical").
//
// Edition rules, in order (see docs/specs/editions-and-versions.md §6a):
//  1. a Plex-style {edition-Name} tag, matched against names and aliases
//  2. an alias keyword after " - " in the title, after the year, or inside
//     [brackets], for example "Alien - Remastered (1979)"
//  3. otherwise the default edition
//
// Aliases are never searched for in the title itself, so a film called
// "Uncut Gems" isn't mistaken for an Uncut edition.
func ParseName(path string, types []catalog.EditionType, defaultEdition string) ParsedName {
	p := ParsedName{EditionTypeID: defaultEdition, EditionSource: EditionDefault}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	text, explicit, tmdb := extractTags(base)
	p.TMDbID = tmdb
	text, brackets := extractBrackets(text)
	text = normalizeSeparators(text)

	title, year, afterYear := splitYear(text)
	if year == 0 {
		// "Alien (1979)/Alien Director's Cut.mkv": take title and year from the folder.
		if ft, fy, _ := splitYear(normalizeSeparators(stripAll(filepath.Base(filepath.Dir(path))))); fy != 0 {
			afterYear = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(title), strings.ToLower(ft)))
			title, year = ft, fy
		}
	}
	p.Year = year

	// Candidate places for edition keywords.
	var candidates []string
	if parts := strings.Split(title, " - "); len(parts) > 1 {
		// Only treat the last " - " piece as an edition if it matches one,
		// so "Mission: Impossible - Fallout" keeps its full title.
		if _, ok := matchAlias(parts[len(parts)-1], types); ok {
			candidates = append(candidates, parts[len(parts)-1])
			title = strings.Join(parts[:len(parts)-1], " - ")
		}
	}
	candidates = append(candidates, afterYear)
	candidates = append(candidates, brackets...)

	switch {
	case explicit != "":
		if id, ok := matchAlias(explicit, types); ok {
			p.EditionTypeID, p.EditionSource = id, EditionExplicit
		} else {
			p.EditionSource, p.UnmatchedLabel = EditionUnmatched, explicit
		}
	default:
		best, bestLen := "", 0
		for _, c := range candidates {
			if id, n := longestAlias(c, types); n > bestLen {
				best, bestLen = id, n
			}
		}
		if best != "" {
			p.EditionTypeID, p.EditionSource = best, EditionAlias
		}
	}

	p.Title = strings.Trim(strings.TrimSpace(title), "-–— ")
	p.SortName = SortName(p.Title)
	return p
}

// SortName lowercases a title and drops a leading article.
func SortName(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	for _, a := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(s, a) && len(s) > len(a) {
			return s[len(a):]
		}
	}
	return s
}

func extractTags(s string) (rest, edition string, tmdb int) {
	for _, m := range reTag.FindAllStringSubmatch(s, -1) {
		switch strings.ToLower(m[1]) {
		case "edition":
			edition = strings.TrimSpace(m[2])
		case "tmdb", "tmdbid":
			tmdb, _ = strconv.Atoi(strings.TrimSpace(m[2]))
		}
	}
	return reTag.ReplaceAllString(s, " "), edition, tmdb
}

func extractBrackets(s string) (rest string, contents []string) {
	for _, m := range reBracket.FindAllStringSubmatch(s, -1) {
		contents = append(contents, m[1])
	}
	return reBracket.ReplaceAllString(s, " "), contents
}

func stripAll(s string) string {
	s, _, _ = extractTags(s)
	s, _ = extractBrackets(s)
	return s
}

// normalizeSeparators turns "Alien.1979.Directors.Cut" into "Alien 1979 Directors Cut".
func normalizeSeparators(s string) string {
	if !strings.Contains(s, " ") && strings.Count(s, ".") >= 2 {
		s = strings.ReplaceAll(s, ".", " ")
	}
	s = strings.ReplaceAll(s, "_", " ")
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

// splitYear finds the release year. A year in parentheses wins; otherwise the
// last bare 19xx/20xx that isn't the start of the name (so "1917 (2019)" and
// "2001 A Space Odyssey 1968" both work).
func splitYear(s string) (title string, year int, after string) {
	if locs := reYearParen.FindAllStringSubmatchIndex(s, -1); len(locs) > 0 {
		l := locs[len(locs)-1]
		year, _ = strconv.Atoi(s[l[2]:l[3]])
		return strings.TrimSpace(s[:l[0]]), year, strings.TrimSpace(s[l[1]:])
	}
	locs := reYearBare.FindAllStringSubmatchIndex(s, -1)
	for i := len(locs) - 1; i >= 0; i-- {
		l := locs[i]
		if l[2] == 0 {
			continue
		}
		year, _ = strconv.Atoi(s[l[2]:l[3]])
		return strings.TrimSpace(s[:l[2]]), year, strings.TrimSpace(s[l[3]:])
	}
	return s, 0, ""
}

// normalizeKey lowercases and reduces punctuation to single spaces, so
// "Director's Cut", "directors.cut" and "DIRECTORS CUT" compare equal.
func normalizeKey(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '\'' || r == '’':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		default:
			if !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func keysFor(e catalog.EditionType) []string {
	keys := []string{normalizeKey(e.DisplayName), normalizeKey(strings.ReplaceAll(e.ID, "_", " "))}
	for _, a := range e.Aliases {
		keys = append(keys, normalizeKey(a))
	}
	return keys
}

// matchAlias reports whether s as a whole names an edition.
func matchAlias(s string, types []catalog.EditionType) (string, bool) {
	k := normalizeKey(s)
	if k == "" {
		return "", false
	}
	for _, e := range types {
		for _, key := range keysFor(e) {
			if key == k {
				return e.ID, true
			}
		}
	}
	return "", false
}

// longestAlias finds the longest edition keyword appearing as whole words in s.
func longestAlias(s string, types []catalog.EditionType) (string, int) {
	hay := " " + normalizeKey(s) + " "
	best, bestLen := "", 0
	for _, e := range types {
		for _, key := range keysFor(e) {
			if key != "" && len(key) > bestLen && strings.Contains(hay, " "+key+" ") {
				best, bestLen = e.ID, len(key)
			}
		}
	}
	return best, bestLen
}
