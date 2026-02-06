package models

import (
	"time"
	"github.com/google/uuid"
)

type UserRole string
const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
	RoleGuest UserRole = "guest"
)

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         UserRole  `json:"role" db:"role"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type MediaType string
const (
	MediaTypeMovies  MediaType = "movies"
	MediaTypeTVShows MediaType = "tv_shows"
)

type Library struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Name          string     `json:"name" db:"name"`
	MediaType     MediaType  `json:"media_type" db:"media_type"`
	Path          string     `json:"path" db:"path"`
	IsEnabled     bool       `json:"is_enabled" db:"is_enabled"`
	ScanOnStartup bool       `json:"scan_on_startup" db:"scan_on_startup"`
	LastScanAt    *time.Time `json:"last_scan_at" db:"last_scan_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

type MediaItem struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	LibraryID       uuid.UUID  `json:"library_id" db:"library_id"`
	MediaType       MediaType  `json:"media_type" db:"media_type"`
	FilePath        string     `json:"file_path" db:"file_path"`
	FileName        string     `json:"file_name" db:"file_name"`
	FileSize        int64      `json:"file_size" db:"file_size"`
	Title           string     `json:"title" db:"title"`
	DurationSeconds *int       `json:"duration_seconds" db:"duration_seconds"`
	Resolution      *string    `json:"resolution" db:"resolution"`
	Width           *int       `json:"width" db:"width"`
	Height          *int       `json:"height" db:"height"`
	Codec           *string    `json:"codec" db:"codec"`
	Container       *string    `json:"container" db:"container"`
	AddedAt         time.Time  `json:"added_at" db:"added_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}
