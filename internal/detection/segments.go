package detection

import "time"

type SegmentType string

const (
	SegmentIntro   SegmentType = "intro"
	SegmentCredits SegmentType = "credits"
	SegmentRecap   SegmentType = "recap"
)

type MediaSegment struct {
	ID          string      `json:"id"`
	MediaItemID string      `json:"media_item_id"`
	SegmentType SegmentType `json:"segment_type"`
	StartTime   float64     `json:"start_time"`
	EndTime     float64     `json:"end_time"`
	Confidence  float64     `json:"confidence"`
	CreatedAt   time.Time   `json:"created_at"`
}
