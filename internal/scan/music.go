package scan

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
)

// AudioExtensions are the file types scanned in music libraries.
var AudioExtensions = map[string]bool{
	".flac": true, ".mp3": true, ".m4a": true, ".m4b": true, ".aac": true, ".ogg": true, ".oga": true,
	".opus": true, ".wav": true, ".wma": true, ".aiff": true, ".aif": true, ".alac": true, ".wv": true, ".ape": true,
}

// ParsedTrack is what a music file's tags and path say about it.
type ParsedTrack struct {
	// Album is the album as a title: Title is the album name, Artist the
	// album artist, plus Year and the edition (Deluxe, Remaster...).
	Album  ParsedName
	Disc   int
	Number int    // 0 when unknown
	Name   string // the song
	Artist string // the track's artist when it differs from the album artist
	Genre  string
}

var (
	reDiscDir    = regexp.MustCompile(`(?i)^(?:cd|disc|disk)[ ._-]*(\d{1,2})$`)
	reNumberPart = regexp.MustCompile(`^(?:(\d{1,2})-)?(\d{1,3})$`)
	reLeadNumber = regexp.MustCompile(`^(?:(\d{1,2})-)?(\d{1,3})(?:\s*[-.]\s*|\s+)(.+)$`)
	reYearPrefix = regexp.MustCompile(`^((?:19|20)\d{2})`)
)

// ParseTrack works out album, disc, track number and song from a music
// file. Tags come first; the folders (Artist/Album (Year)/[CD1/]) and the
// file name ("Artist - 03 - Song", "03 Song", "1-03 Song") fill the gaps.
func ParseTrack(root, path string, info mediainfo.Info, types []catalog.EditionType, defaultEdition string) (ParsedTrack, bool) {
	var t ParsedTrack
	t.Album = ParsedName{EditionTypeID: defaultEdition, EditionSource: EditionDefault}

	var dirs []string
	if rel, err := filepath.Rel(root, filepath.Dir(path)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		dirs = strings.Split(filepath.ToSlash(rel), "/")
	}
	if n := len(dirs); n > 0 {
		if m := reDiscDir.FindStringSubmatch(dirs[n-1]); m != nil {
			t.Disc, _ = strconv.Atoi(m[1])
			dirs = dirs[:n-1]
		}
	}
	albumDir, artistDir := "", ""
	if n := len(dirs); n > 0 {
		albumDir = dirs[n-1]
		if n > 1 {
			artistDir = dirs[n-2]
		}
	}

	// Album name, edition and year.
	album := info.Tag("album")
	if album == "" {
		album, _ = extractBrackets(albumDir)
	}
	album, _ = extractBrackets(album)
	album = strings.TrimSpace(reSpaces.ReplaceAllString(album, " "))
	album, edition, year := trimEditionSuffixes(album, types)
	if album == "" && albumDir != "" {
		album = albumDir
	}
	if album == "" {
		album = "Unknown album"
	}
	if edition != "" {
		t.Album.EditionTypeID, t.Album.EditionSource = edition, EditionAlias
	}
	// The folder's year and edition first: every song in the folder shares
	// them, so one song with a different (or missing) tag can't split the album.
	if albumDir != "" {
		dirName, _ := extractBrackets(albumDir)
		_, dirEdition, y := trimEditionSuffixes(strings.TrimSpace(dirName), types)
		if y != 0 {
			year = y
		}
		if dirEdition != "" {
			t.Album.EditionTypeID, t.Album.EditionSource = dirEdition, EditionAlias
		}
	}
	if year == 0 {
		if m := reYearPrefix.FindString(info.Tag("date", "year", "originaldate")); m != "" {
			year, _ = strconv.Atoi(m)
		}
	}
	t.Album.Title, t.Album.Year = album, year

	// Album artist: the tag, else the artist folder, else the track's artist.
	artist := info.Tag("album_artist")
	if artist == "" {
		artist = artistDir
	}
	if artist == "" {
		artist = info.Tag("artist")
	}
	if artist == "" {
		artist = "Unknown artist"
	}
	t.Album.Artist = artist
	t.Album.SortName = strings.TrimSpace(SortName(artist) + " " + yearKey(year) + " " + SortName(album))

	// Disc and track numbers.
	if d := leadingInt(info.Tag("disc")); d > 0 {
		t.Disc = d
	}
	t.Number = leadingInt(info.Tag("track"))

	// Song name, and the number from the file name if the tags had none.
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base, _ = extractBrackets(base)
	base = strings.TrimSpace(reSpaces.ReplaceAllString(base, " "))
	fileDisc, fileNum, fileName := splitTrackFileName(base, artist)
	if t.Number == 0 {
		t.Number = fileNum
	}
	if t.Disc == 0 {
		t.Disc = fileDisc
	}
	if t.Disc == 0 {
		t.Disc = 1
	}
	t.Name = info.Tag("title")
	if t.Name == "" {
		t.Name = fileName
	}
	if ta := info.Tag("artist"); ta != "" && !strings.EqualFold(ta, artist) {
		t.Artist = ta
	}
	t.Genre = info.Tag("genre")
	return t, t.Name != ""
}

// splitTrackFileName reads "Artist - 03 - Song", "03 - Song", "03 Song" and
// "1-03 Song" (disc 1, track 3).
func splitTrackFileName(base, artist string) (disc, num int, name string) {
	parts := strings.Split(base, " - ")
	for i, p := range parts {
		if m := reNumberPart.FindStringSubmatch(strings.TrimSpace(p)); m != nil && i < len(parts)-1 {
			disc, _ = strconv.Atoi(m[1])
			num, _ = strconv.Atoi(m[2])
			return disc, num, strings.TrimSpace(strings.Join(parts[i+1:], " - "))
		}
	}
	s := base
	if artist != "" && len(s) > len(artist)+3 && strings.EqualFold(s[:len(artist)], artist) && strings.HasPrefix(s[len(artist):], " - ") {
		s = s[len(artist)+3:]
	}
	if m := reLeadNumber.FindStringSubmatch(s); m != nil {
		disc, _ = strconv.Atoi(m[1])
		num, _ = strconv.Atoi(m[2])
		return disc, num, strings.TrimSpace(m[3])
	}
	return 0, 0, s
}

// leadingInt reads "3" or "3/12" as 3; anything else as 0.
func leadingInt(s string) int {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// yearKey sorts an artist's albums oldest first; unknown years go last.
func yearKey(y int) string {
	if y == 0 {
		return "9999"
	}
	return strconv.Itoa(y)
}
