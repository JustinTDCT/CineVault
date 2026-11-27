package scan

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ParsedEpisode is what a TV file's path says about its show and episode.
type ParsedEpisode struct {
	Show     ParsedName // Title, SortName, Year and TMDbID of the show
	Season   int
	Episode  int
	Last     int    // last episode in a multi-episode file (S01E01-E02); 0 for one episode
	Name     string // episode title from the file name, "" when there's none
	FromPath string // the part of the path the show name came from, for errors
}

var (
	// S01E02, s1e2, S01.E02, S01E01E02, S01E01-E02, S01E01-02
	reSxxEyy = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,3})[ ._-]?e(\d{1,4})((?:-?e\d{1,4}|-\d{1,4}(?:[^\dp]|$))*)`)
	// 1x02
	reNxNN       = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d{1,2})x(\d{2,3})(?:[^\dp]|$)`)
	reMoreEps    = regexp.MustCompile(`(?i)-?e?(\d{1,4})`)
	reSeasonDir  = regexp.MustCompile(`(?i)^(?:season|series|staffel|saison|temporada)[ ._-]*(\d{1,3})$|^s(\d{1,3})$`)
	reSpecialDir = regexp.MustCompile(`(?i)^(?:specials?|extras?)$`)
	// Release-name words that end an episode title in scene-style names.
	reQuality = regexp.MustCompile(`(?i)\b(?:2160p|1080p|720p|576p|480p|4k|uhd|web[ -]?dl|webrip|web|bluray|blu[ -]ray|bdrip|brrip|hdtv|dvdrip|remux|x264|x265|h ?264|h ?265|hevc|avc|hdr|dv|proper|repack|internal)\b`)
)

// ParseEpisode reads show, season and episode from a TV file's path. root is
// the library folder the file was found in. The show is the first folder under
// root ("3 Body Problem/Season 1/…"); for a file directly in root it's the
// part of the file name before the episode number. ok is false when the name
// has no season and episode number.
func ParseEpisode(root, path string) (ParsedEpisode, bool) {
	var p ParsedEpisode
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	text, _, _ := extractTags(base)
	text, _ = extractBrackets(text)
	text = normalizeSeparators(text)

	before, after, ok := findEpisode(text, &p)
	if !ok {
		return p, false
	}

	showText := before
	if rel, err := filepath.Rel(root, path); err == nil {
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 1 && !reSeasonDir.MatchString(parts[0]) && !reSpecialDir.MatchString(parts[0]) {
			showText = parts[0]
		}
		// A "Specials" folder marks season 0 even if the file says otherwise.
		if len(parts) > 2 && reSpecialDir.MatchString(parts[len(parts)-2]) {
			p.Season = 0
		}
	}
	p.FromPath = showText

	rest, _, tmdb := extractTags(showText)
	rest, _ = extractBrackets(rest)
	if !strings.Contains(strings.TrimSpace(rest), " ") {
		rest = strings.ReplaceAll(rest, ".", " ") // "The.Expanse"
	}
	rest = strings.Trim(normalizeSeparators(rest), " -–—")
	title, year, _ := splitYear(rest)
	p.Show = ParsedName{Title: strings.Trim(title, " -–—"), Year: year, TMDbID: tmdb}
	p.Show.SortName = SortName(p.Show.Title)
	p.Name = episodeTitle(after)
	return p, p.Show.Title != ""
}

// findEpisode locates the episode number in text and returns what comes
// before and after it.
func findEpisode(text string, p *ParsedEpisode) (before, after string, ok bool) {
	if m := reSxxEyy.FindStringSubmatchIndex(text); m != nil {
		p.Season, _ = strconv.Atoi(text[m[2]:m[3]])
		p.Episode, _ = strconv.Atoi(text[m[4]:m[5]])
		end := m[5]
		if m[6] < m[7] {
			for _, more := range reMoreEps.FindAllStringSubmatch(text[m[6]:m[7]], -1) {
				if n, _ := strconv.Atoi(more[1]); n > p.Episode && n > p.Last && n-p.Episode < 50 {
					p.Last = n
				}
			}
			end = m[7]
			// The pattern may have taken the character after the range.
			if end > 0 && end <= len(text) && !isDigit(text[end-1]) {
				end--
			}
		}
		return text[:m[0]], text[end:], true
	}
	if m := reNxNN.FindStringSubmatchIndex(text); m != nil {
		p.Season, _ = strconv.Atoi(text[m[2]:m[3]])
		p.Episode, _ = strconv.Atoi(text[m[4]:m[5]])
		return text[:m[0]], text[m[5]:], true
	}
	return "", "", false
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// episodeTitle cleans up the text after the episode number:
// " - Countdown" becomes "Countdown"; "Countdown 1080p WEB" becomes "Countdown".
func episodeTitle(s string) string {
	if loc := reQuality.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	s = strings.Trim(strings.TrimSpace(s), "-–—.:_ ")
	if _, y, _ := splitYear(s); y != 0 && strings.TrimSpace(strings.Trim(s, "()")) == strconv.Itoa(y) {
		return ""
	}
	return s
}
