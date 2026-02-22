package scanner

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Parsed Result ────────────────────

// ParsedFilename holds all metadata extracted from a media filename.
type ParsedFilename struct {
	Title       string
	Year        *int
	Edition     string // defaults to "Theatrical" if none found
	Resolution  string // e.g. "1080p", "720p", "2160p"
	Container   string // e.g. "mkv", "mp4"
	Artist      string // for music and music videos
	Album       string // for music
	Song        string // for music videos
	DiscNumber  *int   // for music (DxxTxx)
	TrackNumber *int   // for music (DxxTxx)
	Season      int    // for TV shows
	Episode     int    // for TV shows
	EpisodeEnd  int    // for multi-episode files (S01E01-E03 → EpisodeEnd=3)
	PartNumber  *int   // for multi-part movies (CD-x, DISC-x, PART-x)
	PartType    string // "CD", "DISC", or "PART"
	BaseTitle   string // full base name without part indicator (for multi-part grouping)
	ExtraType   string // "trailer", "sample", "featurette", etc. Empty = not an extra
	Source      string // "bluray", "dvd", "web", "hdtv", etc.
	IMDBID      string // from NFO sidecar file (tt1234567)
	TMDBID      string // from NFO or inline filename [tmdbid-12345]
	TVDBID      string // from NFO or inline filename [tvdbid-12345]
	ASIN        string // Audible ASIN for audiobooks [asin-B08G9PRS1K]
}

// multiPartEntry tracks a media item that is part of a multi-part set.
type multiPartEntry struct {
	ItemID     uuid.UUID
	PartNumber int
}

// ──────────────────── Compiled Regex (init once) ────────────────────

// Year extraction: requires delimiters to avoid false matches on episode numbers.
// Matches: (2020) [2020] .2020. -2020- _2020_ ,2020+
// Negative lookahead prevents matching dates like 2020.01.15 or episode nums like 20200102
var yearRx = regexp.MustCompile(`(?:[\(\[\.\-_,\s])([12]\d{3})(?:[\)\]\.\-_,+\s]|$)`)
var yearInParensRx = regexp.MustCompile(`[\(\[]([12]\d{3})[\)\]]`)

// Edition: {edition-XXX} or {XXX} from Radarr/Sonarr convention
var editionBracePattern = regexp.MustCompile(`(?i)\{(?:edition-)?([^}]+)\}`)

// Multi-part indicator: CD-x, DISC-x, PART-x, PT-x (may be followed by year/resolution)
var multiPartPattern = regexp.MustCompile(`(?i)[\s._-]+(CD|DISC|DISK|PART|PT)[\s._-]*(\d+)`)

// Adult prefix
var adultPrefixRx = regexp.MustCompile(`(?i)^XXX\s*[-–]\s*`)

// (Episode patterns are inlined in extractEpisodeInfo for cleaner match-position tracking)

// CineVault-specific strict patterns (tried first for well-named files)
var cvMoviePattern = regexp.MustCompile(
	`(?i)^(.+?)\s*\((\d{4})\)\s*(?:\{([^}]+)\})?\s*(?:\[([^/\]]+)/([^\]]+)\])?\s*$`)
var cvAdultMoviePattern = regexp.MustCompile(
	`(?i)^XXX\s+-\s+(.+?)\s*\((\d{4})\)\s*(?:\{([^}]+)\})?\s*(?:\[([^/\]]+)/([^\]]+)\])?\s*$`)
var cvTVPattern = regexp.MustCompile(
	`(?i)^(.+?)\s+-\s+S(\d{1,3})E(\d{1,3})\s*$`)
var cvMusicPattern = regexp.MustCompile(
	`(?i)^(.+?)\s+-\s+(.+?)\s+-\s+D(\d{1,3})T(\d{1,3})\s*$`)
var cvMusicVideoPattern = regexp.MustCompile(
	`^(.+?)\s+-\s+(.+)$`)

// ──────────────────── Token-Based Garbage Detection ────────────────────
// Inspired by Plex's VideoFiles.CleanName approach

// garbageTokens is the comprehensive set of junk tokens to identify in filenames.
// Checked case-insensitively. Organized by category for maintainability.
var garbageTokens = buildGarbageSet(
	// Video codecs
	[]string{"x264", "x265", "h264", "h265", "h.264", "h.265", "hevc", "avc", "divx", "xvid", "divx5", "mpeg4", "vp9", "av1", "10bit", "8bit", "hi10p", "hi10"},
	// Audio codecs & channels
	[]string{"aac", "ac3", "ac-3", "dts", "dts-hd", "dtshd", "dts-x", "truehd", "atmos", "flac", "mp3", "ogg", "vorbis", "opus", "eac3", "dd5.1", "dd2.0", "5.1", "7.1", "2.0", "5.1ch", "7.1ch"},
	// Resolution
	[]string{"480p", "480i", "576p", "576i", "720p", "720i", "1080p", "1080i", "2160p", "4k", "uhd", "ultrahd", "hd", "sd"},
	// Source
	[]string{"bluray", "blu-ray", "bdrip", "brrip", "bdrc", "bdremux", "hdrip", "hddvd", "hddvdrip",
		"dvd", "dvdrip", "dvdscr", "dvdscreener", "r1", "r3", "r5",
		"webrip", "web-dl", "webdl", "web",
		"hdtv", "pdtv", "dsr", "dsrip", "stv", "tvrip",
		"cam", "screener", "scr", "tc", "telecine", "ts", "telesync",
		"ppv", "retail"},
	// Release type / misc
	[]string{"remux", "proper", "repack", "rerip", "internal", "limited", "custom",
		"extended", "unrated", "theatrical", "remastered", "special.edition",
		"read.nfo", "readnfo", "nfofix", "nfo",
		"multi", "multisubs", "dubbed", "subbed", "subs", "sub",
		"cd1", "cd2", "cd3", "cd4", "1cd", "2cd", "3cd", "4cd",
		"ws", "fs", "fragment", "xxx"},
	// Container formats (when appearing as tokens, not as file extensions)
	[]string{"mkv", "mp4", "avi"},
)

// sourceTokens maps source labels to their identifying tokens (Plex-style)
var sourceTokenMap = map[string][]string{
	"bluray":   {"bluray", "blu-ray", "bdrc", "bdrip", "brrip", "hdrip", "hddvd", "hddvdrip", "bdremux", "remux"},
	"dvd":      {"dvd", "dvdrip", "r1", "r3", "r5"},
	"web":      {"webrip", "web-dl", "webdl", "web"},
	"hdtv":     {"hdtv", "pdtv", "dsr", "dsrip"},
	"cam":      {"cam"},
	"screener": {"dvdscr", "dvdscreener", "screener", "scr"},
	"telecine": {"tc", "telecine"},
	"telesync": {"ts", "telesync"},
}

// ──────────────────── Extras Detection ────────────────────
// Inspired by Jellyfin ExtraRuleResolver + Plex ignore patterns

// Directories whose contents should be classified as extras (not main library content)
var extrasDirPatterns = []struct {
	rx        *regexp.Regexp
	extraType string
}{
	{regexp.MustCompile(`(?i)^trailers?$`), "trailer"},
	{regexp.MustCompile(`(?i)^samples?$`), "sample"},
	{regexp.MustCompile(`(?i)^extras?$`), "extra"},
	{regexp.MustCompile(`(?i)^bonus$`), "extra"},
	{regexp.MustCompile(`(?i)^deleted[\s._-]?scenes?$`), "deleted_scene"},
	{regexp.MustCompile(`(?i)^behind[\s._-]?the[\s._-]?scenes?$`), "behind_the_scenes"},
	{regexp.MustCompile(`(?i)^interviews?$`), "interview"},
	{regexp.MustCompile(`(?i)^featurettes?$`), "featurette"},
	{regexp.MustCompile(`(?i)^scenes?$`), "scene"},
	{regexp.MustCompile(`(?i)^shorts?$`), "short"},
	{regexp.MustCompile(`(?i)^clips?$`), "clip"},
	{regexp.MustCompile(`(?i)^other$`), "extra"},
	{regexp.MustCompile(`(?i)^backdrops?$`), "theme_video"},
	{regexp.MustCompile(`(?i)^theme[\s._-]?music$`), "theme_song"},
	{regexp.MustCompile(`(?i)^special[\s._-]?features?$`), "extra"},
}

// Filename suffixes that indicate extras
var extrasSuffixPatterns = []struct {
	rx        *regexp.Regexp
	extraType string
}{
	{regexp.MustCompile(`(?i)[\s._-]trailer$`), "trailer"},
	{regexp.MustCompile(`(?i)[\s._-]sample$`), "sample"},
	{regexp.MustCompile(`(?i)[\s._-]behindthescenes$`), "behind_the_scenes"},
	{regexp.MustCompile(`(?i)[\s._-]deleted(?:scene)?$`), "deleted_scene"},
	{regexp.MustCompile(`(?i)[\s._-]featurette$`), "featurette"},
	{regexp.MustCompile(`(?i)[\s._-]short$`), "short"},
	{regexp.MustCompile(`(?i)[\s._-]interview$`), "interview"},
	{regexp.MustCompile(`(?i)[\s._-]scene$`), "scene"},
	{regexp.MustCompile(`(?i)[\s._-]clip$`), "clip"},
	{regexp.MustCompile(`(?i)[\s._-]extra$`), "extra"},
	{regexp.MustCompile(`(?i)[\s._-]other$`), "extra"},
}

// Sample file detection: pattern + size threshold
var sampleFileRx = regexp.MustCompile(`(?i)[\s._-]sample[\s._-]|^sample[\s._-]|[\s._-]sample$`)

// SampleFileSizeThreshold is the max file size (300MB) for a sample to be ignored
const SampleFileSizeThreshold = 300 * 1024 * 1024

// ──────────────────── Inline Provider ID Support ────────────────────
// Jellyfin-style: [tmdbid-12345], [imdbid-tt1234567], [tvdbid-12345]
// Plex-style:     {tmdb-12345},   {imdb-tt1234567},   {tvdb-12345}

var inlineTMDBIDPattern = regexp.MustCompile(`(?i)\[tmdbid[=-](\d+)\]`)
var inlineIMDBIDPattern = regexp.MustCompile(`(?i)\[imdbid[=-](tt\d+)\]`)
var inlineTVDBIDPattern = regexp.MustCompile(`(?i)\[tvdbid[=-](\d+)\]`)

var plexTMDBIDPattern = regexp.MustCompile(`(?i)\{tmdb[=-](\d+)\}`)
var plexIMDBIDPattern = regexp.MustCompile(`(?i)\{imdb[=-](tt\d+)\}`)
var plexTVDBIDPattern = regexp.MustCompile(`(?i)\{tvdb[=-](\d+)\}`)

var inlineASINPattern = regexp.MustCompile(`(?i)[\[{]asin[=-]([A-Z0-9]{10})[\]}]`)

func extractInlineProviderIDs(filename string, result *ParsedFilename) {
	// Jellyfin-style [tmdbid-X]
	if m := inlineTMDBIDPattern.FindStringSubmatch(filename); len(m) >= 2 {
		result.TMDBID = m[1]
		log.Printf("Inline ID: found TMDB ID %s in %q", m[1], filename)
	}
	if m := inlineIMDBIDPattern.FindStringSubmatch(filename); len(m) >= 2 {
		result.IMDBID = m[1]
		log.Printf("Inline ID: found IMDB ID %s in %q", m[1], filename)
	}
	if m := inlineTVDBIDPattern.FindStringSubmatch(filename); len(m) >= 2 {
		result.TVDBID = m[1]
		log.Printf("Inline ID: found TVDB ID %s in %q", m[1], filename)
	}
	// Plex-style {tmdb-X}
	if result.TMDBID == "" {
		if m := plexTMDBIDPattern.FindStringSubmatch(filename); len(m) >= 2 {
			result.TMDBID = m[1]
			log.Printf("Inline ID: found Plex-style TMDB ID %s in %q", m[1], filename)
		}
	}
	if result.IMDBID == "" {
		if m := plexIMDBIDPattern.FindStringSubmatch(filename); len(m) >= 2 {
			result.IMDBID = m[1]
			log.Printf("Inline ID: found Plex-style IMDB ID %s in %q", m[1], filename)
		}
	}
	if result.TVDBID == "" {
		if m := plexTVDBIDPattern.FindStringSubmatch(filename); len(m) >= 2 {
			result.TVDBID = m[1]
			log.Printf("Inline ID: found Plex-style TVDB ID %s in %q", m[1], filename)
		}
	}
	// ASIN (Audible) — [asin-B08G9PRS1K] or {asin-B08G9PRS1K}
	if result.ASIN == "" {
		if m := inlineASINPattern.FindStringSubmatch(filename); len(m) >= 2 {
			result.ASIN = m[1]
			log.Printf("Inline ID: found ASIN %s in %q", m[1], filename)
		}
	}
}

// ──────────────────── NFO Sidecar Support ────────────────────

var nfoIMDBPattern = regexp.MustCompile(`(tt\d{7,})`)

// ──────────────────── Helper Types ────────────────────


// ──────────────────── Main Parser ────────────────────

// parseFilename extracts metadata from a media filename based on the library's media type.
// Uses a hybrid approach: tries CineVault strict patterns first, then falls back to
// Plex/Jellyfin-style token-based cleaning for maximum flexibility.
func (s *Scanner) parseFilename(filename string, mediaType models.MediaType) ParsedFilename {
	result := ParsedFilename{
		Edition: "Theatrical",
	}

	// Strip file extension
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)
	result.Container = strings.ToLower(strings.TrimPrefix(ext, "."))

	// Step 0a: Extract inline provider IDs before stripping brackets
	// Supports both Jellyfin [tmdbid-X] and Plex {tmdb-X} formats
	extractInlineProviderIDs(baseName, &result)
	baseName = inlineTMDBIDPattern.ReplaceAllString(baseName, "")
	baseName = inlineIMDBIDPattern.ReplaceAllString(baseName, "")
	baseName = inlineTVDBIDPattern.ReplaceAllString(baseName, "")
	baseName = plexTMDBIDPattern.ReplaceAllString(baseName, "")
	baseName = plexIMDBIDPattern.ReplaceAllString(baseName, "")
	baseName = plexTVDBIDPattern.ReplaceAllString(baseName, "")
	baseName = inlineASINPattern.ReplaceAllString(baseName, "")
	baseName = strings.TrimSpace(baseName)

	// Step 0b: Check for extras by filename suffix (before any other parsing)
	result.ExtraType = detectExtraFromFilename(baseName)

	// Step 1: For movies/adult movies, check for multi-part indicator and strip it.
	// Use last match to avoid false positives on titles containing "Part" mid-string.
	// Everything from the match position onward (year, resolution suffixes) is stripped
	// since the media-type parser will extract year from the remaining base title.
	if mediaType == models.MediaTypeMovies || mediaType == models.MediaTypeAdultMovies {
		allLocs := multiPartPattern.FindAllStringSubmatchIndex(baseName, -1)
		if len(allLocs) > 0 {
			last := allLocs[len(allLocs)-1]
			partStr := baseName[last[4]:last[5]]
			partNum, _ := strconv.Atoi(partStr)
			result.PartNumber = &partNum
			result.PartType = strings.ToUpper(baseName[last[2]:last[3]])
			baseName = strings.TrimSpace(baseName[:last[0]])
		}
	}

	// Step 2: Try CineVault strict patterns first (well-named files),
	//         then fall back to universal token-based cleaning
	switch mediaType {
	case models.MediaTypeAdultMovies:
		result = s.parseAdultMovieFilename(baseName, result)
	case models.MediaTypeMovies:
		result = s.parseMovieFilename(baseName, result)
	case models.MediaTypeTVShows:
		result = s.parseTVShowFilename(baseName, result)
	case models.MediaTypeMusic:
		result = s.parseMusicFilename(baseName, result)
	case models.MediaTypeMusicVideos:
		result = s.parseMusicVideoFilename(baseName, result)
	default:
		// For other types, use universal cleaning
		cleaned := cleanFilenameUniversal(baseName)
		result.Title = cleaned.title
		result.Year = cleaned.year
		result.Resolution = cleaned.resolution
		result.Source = cleaned.source
		result.Edition = extractEdition(baseName)
	}

	// Step 3: Set BaseTitle for multi-part grouping (use the parsed title)
	if result.PartNumber != nil {
		result.BaseTitle = result.Title
	}

	return result
}

// ──────────────────── Type-Specific Parsers ────────────────────

// parseMovieFilename tries CineVault strict pattern first, then universal cleaning.
func (s *Scanner) parseMovieFilename(baseName string, result ParsedFilename) ParsedFilename {
	// Try CineVault strict: name (year) {edition} [resolution/container]
	if matches := cvMoviePattern.FindStringSubmatch(baseName); len(matches) >= 3 {
		result.Title = strings.TrimSpace(matches[1])
		year, err := strconv.Atoi(matches[2])
		if err == nil && year >= 1900 && year <= 2100 {
			result.Year = &year
		}
		if len(matches) >= 4 && matches[3] != "" {
			result.Edition = cleanEditionName(matches[3])
		}
		if len(matches) >= 5 && matches[4] != "" {
			result.Resolution = strings.TrimSpace(matches[4])
		}
		if len(matches) >= 6 && matches[5] != "" {
			result.Container = strings.ToLower(strings.TrimSpace(matches[5]))
		}
		log.Printf("Filename parse (movie/strict): %q → title=%q year=%v edition=%q",
			baseName, result.Title, result.Year, result.Edition)
		return result
	}

	// Fallback: universal token-based cleaning (handles scene releases, Radarr, etc.)
	cleaned := cleanFilenameUniversal(baseName)
	result.Title = cleaned.title
	result.Year = cleaned.year
	result.Resolution = cleaned.resolution
	result.Source = cleaned.source
	result.Edition = extractEdition(baseName)

	log.Printf("Filename parse (movie/universal): %q → title=%q year=%v res=%q source=%q",
		baseName, result.Title, result.Year, result.Resolution, result.Source)
	return result
}

// parseAdultMovieFilename tries CineVault strict pattern first, then universal cleaning.
func (s *Scanner) parseAdultMovieFilename(baseName string, result ParsedFilename) ParsedFilename {
	// Try CineVault strict: XXX - name (year) {edition} [resolution/container]
	if matches := cvAdultMoviePattern.FindStringSubmatch(baseName); len(matches) >= 3 {
		result.Title = strings.TrimSpace(matches[1])
		year, err := strconv.Atoi(matches[2])
		if err == nil && year >= 1900 && year <= 2100 {
			result.Year = &year
		}
		if len(matches) >= 4 && matches[3] != "" {
			result.Edition = cleanEditionName(matches[3])
		}
		if len(matches) >= 5 && matches[4] != "" {
			result.Resolution = strings.TrimSpace(matches[4])
		}
		if len(matches) >= 6 && matches[5] != "" {
			result.Container = strings.ToLower(strings.TrimSpace(matches[5]))
		}
		log.Printf("Filename parse (adult/strict): %q → title=%q year=%v",
			baseName, result.Title, result.Year)
		return result
	}

	// Fallback: strip XXX prefix, then universal clean
	stripped := adultPrefixRx.ReplaceAllString(baseName, "")
	cleaned := cleanFilenameUniversal(stripped)
	result.Title = cleaned.title
	result.Year = cleaned.year
	result.Resolution = cleaned.resolution
	result.Source = cleaned.source
	result.Edition = extractEdition(baseName)

	log.Printf("Filename parse (adult/universal): %q → title=%q year=%v",
		baseName, result.Title, result.Year)
	return result
}

// parseTVShowFilename tries CineVault strict, then multi-pattern episode detection.
func (s *Scanner) parseTVShowFilename(baseName string, result ParsedFilename) ParsedFilename {
	// Try CineVault strict: name - SxxExx
	if matches := cvTVPattern.FindStringSubmatch(baseName); len(matches) >= 4 {
		result.Title = strings.TrimSpace(matches[1])
		result.Season, _ = strconv.Atoi(matches[2])
		result.Episode, _ = strconv.Atoi(matches[3])
		log.Printf("Filename parse (tv/strict): %q → title=%q S%02dE%02d",
			baseName, result.Title, result.Season, result.Episode)
		return result
	}

	// Fallback: multi-pattern episode detection (Jellyfin/Plex style)
	// Normalize delimiters for pattern matching
	normalized := baseName
	normalized = strings.ReplaceAll(normalized, "_", " ")

	season, episode, episodeEnd, matchPos := extractEpisodeInfo(normalized)
	if episode > 0 {
		result.Season = season
		result.Episode = episode
		result.EpisodeEnd = episodeEnd

		// Extract show name: everything before the episode indicator
		if matchPos > 0 {
			showPart := normalized[:matchPos]
			showPart = strings.ReplaceAll(showPart, ".", " ")
			showPart = strings.TrimRight(showPart, " -._")
			showPart = collapseSpaces(showPart)
			result.Title = strings.TrimSpace(showPart)
		}
		if result.Title == "" {
			// Couldn't extract name from before pattern, use folder-aware cleaning
			result.Title = cleanShowNameFromFilename(baseName)
		}
		// Clean junk from show title
		result.Title = stripYearSuffix(result.Title)

		log.Printf("Filename parse (tv/universal): %q → title=%q S%02dE%02d",
			baseName, result.Title, result.Season, result.Episode)
		return result
	}

	// Last resort: no episode pattern matched at all
	cleaned := cleanFilenameUniversal(baseName)
	result.Title = cleaned.title
	result.Year = cleaned.year
	log.Printf("Filename parse (tv/fallback): %q → title=%q year=%v",
		baseName, result.Title, result.Year)
	return result
}

// parseMusicFilename handles CineVault's strict music format.
func (s *Scanner) parseMusicFilename(baseName string, result ParsedFilename) ParsedFilename {
	if matches := cvMusicPattern.FindStringSubmatch(baseName); len(matches) >= 5 {
		result.Artist = strings.TrimSpace(matches[1])
		result.Album = strings.TrimSpace(matches[2])
		disc, _ := strconv.Atoi(matches[3])
		result.DiscNumber = &disc
		track, _ := strconv.Atoi(matches[4])
		result.TrackNumber = &track
		result.Title = result.Album
		log.Printf("Filename parse (music): %q → artist=%q album=%q D%02dT%02d",
			baseName, result.Artist, result.Album, disc, track)
	} else {
		cleaned := cleanFilenameUniversal(baseName)
		result.Title = cleaned.title
	}
	return result
}

// parseMusicVideoFilename handles CineVault's strict music video format.
func (s *Scanner) parseMusicVideoFilename(baseName string, result ParsedFilename) ParsedFilename {
	if matches := cvMusicVideoPattern.FindStringSubmatch(baseName); len(matches) >= 3 {
		result.Artist = strings.TrimSpace(matches[1])
		result.Song = strings.TrimSpace(matches[2])
		result.Title = result.Song
		log.Printf("Filename parse (music_video): %q → artist=%q song=%q",
			baseName, result.Artist, result.Song)
	} else {
		cleaned := cleanFilenameUniversal(baseName)
		result.Title = cleaned.title
	}
	return result
}

// ──────────────────── Universal Token-Based Cleaner ────────────────────
// Hybrid of Plex's garbage-bitmap approach and Jellyfin's multi-pass stripping.

type cleanResult struct {
	title      string
	year       *int
	resolution string
	source     string
}

// cleanFilenameUniversal parses any filename format by:
// 1. Extracting year (used as breakpoint)
// 2. Tokenizing on delimiters
// 3. Building a good/bad bitmap using garbage token set
// 4. Keeping good tokens, stopping after 2+ consecutive bad tokens
func cleanFilenameUniversal(baseName string) cleanResult {
	result := cleanResult{}
	name := baseName

	// --- Pass 1: Strip bracketed content [xxx] and {xxx} ---
	name = regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(name, " ")

	// --- Pass 2: Extract year ---
	// Try parens/brackets first: (2020) or [2020]
	if m := yearInParensRx.FindStringSubmatch(name); len(m) >= 2 {
		y, _ := strconv.Atoi(m[1])
		if y >= 1900 && y <= 2100 {
			result.year = &y
			// Use year as breakpoint: take everything before it
			idx := strings.Index(name, m[0])
			if idx > 0 {
				name = name[:idx]
			}
		}
	} else if m := yearRx.FindStringSubmatch(name); len(m) >= 2 {
		// Delimited year: .2020. -2020- etc.
		y, _ := strconv.Atoi(m[1])
		if y >= 1900 && y <= 2100 {
			result.year = &y
			// Use year as breakpoint
			idx := strings.Index(name, m[1])
			if idx > 0 {
				name = name[:idx]
			}
		}
	}

	// --- Pass 3: Strip edition phrases before tokenization ---
	// "Cut"/"Edition" are too common to remove as standalone tokens,
	// so strip full phrases like "Extended Cut", "Director's Cut", etc.
	editionPhraseRx := regexp.MustCompile(`(?i)[-–]?\s*\b(` +
		`director'?s?\s*cut|final\s+cut|extended\s+cut|theatrical\s+cut|unrated\s+cut|ultimate\s+cut|` +
		`criterion\s+edition|anniversary\s+edition|collector'?s?\s+edition|ultimate\s+edition|` +
		`deluxe\s+edition|imax\s+edition|special\s+edition|limited\s+edition|` +
		`extended\s+edition|unrated\s+edition|theatrical\s+edition|remastered\s+edition` +
		`)\b`)
	name = editionPhraseRx.ReplaceAllString(name, " ")

	// --- Pass 4: Normalize delimiters ---
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// --- Pass 4: Tokenize and build garbage bitmap ---
	tokens := tokenize(name)
	if len(tokens) == 0 {
		result.title = baseName
		return result
	}

	// Detect resolution and source from ALL tokens (before discarding)
	for _, t := range tokens {
		tl := strings.ToLower(t)
		if result.resolution == "" {
			if isResolution(tl) {
				result.resolution = tl
			}
		}
		if result.source == "" {
			result.source = detectSource(tl)
		}
	}

	// Build bitmap: true = good token, false = garbage
	bitmap := make([]bool, len(tokens))
	seen := make(map[string]bool)
	for i := len(tokens) - 1; i >= 0; i-- {
		tl := strings.ToLower(tokens[i])
		if garbageTokens[tl] && !seen[tl] {
			bitmap[i] = false
			seen[tl] = true
		} else {
			bitmap[i] = true
		}
	}

	// If only 1-2 tokens total, keep them all (might be the actual title like "XXX 2")
	if len(tokens) <= 2 {
		for i := range bitmap {
			bitmap[i] = true
		}
	}

	// Walk forward: collect good tokens, stop after 2+ consecutive bad
	var finalTokens []string
	consecutiveBad := 0
	for i, good := range bitmap {
		if good {
			finalTokens = append(finalTokens, tokens[i])
			consecutiveBad = 0
		} else {
			consecutiveBad++
			if consecutiveBad >= 2 {
				break
			}
		}
	}

	// Safety: if all tokens were stripped, use the first one
	if len(finalTokens) == 0 && len(tokens) > 0 {
		finalTokens = append(finalTokens, tokens[0])
	}

	result.title = strings.TrimSpace(strings.Join(finalTokens, " "))

	// Clean trailing dashes and parentheses
	result.title = strings.TrimRight(result.title, " -–")
	result.title = collapseSpaces(result.title)

	return result
}

// ──────────────────── Episode Info Extraction ────────────────────

// extractEpisodeInfo tries multiple patterns against the filename to find season/episode.
// Returns (season, episode, episodeEnd, matchPosition).
// matchPosition is the byte offset where the episode pattern starts (for name extraction).
func extractEpisodeInfo(filename string) (season, episode, episodeEnd, matchPos int) {
	// Pattern 1: S01E01 (most specific, try first)
	if m := regexp.MustCompile(`(?i)(?:^|[/\\._ -])S(\d{1,4})\s*E(\d{1,4})(?:\s*-?\s*E?(\d{1,4}))?`).FindStringSubmatchIndex(filename); m != nil {
		season, _ = strconv.Atoi(filename[m[2]:m[3]])
		episode, _ = strconv.Atoi(filename[m[4]:m[5]])
		if m[6] >= 0 && m[7] >= 0 {
			episodeEnd, _ = strconv.Atoi(filename[m[6]:m[7]])
		}
		matchPos = m[0]
		return
	}

	// Pattern 2: 1x01
	if m := regexp.MustCompile(`(?i)(?:^|[/\\._ -])(\d{1,2})[xX](\d{1,3})(?:\s*-\s*(\d{1,3}))?`).FindStringSubmatchIndex(filename); m != nil {
		season, _ = strconv.Atoi(filename[m[2]:m[3]])
		episode, _ = strconv.Atoi(filename[m[4]:m[5]])
		if m[6] >= 0 && m[7] >= 0 {
			episodeEnd, _ = strconv.Atoi(filename[m[6]:m[7]])
		}
		matchPos = m[0]
		return
	}

	// Pattern 3: Season X Episode Y (verbose)
	if m := regexp.MustCompile(`(?i)[Ss](?:eason)?\s*(\d{1,4})\s*[Ee](?:pisode)?\s*(\d{1,4})`).FindStringSubmatchIndex(filename); m != nil {
		season, _ = strconv.Atoi(filename[m[2]:m[3]])
		episode, _ = strconv.Atoi(filename[m[4]:m[5]])
		matchPos = m[0]
		return
	}

	// Pattern 4: Episode N (no season — assume season 1)
	if m := regexp.MustCompile(`(?i)(?:^|[\s._-])[Ee](?:pisode)?\s*(\d{1,4})`).FindStringSubmatchIndex(filename); m != nil {
		season = 1
		episode, _ = strconv.Atoi(filename[m[2]:m[3]])
		matchPos = m[0]
		return
	}

	// Pattern 5: Date-based yyyy.mm.dd (encode as season=0, episode=mmdd)
	if m := regexp.MustCompile(`((?:19|20)\d{2})[.\-/](\d{2})[.\-/](\d{2})`).FindStringSubmatchIndex(filename); m != nil {
		// For date-based shows, we'll store as season = year, episode = month*100+day
		// This allows the hierarchy handler to use the date for grouping
		yr, _ := strconv.Atoi(filename[m[2]:m[3]])
		mo, _ := strconv.Atoi(filename[m[4]:m[5]])
		dy, _ := strconv.Atoi(filename[m[6]:m[7]])
		season = yr
		episode = mo*100 + dy
		matchPos = m[0]
		return
	}

	return 0, 0, 0, -1
}

// ──────────────────── Extras Detection ────────────────────

// IsExtraFile checks if a file should be classified as an extra based on its full path.
// Returns the extra type string (e.g. "trailer", "sample") or empty string.
func IsExtraFile(fullPath string, fileSize int64) string {
	// Check parent directories
	dir := filepath.Dir(fullPath)
	parts := strings.Split(dir, string(filepath.Separator))
	for _, part := range parts {
		for _, dp := range extrasDirPatterns {
			if dp.rx.MatchString(part) {
				return dp.extraType
			}
		}
	}

	// Check filename
	base := strings.TrimSuffix(filepath.Base(fullPath), filepath.Ext(fullPath))

	// Sample files: must also be under size threshold
	if sampleFileRx.MatchString(base) && fileSize < SampleFileSizeThreshold {
		return "sample"
	}

	return detectExtraFromFilename(base)
}

// detectExtraFromFilename checks filename suffixes for extra indicators.
func detectExtraFromFilename(baseName string) string {
	for _, sp := range extrasSuffixPatterns {
		if sp.rx.MatchString(baseName) {
			return sp.extraType
		}
	}
	return ""
}

// ──────────────────── NFO Sidecar Support ────────────────────

// ReadNFOIMDBID looks for a .nfo sidecar file next to the media file
// and extracts an IMDB ID (tt1234567) if found.
func ReadNFOIMDBID(mediaFilePath string) string {
	dir := filepath.Dir(mediaFilePath)
	base := strings.TrimSuffix(filepath.Base(mediaFilePath), filepath.Ext(mediaFilePath))

	// Check for sidecar NFO: "MovieName.nfo"
	nfoPath := filepath.Join(dir, base+".nfo")
	if id := extractIMDBFromNFO(nfoPath); id != "" {
		return id
	}

	// Check for any .nfo file in the same directory (when it's the only video)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	videoCount := 0
	for _, e := range entries {
		if !e.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if videoExtensions[ext] {
				videoCount++
			}
		}
	}
	if videoCount == 1 {
		for _, e := range entries {
			if !e.IsDir() && strings.ToLower(filepath.Ext(e.Name())) == ".nfo" {
				if id := extractIMDBFromNFO(filepath.Join(dir, e.Name())); id != "" {
					return id
				}
			}
		}
	}

	return ""
}

func extractIMDBFromNFO(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if m := nfoIMDBPattern.FindSubmatch(data); len(m) >= 2 {
		return string(m[1])
	}
	return ""
}

// ──────────────────── Utility Functions ────────────────────

// buildGarbageSet creates a case-insensitive set of garbage tokens from multiple slices.
func buildGarbageSet(slices ...[]string) map[string]bool {
	set := make(map[string]bool)
	for _, sl := range slices {
		for _, s := range sl {
			set[strings.ToLower(s)] = true
		}
	}
	return set
}

// tokenize splits a string on whitespace and common delimiters, returning non-empty tokens.
func tokenize(s string) []string {
	// Split on spaces, and strip surrounding dashes (but keep them inside words like "Spider-Man")
	parts := strings.Fields(s)
	var tokens []string
	for _, p := range parts {
		p = strings.Trim(p, "-–()[]{}+,;")
		if p != "" {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// isResolution returns true if the token is a resolution indicator.
func isResolution(token string) bool {
	resolutions := map[string]bool{
		"480p": true, "480i": true, "576p": true, "576i": true,
		"720p": true, "720i": true, "1080p": true, "1080i": true,
		"2160p": true, "4k": true, "uhd": true, "ultrahd": true,
	}
	return resolutions[strings.ToLower(token)]
}

// detectSource returns the source type for a token, or empty string.
func detectSource(token string) string {
	tl := strings.ToLower(token)
	for source, keywords := range sourceTokenMap {
		for _, kw := range keywords {
			if tl == kw {
				return source
			}
		}
	}
	return ""
}

// collapseSpaces replaces multiple consecutive spaces with a single space.
func collapseSpaces(s string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

// stripYearSuffix removes a trailing "(2020)" or "[2020]" from a show name.
func stripYearSuffix(s string) string {
	return regexp.MustCompile(`\s*[\(\[]\d{4}[\)\]]\s*$`).ReplaceAllString(s, "")
}

// cleanShowNameFromFilename extracts a show name from a filename by stripping junk.
func cleanShowNameFromFilename(baseName string) string {
	name := baseName
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	// Remove episode indicators
	name = regexp.MustCompile(`(?i)\s*S\d+E\d+.*$`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`(?i)\s*\d{1,2}[xX]\d+.*$`).ReplaceAllString(name, "")
	name = strings.TrimRight(name, " -–._")
	return collapseSpaces(strings.TrimSpace(name))
}

// extractEdition checks for {edition-XXX} or {XXX} in the filename.
func extractEdition(baseName string) string {
	if m := editionBracePattern.FindStringSubmatch(baseName); len(m) >= 2 {
		return cleanEditionName(m[1])
	}
	return "Theatrical"
}

// cleanEditionName normalizes edition names, stripping the "edition-" prefix.
func cleanEditionName(raw string) string {
	name := strings.TrimSpace(raw)
	name = regexp.MustCompile(`(?i)^edition-`).ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if name == "" {
		return "Theatrical"
	}
	return name
}

// ──────────────────── Folder-First Title Resolution ────────────────────

// folderYearRx matches "Title (Year)" or "Title [Year]" with optional trailing info.
var folderYearRx = regexp.MustCompile(`[\(\[]([12]\d{3})[\)\]]`)

// ParseFolderName extracts a clean title and year from a parent folder name.
// Returns empty title if the folder doesn't follow a "Title (Year)" convention.
func ParseFolderName(folderName string) (title string, year *int) {
	if folderName == "" || folderName == "." || folderName == "/" {
		return "", nil
	}

	name := folderName

	// Strip bracketed content: [anything] and {anything}
	name = regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(name, " ")

	// Look for year in parentheses from original folder name
	if m := folderYearRx.FindStringSubmatch(folderName); len(m) >= 2 {
		y, _ := strconv.Atoi(m[1])
		if y >= 1900 && y <= 2100 {
			year = &y
			// Take everything before the year match in the cleaned name
			idx := strings.Index(folderName, m[0])
			if idx > 0 {
				name = folderName[:idx]
			}
		}
	}

	name = strings.TrimRight(name, " -–._")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	title = strings.TrimSpace(name)
	return title, year
}

// EditionFromFileVsFolder extracts an edition label by comparing a filename
// against the folder-derived title. The text between the title and the year
// in the filename becomes the edition.
//
// Example:
//
//	folder title = "3 Days to Kill"
//	filename     = "3 Days to Kill - Extended Cut (2014) [Bluray-1080p x265].mkv"
//	→ edition    = "Extended Cut"
func EditionFromFileVsFolder(filename, folderTitle string) string {
	// Check for {edition-XXX} braces first (Radarr/Plex convention)
	if m := editionBracePattern.FindStringSubmatch(filename); len(m) >= 2 {
		return cleanEditionName(m[1])
	}

	if folderTitle == "" {
		return "Theatrical"
	}

	// Strip extension
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	// Strip bracketed content [xxx] and {xxx}
	base = regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(base, " ")
	base = regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(base, " ")

	// Strip year in parens
	base = regexp.MustCompile(`\(\d{4}\)`).ReplaceAllString(base, " ")

	// Collapse whitespace and trim
	base = regexp.MustCompile(`\s+`).ReplaceAllString(base, " ")
	base = strings.TrimSpace(base)

	// Case-insensitive prefix removal: strip the folder title from the start
	if len(base) > len(folderTitle) && strings.EqualFold(base[:len(folderTitle)], folderTitle) {
		remainder := base[len(folderTitle):]
		remainder = strings.TrimLeft(remainder, " -–.")
		remainder = strings.TrimRight(remainder, " -–.")
		remainder = strings.TrimSpace(remainder)
		if remainder != "" {
			return remainder
		}
	}

	return "Theatrical"
}

// ──────────────────── Music Hierarchy ────────────────────

// handleMusicHierarchy finds or creates Artist and Album records from parsed filename data,
// and links them to the media item before it is persisted. Uses the in-memory cache
// to avoid redundant DB lookups for the same artist/album across tracks.
func (s *Scanner) handleMusicHierarchy(library *models.Library, item *models.MediaItem, parsed ParsedFilename) error {
	if parsed.Artist == "" || s.musicRepo == nil {
		return nil
	}

	artist, err := s.cachedFindOrCreateArtist(library.ID, parsed.Artist)
	if err != nil {
		return err
	}
	item.ArtistID = &artist.ID

	if parsed.Album != "" {
		album, err := s.cachedFindOrCreateAlbum(artist.ID, library.ID, parsed.Album, nil)
		if err != nil {
			return err
		}
		item.AlbumID = &album.ID
	}

	return nil
}

// ──────────────────── Multi-Part Grouping ────────────────────

// groupMultiPartFiles creates sister groups for multi-part files detected during the scan.
// Files sharing the same base title in the same directory are grouped together.
func (s *Scanner) groupMultiPartFiles() {
	if s.sisterRepo == nil {
		log.Printf("Multi-part: sister repository not available, skipping grouping")
		return
	}

	for key, parts := range s.pendingMultiParts {
		if len(parts) < 2 {
			continue
		}

		// Extract the base title from the grouping key (format: "dir|baseTitle")
		groupName := key
		if idx := strings.Index(key, "|"); idx >= 0 {
			groupName = key[idx+1:]
		}

		// Create a sister group for this multi-part set
		group := &models.SisterGroup{
			ID:   uuid.New(),
			Name: groupName,
		}
		if err := s.sisterRepo.Create(group); err != nil {
			log.Printf("Multi-part: failed to create sister group for %q: %v", groupName, err)
			continue
		}

		// Link all parts to the sister group and set sort_position = part number
		for _, part := range parts {
			if err := s.sisterRepo.AddMemberWithPosition(group.ID, part.ItemID, part.PartNumber); err != nil {
				log.Printf("Multi-part: failed to add part %d (item %s) to group: %v",
					part.PartNumber, part.ItemID, err)
			}
		}

		log.Printf("Multi-part: grouped %d parts as %q (sister group %s)",
			len(parts), groupName, group.ID)
	}
}
