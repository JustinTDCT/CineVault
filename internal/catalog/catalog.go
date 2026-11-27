// Package catalog holds CineVault's library data types, shared by the database
// layer and the API without tying the API to a database driver.
package catalog

import (
	"errors"
	"time"
)

// MediaTypes are the kinds of library CineVault supports.
var MediaTypes = []string{"movie", "tv", "music", "audiobook", "book"}

// EditionType is one entry in the Edition List: a fixed edition name such as
// "Director's Cut" (see docs/specs/editions-and-versions.md §4).
type EditionType struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	ShortLabel  string   `json:"short_label"`
	MediaTypes  []string `json:"media_types"`
	Aliases     []string `json:"aliases"`
	SortOrder   int      `json:"sort_order"`
	IsDefault   bool     `json:"is_default"`
	Builtin     bool     `json:"builtin"`
	Hidden      bool     `json:"hidden"`
}

// AppliesTo reports whether the edition type can be used for the media type.
func (e EditionType) AppliesTo(mediaType string) bool {
	for _, m := range e.MediaTypes {
		if m == mediaType {
			return true
		}
	}
	return false
}

// ValidMediaType reports whether s is a supported media type.
func ValidMediaType(s string) bool {
	for _, m := range MediaTypes {
		if m == s {
			return true
		}
	}
	return false
}

// Errors shared by stores and the API.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

// Library is a set of folders holding one media type.
type Library struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	MediaType string    `json:"media_type"`
	Paths     []string  `json:"paths"`
	CreatedAt time.Time `json:"created_at"`
}

// Title is one work (a movie, show, album...) with its editions.
type Title struct {
	ID        int64     `json:"id"`
	LibraryID int64     `json:"library_id"`
	MediaType string    `json:"media_type"`
	Name      string    `json:"name"`
	SortName  string    `json:"sort_name"`
	Year      int       `json:"year,omitempty"`
	TMDbID    int       `json:"tmdb_id,omitempty"`
	Editions  []Edition `json:"editions"`
}

// Edition is one cut of a title, such as the Director's Cut, with its files.
type Edition struct {
	ID             int64     `json:"id"`
	EditionTypeID  string    `json:"edition_type_id"`
	DisplayName    string    `json:"display_name"`
	ShortLabel     string    `json:"short_label"`
	RuntimeSeconds int       `json:"runtime_seconds,omitempty"`
	Versions       []Version `json:"versions"`
}

// Version is one file of an edition, such as a 4K or 1080p copy.
type Version struct {
	ID              int64   `json:"id"`
	Path            string  `json:"path"`
	SizeBytes       int64   `json:"size_bytes"`
	Label           string  `json:"label,omitempty"`
	Container       string  `json:"container,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	HDR             string  `json:"hdr,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
	AudioChannels   int     `json:"audio_channels,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Bitrate         int64   `json:"bitrate,omitempty"`
	EditionLocked   bool    `json:"edition_locked"`
	Missing         bool    `json:"missing"`
}
