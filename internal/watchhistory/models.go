package watchhistory

import "time"

type WatchEntry struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	MediaItemID     string    `json:"media_item_id"`
	PositionSeconds float64   `json:"position_seconds"`
	DurationSeconds *float64  `json:"duration_seconds,omitempty"`
	Completed       bool      `json:"completed"`
	LastWatched     time.Time `json:"last_watched"`
	PlayCount       int       `json:"play_count"`
}
