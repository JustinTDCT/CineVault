package collections

import (
	"encoding/json"
	"time"
)

type Collection struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	Name         string          `json:"name"`
	Description  *string         `json:"description,omitempty"`
	PosterPath   *string         `json:"poster_path,omitempty"`
	IsSmart      bool            `json:"is_smart"`
	SmartFilters json.RawMessage `json:"smart_filters,omitempty"`
	SortOrder    int             `json:"sort_order"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type CollectionItem struct {
	ID           string `json:"id"`
	CollectionID string `json:"collection_id"`
	MediaItemID  string `json:"media_item_id"`
	SortOrder    int    `json:"sort_order"`
}
