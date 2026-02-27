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
	"strconv"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
)

const (
	// scraperDelay prevents hammering external APIs
	scraperDelay = 300 * time.Millisecond

	// DefaultAutoMinMatch is the default minimum confidence for automatic indexing (95%)
	DefaultAutoMinMatch = 0.95
	// DefaultManualMinMatch is the default minimum confidence for manual matching (75%)
	DefaultManualMinMatch = 0.75
	// DefaultMaxResults is the default number of results returned for manual matching
	DefaultMaxResults = 5

	// Settings keys
	SettingAutoMinMatch  = "metadata_match_auto_min"
	SettingManualMinMatch = "metadata_match_manual_min"
	SettingMaxResults     = "metadata_match_max_results"
)

// MatchConfig holds configurable thresholds for metadata matching.
type MatchConfig struct {
	MinConfidence float64
	MaxResults    int
}

// AutoMatchConfig reads auto-match thresholds from system settings.
func AutoMatchConfig(settingsRepo *repository.SettingsRepository) MatchConfig {
	cfg := MatchConfig{MinConfidence: DefaultAutoMinMatch, MaxResults: 1}
	if settingsRepo == nil {
		return cfg
	}
	if v, err := settingsRepo.Get(SettingAutoMinMatch); err == nil && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 100 {
			if f > 1 {
				f = f / 100.0
			}
			cfg.MinConfidence = f
		}
	}
	return cfg
}

// ManualMatchConfig reads manual-match thresholds from system settings.
func ManualMatchConfig(settingsRepo *repository.SettingsRepository) MatchConfig {
	cfg := MatchConfig{MinConfidence: DefaultManualMinMatch, MaxResults: DefaultMaxResults}
	if settingsRepo == nil {
		return cfg
	}
	if v, err := settingsRepo.Get(SettingManualMinMatch); err == nil && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 100 {
			if f > 1 {
				f = f / 100.0
			}
			cfg.MinConfidence = f
		}
	}
	if v, err := settingsRepo.Get(SettingMaxResults); err == nil && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			cfg.MaxResults = n
		}
	}
	return cfg
}

// FindTopMatches searches all applicable scrapers and returns the top matches
// above the configured confidence threshold, sorted by confidence descending.
func FindTopMatches(scrapers []Scraper, query string, mediaType models.MediaType, cfg MatchConfig, itemYear ...*int) []*models.MetadataMatch {
	applicable := ScrapersForMediaType(scrapers, mediaType)
	if len(applicable) == 0 {
		return nil
	}

	var yearHint *int
	if len(itemYear) > 0 {
		yearHint = itemYear[0]
	}

	var all []*models.MetadataMatch
	for _, scraper := range applicable {
		matches, err := scraper.Search(query, mediaType, yearHint)
		if err != nil {
			log.Printf("Auto-match: %s search failed for %q: %v", scraper.Name(), query, err)
			continue
		}
		for _, m := range matches {
			conf := m.Confidence
			if yearHint != nil && m.Year != nil && *yearHint == *m.Year {
				conf += 0.20
				if conf > 1.0 {
					conf = 1.0
				}
			}
			if yearHint != nil && m.Year != nil && *yearHint != *m.Year {
				diff := *yearHint - *m.Year
				if diff < 0 {
					diff = -diff
				}
				if diff <= 1 {
					conf -= 0.10
				} else {
					conf -= 0.40
				}
			}
			m.Confidence = conf
			if m.Confidence >= cfg.MinConfidence {
				all = append(all, m)
			}
		}
		time.Sleep(scraperDelay)
	}

	// Sort by confidence descending
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Confidence > all[i].Confidence {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if cfg.MaxResults > 0 && len(all) > cfg.MaxResults {
		all = all[:cfg.MaxResults]
	}

	return all
}

// FindBestMatch is a convenience wrapper that returns the single highest-confidence match.
// Uses a low internal threshold (0.1) so callers can apply their own configurable threshold.
func FindBestMatch(scrapers []Scraper, query string, mediaType models.MediaType, itemYear ...*int) *models.MetadataMatch {
	cfg := MatchConfig{MinConfidence: 0.1, MaxResults: 1}
	matches := FindTopMatches(scrapers, query, mediaType, cfg, itemYear...)
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

// FindBestMatchWithCache tries the cache server first via Lookup, then falls
// back to direct scrapers if the cache has no match or is unavailable.
func FindBestMatchWithCache(cache *CacheClient, scrapers []Scraper, query string, mediaType models.MediaType, cfg MatchConfig, itemYear ...*int) *models.MetadataMatch {
	var yearHint *int
	if len(itemYear) > 0 {
		yearHint = itemYear[0]
	}

	if cache != nil {
		result := cache.Lookup(query, yearHint, mediaType)
		if result != nil && result.Match != nil && result.Match.Confidence >= cfg.MinConfidence {
			return result.Match
		}
	}

	return FindBestMatch(scrapers, query, mediaType, itemYear...)
}

// ScrapersForMediaType returns the scrapers relevant to a given media type.
func ScrapersForMediaType(scrapers []Scraper, mediaType models.MediaType) []Scraper {
	var result []Scraper
	for _, s := range scrapers {
		switch mediaType {
		case models.MediaTypeMovies, models.MediaTypeAdultMovies:
			if s.Name() == "tmdb" {
				result = append(result, s)
			}
		case models.MediaTypeMusicVideos:
			if s.Name() == "musicbrainz" {
				result = append(result, s)
			}
		case models.MediaTypeTVShows:
			// TMDB is primary, TVDB is fallback for TV shows
			if s.Name() == "tmdb" || s.Name() == "tvdb" {
				result = append(result, s)
			}
		case models.MediaTypeMusic:
			if s.Name() == "musicbrainz" {
				result = append(result, s)
			}
		case models.MediaTypeAudiobooks:
			if s.Name() == "openlibrary" || s.Name() == "audnexus" {
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

var (
	bracesRx      = regexp.MustCompile(`\{[^}]*\}`)
	bracketsRx    = regexp.MustCompile(`\[[^\]]*\]`)
	yearParensRx  = regexp.MustCompile(`[\(\[\{]\d{4}[\)\]\}]`)
	editionPhraseRx = regexp.MustCompile(`(?i)\b(` +
		`director'?s?\s*cut|final\s+cut|extended\s+cut|theatrical\s+cut|unrated\s+cut|ultimate\s+cut|` +
		`criterion\s+edition|anniversary\s+edition|collector'?s?\s+edition|ultimate\s+edition|` +
		`deluxe\s+edition|imax\s+edition|special\s+edition|limited\s+edition|` +
		`extended\s+edition|unrated\s+edition|theatrical\s+edition|remastered\s+edition` +
		`)\b`)
	junkRx = regexp.MustCompile(`(?i)\b(` +
		`x264|x265|h264|h265|h\.264|h\.265|hevc|avc|divx|xvid|10bit|8bit|hi10p|hi10|av1|vp9|mpeg4|` +
		`aac|ac3|ac-3|dts|dts-hd|dtshd|dts-x|truehd|atmos|flac|mp3|ogg|vorbis|opus|eac3|` +
		`dd5\.1|dd2\.0|5\.1ch|7\.1ch|5\.1|7\.1|2\.0|` +
		`480p|480i|576p|576i|720p|720i|1080p|1080i|2160p|4k|uhd|ultrahd|` +
		`bluray|blu-ray|bdrip|brrip|bdrc|bdremux|hdrip|hddvd|hddvdrip|` +
		`dvd|dvdrip|dvdscr|dvdscreener|` +
		`webrip|web-dl|webdl|` +
		`hdtv|pdtv|dsr|dsrip|stv|tvrip|` +
		`cam|screener|scr|tc|telecine|telesync|ppv|retail|` +
		`remux|proper|repack|rerip|internal|limited|custom|` +
		`extended|unrated|theatrical|remastered|` +
		`read\.nfo|readnfo|nfofix|nfo|` +
		`multi|multisubs|dubbed|subbed|subs|sub|` +
		`ws|fs|fragment|xxx|` +
		`directors[\s.]cut` +
		`)\b`)
	trailingGroupRx = regexp.MustCompile(`\s*-\s*\w+\s*$`)
	trailingDashRx  = regexp.MustCompile(`\s*[-–]\s*$`)
	multiSpaceRx    = regexp.MustCompile(`\s+`)
)

// CleanTitleForSearch strips common junk from titles to improve search accuracy.
// Uses comprehensive token-based cleaning inspired by Plex/Jellyfin.
func CleanTitleForSearch(title string) string {
	if title == "" {
		return ""
	}

	cleaned := title

	cleaned = bracesRx.ReplaceAllString(cleaned, " ")
	cleaned = bracketsRx.ReplaceAllString(cleaned, " ")
	cleaned = yearParensRx.ReplaceAllString(cleaned, " ")
	cleaned = editionPhraseRx.ReplaceAllString(cleaned, " ")
	cleaned = junkRx.ReplaceAllString(cleaned, " ")
	cleaned = trailingGroupRx.ReplaceAllString(cleaned, "")
	cleaned = trailingDashRx.ReplaceAllString(cleaned, "")
	cleaned = multiSpaceRx.ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}

// TitleFromFilename derives a clean display title from a media filename.
// Uses the same token-based approach as the scanner for consistency.
func TitleFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)
	return CleanTitleForSearch(baseName)
}

var (
	yearParensExtractRx    = regexp.MustCompile(`[\(\[]([12]\d{3})[\)\]]`)
	yearDelimitedExtractRx = regexp.MustCompile(`(?:[\.\-_,\s])([12]\d{3})(?:[\.\-_,+\s]|$)`)
)

// YearFromFilename extracts a 4-digit year from a filename.
// Uses improved patterns with delimiter requirements to avoid false positives.
func YearFromFilename(filename string) *int {
	if m := yearParensExtractRx.FindStringSubmatch(filename); len(m) >= 2 {
		var y int
		fmt.Sscanf(m[1], "%d", &y)
		if y >= 1900 && y <= 2100 {
			return &y
		}
	}
	if m := yearDelimitedExtractRx.FindStringSubmatch(filename); len(m) >= 2 {
		var y int
		fmt.Sscanf(m[1], "%d", &y)
		if y >= 1900 && y <= 2100 {
			return &y
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
