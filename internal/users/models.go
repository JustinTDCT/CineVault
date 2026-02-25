package users

import (
	"encoding/json"
	"time"
)

type AccountType string

const (
	AccountOwner  AccountType = "owner"
	AccountShared AccountType = "shared"
	AccountSub    AccountType = "sub"
)

type User struct {
	ID                string          `json:"id"`
	AccountType       AccountType     `json:"account_type"`
	ParentID          *string         `json:"parent_id,omitempty"`
	FullName          string          `json:"full_name"`
	Email             string          `json:"email"`
	PasswordHash      string          `json:"-"`
	PIN               *string         `json:"-"`
	IsChild           bool            `json:"is_child"`
	ChildRestrictions json.RawMessage `json:"child_restrictions,omitempty"`
	AvatarPath        *string         `json:"avatar_path,omitempty"`
	IsAdmin           bool            `json:"is_admin"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type OverlaySetting struct {
	Enabled  bool   `json:"enabled"`
	Position string `json:"position"`
}

type OverlaySettings struct {
	ResolutionAudio OverlaySetting `json:"resolution_audio"`
	Edition         OverlaySetting `json:"edition"`
	Ratings         OverlaySetting `json:"ratings"`
	ContentRating   OverlaySetting `json:"content_rating"`
	SourceType      OverlaySetting `json:"source_type"`
	HideTheatrical  bool           `json:"hide_theatrical"`
}

type UserProfile struct {
	ID                  string          `json:"id"`
	UserID              string          `json:"user_id"`
	DefaultVideoQuality string          `json:"default_video_quality"`
	AutoPlayMusic       bool            `json:"auto_play_music"`
	AutoPlayVideos      bool            `json:"auto_play_videos"`
	AutoPlayMusicVideos bool            `json:"auto_play_music_videos"`
	AutoPlayAudiobooks  bool            `json:"auto_play_audiobooks"`
	OverlaySettings     json.RawMessage `json:"overlay_settings"`
	LibraryOrder        []string        `json:"library_order"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}
