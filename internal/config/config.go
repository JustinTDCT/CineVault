// Package config loads CineVault's runtime settings from environment variables.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
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
	// TMDbToken is a TMDb API read access token. Without it, titles aren't
	// matched to TMDb.
	TMDbToken string
	// ProbeTimeout bounds each capability check at startup.
	ProbeTimeout time.Duration
}

// DefaultListen is the default HTTP port for CineVault.
const DefaultListen = ":8456"

// Load reads settings from the environment, applying defaults.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	c := Config{
		Listen:       getenv("CINEVAULT_LISTEN"),
		DatabaseURL:  getenv("CINEVAULT_DATABASE_URL"),
		FFmpegPath:   getenv("CINEVAULT_FFMPEG"),
		FFprobePath:  getenv("CINEVAULT_FFPROBE"),
		TMDbToken:    getenv("CINEVAULT_TMDB_TOKEN"),
		ProbeTimeout: 15 * time.Second,
	}
	if c.Listen == "" {
		c.Listen = DefaultListen
	}
	if c.FFmpegPath == "" {
		c.FFmpegPath = "ffmpeg"
	}
	if c.FFprobePath == "" {
		c.FFprobePath = "ffprobe"
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("CINEVAULT_DATABASE_URL is not set")
	}
	if _, err := DatabaseAddress(c.DatabaseURL); err != nil {
		return c, err
	}
	return c, nil
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
