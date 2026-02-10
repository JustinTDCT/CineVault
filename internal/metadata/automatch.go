package metadata

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
)

const (
	// MinAutoMatchConfidence is the minimum confidence score to auto-apply metadata
	MinAutoMatchConfidence = 0.6
	// scraperDelay prevents hammering external APIs
	scraperDelay = 300 * time.Millisecond
)

// FindBestMatch searches all applicable scrapers for the best metadata match.
// It selects scrapers based on media type and returns the highest-confidence result.
// If itemYear is non-nil, results matching that year get a confidence boost.
func FindBestMatch(scrapers []Scraper, query string, mediaType models.MediaType, itemYear ...*int) *models.MetadataMatch {
	applicable := ScrapersForMediaType(scrapers, mediaType)
	if len(applicable) == 0 {
		return nil
	}

	// Extract year hint if provided
	var yearHint *int
	if len(itemYear) > 0 {
		yearHint = itemYear[0]
	}

	var best *models.MetadataMatch
	for _, scraper := range applicable {
		matches, err := scraper.Search(query, mediaType, yearHint)
		if err != nil {
			log.Printf("Auto-match: %s search failed for %q: %v", scraper.Name(), query, err)
			continue
		}
		for _, m := range matches {
			conf := m.Confidence
			// Boost confidence when the year from the file matches the result year
			if yearHint != nil && m.Year != nil && *yearHint == *m.Year {
				conf += 0.20
				if conf > 1.0 {
					conf = 1.0
				}
			}
			// Strong penalty when we have a year but the result year doesn't match
			if yearHint != nil && m.Year != nil && *yearHint != *m.Year {
				diff := *yearHint - *m.Year
				if diff < 0 {
					diff = -diff
				}
				if diff <= 1 {
					conf -= 0.10 // off by 1 year, mild penalty
				} else {
					conf -= 0.40 // more than 1 year off, strong penalty
				}
			}
			m.Confidence = conf
			if m.Confidence >= MinAutoMatchConfidence && (best == nil || m.Confidence > best.Confidence) {
				best = m
			}
		}
		// Rate-limit between scraper calls
		time.Sleep(scraperDelay)
	}
	return best
}

// ScrapersForMediaType returns the scrapers relevant to a given media type.
func ScrapersForMediaType(scrapers []Scraper, mediaType models.MediaType) []Scraper {
	var result []Scraper
	for _, s := range scrapers {
		switch mediaType {
		case models.MediaTypeMovies, models.MediaTypeAdultMovies, models.MediaTypeTVShows,
			models.MediaTypeMusicVideos:
			if s.Name() == "tmdb" {
				result = append(result, s)
			}
		case models.MediaTypeMusic:
			if s.Name() == "musicbrainz" {
				result = append(result, s)
			}
		case models.MediaTypeAudiobooks:
			if s.Name() == "openlibrary" {
				result = append(result, s)
			}
		}
	}
	return result
}

// ShouldAutoMatch returns true if the media type supports automatic metadata matching.
func ShouldAutoMatch(mediaType models.MediaType) bool {
	switch mediaType {
	case models.MediaTypeMovies, models.MediaTypeAdultMovies, models.MediaTypeTVShows,
		models.MediaTypeMusicVideos, models.MediaTypeMusic, models.MediaTypeAudiobooks:
		return true
	default:
		// home_videos, other_videos, images - skip
		return false
	}
}

// CleanTitleForSearch strips common junk from titles to improve search accuracy.
// Removes resolution tags, codec info, release group names, and year in brackets.
func CleanTitleForSearch(title string) string {
	junkPatterns := regexp.MustCompile(`(?i)\b(1080p|720p|480p|2160p|4k|uhd|bluray|blu-ray|brrip|bdrip|dvdrip|webrip|web-dl|webdl|hdtv|hdrip|x264|x265|h264|h265|hevc|aac|ac3|dts|atmos|remux|proper|repack|extended|unrated|directors cut|dc)\b`)
	cleaned := junkPatterns.ReplaceAllString(title, "")
	cleaned = regexp.MustCompile(`[\(\[\{]\d{4}[\)\]\}]`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`-\w+$`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

// TitleFromFilename derives a clean display title from a media filename.
// Strips the extension, replaces dots/underscores with spaces, removes
// edition tags, year brackets, resolution/codec junk, and collapses whitespace.
func TitleFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	// Strip edition tags: {edition-Remastered} etc.
	name = regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(name, "")
	// Strip year in parens/brackets: "Title - (2020)" → "Title -"
	name = regexp.MustCompile(`[\(\[\{]\d{4}[\)\]\}]`).ReplaceAllString(name, "")
	// Strip anything in square brackets: "[Bluray-1080p x265]" etc.
	name = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(name, "")
	// Strip resolution, codec, and release junk tokens
	name = regexp.MustCompile(`(?i)\b(1080p|720p|480p|2160p|4k|uhd|bluray|blu-ray|brrip|bdrip|dvdrip|webrip|web-dl|webdl|hdtv|hdrip|x264|x265|h264|h265|hevc|aac|ac3|dts|atmos|remux|proper|repack|extended|unrated|directors cut|dc)\b`).ReplaceAllString(name, "")
	// Strip trailing dash/whitespace separator
	name = regexp.MustCompile(`\s*-\s*$`).ReplaceAllString(name, "")
	// Collapse multiple spaces
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	return strings.TrimSpace(name)
}

// YearFromFilename extracts a 4-digit year from a filename.
func YearFromFilename(filename string) *int {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\((\d{4})\)`),
		regexp.MustCompile(`\[(\d{4})\]`),
		regexp.MustCompile(`[.\s_-](\d{4})[.\s_-]`),
	}
	for _, p := range patterns {
		matches := p.FindStringSubmatch(filename)
		if len(matches) >= 2 {
			var y int
			fmt.Sscanf(matches[1], "%d", &y)
			if y >= 1900 && y <= 2100 {
				return &y
			}
		}
	}
	return nil
}

// DownloadPoster fetches an image from a URL and saves it to destPath.
// Returns the saved file path on success.
// If a poster already exists for this item, compares content hashes:
//   - If identical: skips the download and returns the existing path
//   - If different: saves the new poster alongside the existing one with a suffix
func DownloadPoster(posterURL, destDir, filename string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create poster dir: %w", err)
	}

	destPath := filepath.Join(destDir, filename)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(posterURL)
	if err != nil {
		return "", fmt.Errorf("download poster: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("poster download returned %d", resp.StatusCode)
	}

	// Read entire response into memory for hash comparison
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read poster body: %w", err)
	}

	newHash := hashBytes(data)

	// Check if a poster already exists with the same content
	existingFiles := findExistingPosters(destDir, filename)
	for _, ef := range existingFiles {
		existingHash, err := hashFile(ef)
		if err != nil {
			continue
		}
		if existingHash == newHash {
			// Same image already exists — skip saving a duplicate
			log.Printf("Poster dedup: %s matches existing %s, skipping", filename, filepath.Base(ef))
			return ef, nil
		}
	}

	// If the primary filename already exists and has different content,
	// save the new poster with an alternative suffix
	if len(existingFiles) > 0 {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		altNum := len(existingFiles) // e.g., _alt1, _alt2, ...
		destPath = filepath.Join(destDir, fmt.Sprintf("%s_alt%d%s", base, altNum, ext))
	}

	// Save the new poster
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return "", fmt.Errorf("write poster: %w", err)
	}

	return destPath, nil
}

// hashBytes computes a hex-encoded MD5 hash of the given data.
func hashBytes(data []byte) string {
	h := md5.Sum(data)
	return fmt.Sprintf("%x", h)
}

// hashFile computes a hex-encoded MD5 hash of a file's contents.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

// findExistingPosters returns all poster files for a given item in the directory.
// For filename "abc-123.jpg", it matches "abc-123.jpg", "abc-123_alt1.jpg", etc.
func findExistingPosters(dir, filename string) []string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	pattern := filepath.Join(dir, base+"*"+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	return matches
}
