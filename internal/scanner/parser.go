package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ParsedFile struct {
	Title         string
	Year          int
	Season        int
	Episode       int
	Resolution    string
	Source        string
	AudioCodec    string
	VideoCodec    string
	Edition       string
	IsMultiPart   bool
	PartNumber    int
	CleanFilename string
}

var (
	yearPattern     = regexp.MustCompile(`[\(\[\{]?((?:19|20)\d{2})[\)\]\}]?`)
	seasonEpPattern = regexp.MustCompile(`(?i)S(\d{1,4})E(\d{1,4})`)
	resPatterns     = regexp.MustCompile(`(?i)(2160p|1080p|720p|480p|4K|UHD)`)
	codecVideo      = regexp.MustCompile(`(?i)(x264|x265|h\.?264|h\.?265|hevc|avc|av1|vp9|mpeg[24])`)
	codecAudio      = regexp.MustCompile(`(?i)(aac|ac3|eac3|dts|dts-hd|truehd|atmos|flac|opus|mp3|pcm)`)
	sourcePattern   = regexp.MustCompile(`(?i)(bluray|blu-ray|bdrip|brrip|webrip|web-dl|webdl|hdtv|dvdrip|remux|hdtv)`)
	editionPattern  = regexp.MustCompile(`(?i)(director'?s?.?cut|extended|unrated|theatrical|remastered|special.?edition|imax|criterion)`)
	cleanupPattern  = regexp.MustCompile(`[\.\-_]+`)
	bracketPattern  = regexp.MustCompile(`[\[\(\{][^\]\)\}]*[\]\)\}]`)
)

func ParseFilename(filePath string) ParsedFile {
	name := filepath.Base(filePath)
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)

	p := ParsedFile{CleanFilename: name}

	if m := seasonEpPattern.FindStringSubmatch(name); len(m) == 3 {
		p.Season, _ = strconv.Atoi(m[1])
		p.Episode, _ = strconv.Atoi(m[2])
		name = seasonEpPattern.ReplaceAllString(name, "")
	}

	if m := yearPattern.FindStringSubmatch(name); len(m) == 2 {
		p.Year, _ = strconv.Atoi(m[1])
		name = strings.Replace(name, m[0], "", 1)
	}

	if m := resPatterns.FindString(name); m != "" {
		p.Resolution = strings.ToLower(m)
	}
	if m := codecVideo.FindString(name); m != "" {
		p.VideoCodec = strings.ToLower(m)
	}
	if m := codecAudio.FindString(name); m != "" {
		p.AudioCodec = strings.ToLower(m)
	}
	if m := sourcePattern.FindString(name); m != "" {
		p.Source = strings.ToLower(m)
	}
	if m := editionPattern.FindString(name); m != "" {
		p.Edition = m
	}

	title := resPatterns.ReplaceAllString(name, "")
	title = codecVideo.ReplaceAllString(title, "")
	title = codecAudio.ReplaceAllString(title, "")
	title = sourcePattern.ReplaceAllString(title, "")
	title = editionPattern.ReplaceAllString(title, "")
	title = bracketPattern.ReplaceAllString(title, "")
	title = cleanupPattern.ReplaceAllString(title, " ")
	p.Title = strings.TrimSpace(title)

	return p
}

var mediaExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".mov": true, ".wmv": true,
	".flv": true, ".webm": true, ".m4v": true, ".ts": true, ".mpg": true,
	".mpeg": true, ".3gp": true, ".ogv": true,
	".mp3": true, ".flac": true, ".wav": true, ".aac": true, ".ogg": true,
	".wma": true, ".m4a": true, ".opus": true, ".alac": true, ".aiff": true,
	".m4b": true,
	".epub": true, ".pdf": true, ".mobi": true, ".azw3": true,
	".cbz": true, ".cbr": true, ".cb7": true,
}

func IsMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return mediaExtensions[ext]
}
