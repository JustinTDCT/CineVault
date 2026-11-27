package scan

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/comic"
)

// ParsedComic is what a comic file's ComicInfo.xml and path say about it.
type ParsedComic struct {
	// Series is the series as a title (Title, Year from a "(1988)" folder).
	Series   ParsedName
	Number   float64 // issue number; 0 when unknown
	Issue    string  // the issue's name, or "Issue 2"
	Overview string
}

var (
	// The issue number at the end of a name: "- 2", "#002", "Issue 12", " 3", "v2 05".
	reIssueNumber = regexp.MustCompile(`(?i)(?:^|[\s#_-])(?:issue\s*|no\.?\s*|#)?(\d{1,4}(?:\.\d{1,2})?)$`)
	reParenGroups = regexp.MustCompile(`\s*\([^()]*\)`)
)

// ParseComic works out a comic's series and issue: ComicInfo.xml first, then
// the folder (the series) and the file name ("Aliens - Book One - 2.cbr").
func ParseComic(root, path string, info *comic.Info) ParsedComic {
	var c ParsedComic
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name, _ := extractBrackets(base)
	// "(1988)", "(Digital)", "(Scanner)" after the number.
	nameYear := 0
	if m := reYearParen.FindStringSubmatch(name); m != nil {
		nameYear, _ = strconv.Atoi(m[1])
	}
	name = strings.TrimSpace(reParenGroups.ReplaceAllString(name, ""))
	name = strings.TrimSpace(reSpaces.ReplaceAllString(strings.ReplaceAll(name, "_", " "), " "))

	fileSeries := name
	if m := reIssueNumber.FindStringSubmatchIndex(name); m != nil {
		c.Number, _ = strconv.ParseFloat(name[m[2]:m[3]], 64)
		fileSeries = strings.TrimRight(strings.TrimSpace(name[:m[0]]), " -#")
	}

	series, year := "", 0
	if rel, err := filepath.Rel(root, filepath.Dir(path)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		dir := filepath.Base(rel)
		dir, _ = extractBrackets(dir)
		if m := reYearParen.FindStringSubmatch(dir); m != nil {
			year, _ = strconv.Atoi(m[1])
		}
		series = strings.TrimSpace(reParenGroups.ReplaceAllString(dir, ""))
	}
	if series == "" {
		series = fileSeries
		year = nameYear
	}
	if info != nil {
		if s := strings.TrimSpace(info.Series); s != "" && !strings.EqualFold(s, series) && filepath.Dir(path) == filepath.Clean(root) {
			series = s // loose files: trust the tag; files in a folder keep the folder, so one tag can't split it
		}
		if n, err := strconv.ParseFloat(strings.TrimSpace(info.Number), 64); err == nil {
			c.Number = n
		}
		c.Issue = strings.TrimSpace(info.Title)
		c.Overview = strings.TrimSpace(info.Summary)
	}
	if series == "" {
		series = "Unknown series"
	}
	if c.Issue == "" {
		if c.Number > 0 {
			c.Issue = "Issue " + strconv.FormatFloat(c.Number, 'f', -1, 64)
		} else {
			c.Issue = name
		}
	}
	c.Series = ParsedName{Title: series, Year: year, EditionSource: EditionDefault}
	c.Series.SortName = SortName(series) + " " + yearKey(year)
	return c
}

// issueTrack turns an issue number into a track number: issue 2 is 200 and
// issue 2.5 is 250, so half issues sort between whole ones.
func issueTrack(n float64) int {
	return int(n*100 + 0.5)
}
