package metadata

import (
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
		matches, err := scraper.Search(query, mediaType)
		if err != nil {
			log.Printf("Auto-match: %s search failed for %q: %v", scraper.Name(), query, err)
			continue
		}
		for _, m := range matches {
			conf := m.Confidence
			// Boost confidence when the year from the file matches the result year
			if yearHint != nil && m.Year != nil && *yearHint == *m.Year {
				conf += 0.15
				if conf > 1.0 {
					conf = 1.0
				}
			}
			// Penalize when we have a year but the result year doesn't match
			if yearHint != nil && m.Year != nil && *yearHint != *m.Year {
				conf -= 0.2
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

// DownloadPoster fetches an image from a URL and saves it to destPath.
// Returns the saved file path on success.
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

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create poster file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("write poster: %w", err)
	}

	return destPath, nil
}
