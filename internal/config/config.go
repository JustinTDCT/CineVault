// Package config loads CineVault's runtime settings from environment variables.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds the server's runtime settings.
type Config struct {
	// Listen is the HTTP listen address, for example ":8456".
	Listen string
	// DatabaseURL is a PostgreSQL connection URL.
	DatabaseURL string
	// FFmpegPath is the ffmpeg binary to run.
	FFmpegPath string
	// FFprobePath is the ffprobe binary used to read media files.
	FFprobePath string
	// TranscodeMode is auto, nvenc or cpu; TranscodeGPU picks an NVIDIA GPU
	// by number (empty for any). See transcode.ParsePreference.
	TranscodeMode string
	TranscodeGPU  string
	// TranscodeDir holds streaming segments while videos play.
	TranscodeDir string
	// CacheDir holds things CineVault can make again, like video thumbnails.
	CacheDir string
	// PauseAhead, ResumeAhead and KeepBehind throttle conversions: FFmpeg
	// pauses once it is PauseAhead ahead of the player, resumes when the lead
	// drops to ResumeAhead, and segments more than KeepBehind behind the
	// player are deleted. Zero means the transcode package's defaults.
	PauseAhead, ResumeAhead, KeepBehind time.Duration
	// TMDbToken is a TMDb API read access token. Without it, titles aren't
	// matched to TMDb.
	TMDbToken string
	// ComicVineKey is a Comic Vine API key. Without it, comic series aren't
	// matched (they keep what ComicInfo.xml and file names say).
	ComicVineKey string
	// CacheURL is the CineVault Cache service; CacheToken is a subscriber's
	// token (empty on free servers, which only share titles with it).
	CacheURL   string
	CacheToken string
	// ProbeTimeout bounds each capability check at startup.
	ProbeTimeout time.Duration
}

// DefaultListen is the default HTTP port for CineVault.
const DefaultListen = ":8456"

// DefaultCacheURL is the public CineVault Cache.
const DefaultCacheURL = "https://cache.cine-vault.tv"

// Load reads settings from the environment, applying defaults.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	c := Config{
		Listen:        getenv("CINEVAULT_LISTEN"),
		DatabaseURL:   getenv("CINEVAULT_DATABASE_URL"),
		FFmpegPath:    getenv("CINEVAULT_FFMPEG"),
		FFprobePath:   getenv("CINEVAULT_FFPROBE"),
		TMDbToken:     getenv("CINEVAULT_TMDB_TOKEN"),
		ComicVineKey:  getenv("CINEVAULT_COMICVINE_KEY"),
		TranscodeMode: getenv("CINEVAULT_TRANSCODE"),
		TranscodeGPU:  getenv("CINEVAULT_TRANSCODE_GPU"),
		TranscodeDir:  getenv("CINEVAULT_TRANSCODE_DIR"),
		CacheDir:      getenv("CINEVAULT_CACHE_DIR"),
		CacheURL:      getenv("CINEVAULT_CACHE_URL"),
		CacheToken:    getenv("CINEVAULT_CACHE_TOKEN"),
		ProbeTimeout:  15 * time.Second,
	}
	if c.Listen == "" {
		c.Listen = DefaultListen
	}
	if c.CacheURL == "" {
		c.CacheURL = DefaultCacheURL
	}
	if u, err := url.Parse(c.CacheURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return c, fmt.Errorf("CINEVAULT_CACHE_URL must be an http(s) URL")
	}
	if c.FFmpegPath == "" {
		c.FFmpegPath = "ffmpeg"
	}
	if c.FFprobePath == "" {
		c.FFprobePath = "ffprobe"
	}
	if c.TranscodeDir == "" {
		c.TranscodeDir = filepath.Join(os.TempDir(), "cinevault-transcode")
	}
	if c.CacheDir == "" {
		if d, err := os.UserCacheDir(); err == nil {
			c.CacheDir = filepath.Join(d, "cinevault")
		} else {
			c.CacheDir = filepath.Join(os.TempDir(), "cinevault-cache")
		}
	}
	for _, d := range []struct {
		name string
		dst  *time.Duration
	}{
		{"CINEVAULT_TRANSCODE_PAUSE_AHEAD", &c.PauseAhead},
		{"CINEVAULT_TRANSCODE_RESUME_AHEAD", &c.ResumeAhead},
		{"CINEVAULT_TRANSCODE_KEEP_BEHIND", &c.KeepBehind},
	} {
		v, err := duration(getenv(d.name))
		if err != nil {
			return c, fmt.Errorf("%s: %v", d.name, err)
		}
		*d.dst = v
	}
	if c.ResumeAhead > 0 && c.PauseAhead > 0 && c.ResumeAhead >= c.PauseAhead {
		return c, fmt.Errorf("CINEVAULT_TRANSCODE_RESUME_AHEAD must be less than CINEVAULT_TRANSCODE_PAUSE_AHEAD")
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("CINEVAULT_DATABASE_URL is not set")
	}
	if _, err := DatabaseAddress(c.DatabaseURL); err != nil {
		return c, err
	}
	return c, nil
}

// duration parses a setting like "3m", "90s" or a plain number of seconds.
// Empty means zero (use the default).
func duration(v string) (time.Duration, error) {
	if v == "" {
		return 0, nil
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil {
		v = strconv.FormatFloat(n, 'f', -1, 64) + "s"
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("%q is not a duration like 3m or 90s", v)
	}
	return d, nil
}

// DatabaseAddress returns the host:port of a PostgreSQL URL, defaulting the port to 5432.
func DatabaseAddress(dbURL string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("CINEVAULT_DATABASE_URL is not a valid URL")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("CINEVAULT_DATABASE_URL must start with postgres://")
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("CINEVAULT_DATABASE_URL has no host")
	}
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	return net.JoinHostPort(host, port), nil
}
