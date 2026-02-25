package config

import (
	"database/sql"
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port           int
	DatabaseURL    string
	JWTSecret      string
	DataDir        string
	CacheServerURL string
	CacheServerKey string
	FFmpegPath     string
	FFprobePath    string
	HWAccelType    string
	MaxTranscodes  int
}

func Load() *Config {
	return &Config{
		Port:           envInt("PORT", 8080),
		DatabaseURL:    env("DATABASE_URL", "postgres://cinevault:cinevault@db:5432/cinevault?sslmode=disable"),
		JWTSecret:      env("JWT_SECRET", "change-me-in-production"),
		DataDir:        env("DATA_DIR", "/data"),
		CacheServerURL: env("CACHE_SERVER_URL", ""),
		CacheServerKey: env("CACHE_SERVER_API_KEY", ""),
		FFmpegPath:     env("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:    env("FFPROBE_PATH", "ffprobe"),
		HWAccelType:    env("HW_ACCEL_TYPE", "cpu"),
		MaxTranscodes:  envInt("MAX_TRANSCODES", 2),
	}
}

func (c *Config) MergeFromDB(db *sql.DB) {
	rows, err := db.Query("SELECT key, value FROM settings")
	if err != nil {
		log.Printf("config: skipping DB merge: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch key {
		case "cache_server_url":
			c.CacheServerURL = value
		case "cache_server_api_key":
			c.CacheServerKey = value
		case "hw_accel_type":
			c.HWAccelType = value
		case "max_transcodes":
			if v, err := strconv.Atoi(value); err == nil {
				c.MaxTranscodes = v
			}
		}
	}
}

func (c *Config) CacheServerEnabled() bool {
	return c.CacheServerURL != "" && c.CacheServerKey != ""
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
