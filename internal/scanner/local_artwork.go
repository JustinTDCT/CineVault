package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
)

// LocalArtwork holds paths to artwork files found alongside media files.
// These follow the Plex/Jellyfin/Kodi naming conventions.
type LocalArtwork struct {
	PosterPath   string // poster.jpg, movie-poster.jpg, folder.jpg, <filename>-poster.jpg
	BackdropPath string // backdrop.jpg, fanart.jpg, background.jpg, <filename>-fanart.jpg
	LogoPath     string // logo.png, clearlogo.png
	BannerPath   string // banner.jpg
	ThumbPath    string // <filename>-thumb.jpg (for episodes)
}

// artworkExtensions lists the image extensions to check for each artwork name.
var artworkExtensions = []string{".jpg", ".jpeg", ".png", ".webp"}

// DetectLocalArtwork scans the directory containing the media file for
// local artwork files using Plex/Jellyfin/Kodi naming conventions.
func DetectLocalArtwork(mediaFilePath string, mediaType models.MediaType) *LocalArtwork {
	dir := filepath.Dir(mediaFilePath)
	base := strings.TrimSuffix(filepath.Base(mediaFilePath), filepath.Ext(mediaFilePath))

	art := &LocalArtwork{}

	// ── Poster detection ──
	// Priority order: <filename>-poster > poster > movie-poster > folder > show
	posterNames := []string{
		base + "-poster",
		"poster",
		"movie-poster",
		"folder",
		"cover",
	}
	if mediaType == models.MediaTypeTVShows {
		posterNames = append(posterNames, "show")
	}
	art.PosterPath = findArtworkFile(dir, posterNames)

	// ── Backdrop / Fanart detection ──
	backdropNames := []string{
		base + "-fanart",
		"backdrop",
		"fanart",
		"background",
		base + "-backdrop",
	}
	art.BackdropPath = findArtworkFile(dir, backdropNames)

	// ── Logo detection ──
	logoNames := []string{
		base + "-logo",
		"logo",
		"clearlogo",
	}
	art.LogoPath = findArtworkFile(dir, logoNames)

	// ── Banner detection ──
	bannerNames := []string{
		base + "-banner",
		"banner",
	}
	art.BannerPath = findArtworkFile(dir, bannerNames)

	// ── Thumb detection (primarily for episodes) ──
	thumbNames := []string{
		base + "-thumb",
		base,
	}
	if mediaType == models.MediaTypeTVShows {
		art.ThumbPath = findArtworkFile(dir, thumbNames)
	}

	return art
}

// DetectTVShowArtwork detects artwork in a TV show's root directory.
// This is for show-level art (not episode-level).
func DetectTVShowArtwork(showDir string) *LocalArtwork {
	art := &LocalArtwork{}

	posterNames := []string{"poster", "show", "folder", "cover"}
	art.PosterPath = findArtworkFile(showDir, posterNames)

	backdropNames := []string{"backdrop", "fanart", "background"}
	art.BackdropPath = findArtworkFile(showDir, backdropNames)

	logoNames := []string{"logo", "clearlogo"}
	art.LogoPath = findArtworkFile(showDir, logoNames)

	bannerNames := []string{"banner"}
	art.BannerPath = findArtworkFile(showDir, bannerNames)

	return art
}

// DetectSeasonArtwork detects season-specific artwork.
// Checks for seasonNN-poster.jpg, seasonNN-banner.jpg, etc.
func DetectSeasonArtwork(showDir string, seasonNumber int) *LocalArtwork {
	art := &LocalArtwork{}

	// Season poster: season01-poster.jpg or seasonNN.jpg
	seasonPrefix := formatSeasonPrefix(seasonNumber)
	posterNames := []string{
		seasonPrefix + "-poster",
		seasonPrefix,
	}

	// Also check in the Season N subdirectory
	seasonDir := filepath.Join(showDir, formatSeasonDir(seasonNumber))
	art.PosterPath = findArtworkFile(showDir, posterNames)
	if art.PosterPath == "" {
		subNames := []string{"poster", "folder", "cover"}
		art.PosterPath = findArtworkFile(seasonDir, subNames)
	}

	bannerNames := []string{seasonPrefix + "-banner"}
	art.BannerPath = findArtworkFile(showDir, bannerNames)

	return art
}

// HasAnyArtwork returns true if any artwork file was found.
func (a *LocalArtwork) HasAnyArtwork() bool {
	return a.PosterPath != "" || a.BackdropPath != "" || a.LogoPath != "" ||
		a.BannerPath != "" || a.ThumbPath != ""
}

// ──────────────────── Internal Helpers ────────────────────

// findArtworkFile checks a directory for artwork files matching any of the given
// base names with any of the standard image extensions.
func findArtworkFile(dir string, baseNames []string) string {
	for _, baseName := range baseNames {
		for _, ext := range artworkExtensions {
			path := filepath.Join(dir, baseName+ext)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

// formatSeasonPrefix returns "season01" format for a season number.
func formatSeasonPrefix(seasonNumber int) string {
	if seasonNumber == 0 {
		return "season-specials"
	}
	return "season" + padNumber(seasonNumber)
}

// formatSeasonDir returns "Season 1" format for a season number.
func formatSeasonDir(seasonNumber int) string {
	if seasonNumber == 0 {
		return "Specials"
	}
	return "Season " + strings.TrimLeft(padNumber(seasonNumber), "0")
}

func padNumber(n int) string {
	return fmt.Sprintf("%02d", n)
}
