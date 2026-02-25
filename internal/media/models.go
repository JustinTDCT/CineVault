package media

import (
	"encoding/json"
	"time"
)

type MediaItem struct {
	ID                   string          `json:"id"`
	LibraryID            string          `json:"library_id"`
	CacheID              *string         `json:"cache_id,omitempty"`
	ParentID             *string         `json:"parent_id,omitempty"`
	Title                *string         `json:"title,omitempty"`
	OriginalTitle        *string         `json:"original_title,omitempty"`
	SortTitle            *string         `json:"sort_title,omitempty"`
	Description          *string         `json:"description,omitempty"`
	ReleaseDate          *string         `json:"release_date,omitempty"`
	ReleaseYear          *int            `json:"release_year,omitempty"`
	RuntimeMinutes       *int            `json:"runtime_minutes,omitempty"`
	FilePath             string          `json:"file_path"`
	FileSize             *int64          `json:"file_size,omitempty"`
	FileHash             *string         `json:"file_hash,omitempty"`
	FileModTime          *time.Time      `json:"file_mod_time,omitempty"`
	VideoCodec           *string         `json:"video_codec,omitempty"`
	AudioCodec           *string         `json:"audio_codec,omitempty"`
	Resolution           *string         `json:"resolution,omitempty"`
	Bitrate              *int            `json:"bitrate,omitempty"`
	PHash                *string         `json:"phash,omitempty"`
	MatchConfidence      float64         `json:"match_confidence"`
	MetadataLocked       bool            `json:"metadata_locked"`
	ManualOverrideFields []string        `json:"manual_override_fields"`
	Metadata             json.RawMessage `json:"metadata"`
	SeasonNumber         *int            `json:"season_number,omitempty"`
	EpisodeNumber        *int            `json:"episode_number,omitempty"`
	DateAdded            time.Time       `json:"date_added"`
	DateModified         time.Time       `json:"date_modified"`
}

type ListParams struct {
	LibraryID string
	Cursor    string
	Limit     int
	SortBy    string
	SortDir   string
	Search    string
}
