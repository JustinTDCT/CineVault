package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Database   DatabaseConfig
	Redis      RedisConfig
	Server     ServerConfig
	JWT        JWTConfig
	Paths      PathsConfig
	FFmpeg     FFmpegConfig
	TMDBAPIKey string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
}

type ServerConfig struct {
	Host string
	Port int
}

type JWTConfig struct {
	Secret           string
	ExpiresIn        string
	RefreshExpiresIn string
}

type PathsConfig struct {
	Media     string
	Preview   string
	Thumbnail string
}

type FFmpegConfig struct {
	FFmpegPath  string
	FFprobePath string
}

func Load() (*Config, error) {
	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "cinevault"),
			Password: getEnv("DB_PASSWORD", "cinevault_dev_pass"),
			DBName:   getEnv("DB_NAME", "cinevault"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvInt("SERVER_PORT", 8080),
		},
		JWT: JWTConfig{
			Secret:           getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			ExpiresIn:        getEnv("JWT_EXPIRES_IN", "24h"),
			RefreshExpiresIn: getEnv("JWT_REFRESH_EXPIRES_IN", "168h"),
		},
		Paths: PathsConfig{
			Media:     getEnv("MEDIA_PATH", "/media"),
			Preview:   getEnv("PREVIEW_PATH", "/previews"),
			Thumbnail: getEnv("THUMBNAIL_PATH", "/thumbnails"),
		},
		FFmpeg: FFmpegConfig{
			FFmpegPath:  getEnv("FFMPEG_PATH", "/usr/bin/ffmpeg"),
			FFprobePath: getEnv("FFPROBE_PATH", "/usr/bin/ffprobe"),
		},
		TMDBAPIKey: getEnv("TMDB_API_KEY", ""),
	}, nil
}

func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

func (c *RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
