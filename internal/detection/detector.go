package detection

import (
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

type Detector struct {
	db         *sql.DB
	ffmpegPath string
}

func NewDetector(db *sql.DB, ffmpegPath string) *Detector {
	return &Detector{db: db, ffmpegPath: ffmpegPath}
}

func (d *Detector) DetectSegments(mediaItemID, filePath string, types []SegmentType) ([]MediaSegment, error) {
	var segments []MediaSegment

	for _, st := range types {
		seg, err := d.detect(filePath, st)
		if err != nil {
			log.Printf("detection: %s failed for %s: %v", st, mediaItemID, err)
			continue
		}
		if seg != nil {
			seg.MediaItemID = mediaItemID
			if err := d.save(seg); err != nil {
				log.Printf("detection: save failed: %v", err)
				continue
			}
			segments = append(segments, *seg)
		}
	}

	return segments, nil
}

func (d *Detector) detect(filePath string, segType SegmentType) (*MediaSegment, error) {
	cmd := exec.Command(d.ffmpegPath,
		"-i", filePath,
		"-filter_complex", "silencedetect=noise=-30dB:d=2",
		"-f", "null", "-")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	silences := parseSilenceDetect(output)
	if len(silences) == 0 {
		return nil, nil
	}

	var seg *MediaSegment
	switch segType {
	case SegmentIntro:
		for _, s := range silences {
			if s.start > 10 && s.start < 300 {
				seg = &MediaSegment{
					SegmentType: SegmentIntro,
					StartTime:   0,
					EndTime:     s.end,
					Confidence:  0.6,
				}
				break
			}
		}
	case SegmentCredits:
		if len(silences) > 0 {
			last := silences[len(silences)-1]
			seg = &MediaSegment{
				SegmentType: SegmentCredits,
				StartTime:   last.start,
				EndTime:      last.end + 60,
				Confidence:  0.5,
			}
		}
	}

	return seg, nil
}

func (d *Detector) save(seg *MediaSegment) error {
	return d.db.QueryRow(`
		INSERT INTO media_segments (media_item_id, segment_type, start_time, end_time, confidence)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		seg.MediaItemID, seg.SegmentType, seg.StartTime, seg.EndTime, seg.Confidence,
	).Scan(&seg.ID, &seg.CreatedAt)
}

func (d *Detector) GetSegments(mediaItemID string) ([]MediaSegment, error) {
	rows, err := d.db.Query(`
		SELECT id, media_item_id, segment_type, start_time, end_time, confidence, created_at
		FROM media_segments WHERE media_item_id=$1
		ORDER BY start_time`, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaSegment
	for rows.Next() {
		var s MediaSegment
		if err := rows.Scan(&s.ID, &s.MediaItemID, &s.SegmentType,
			&s.StartTime, &s.EndTime, &s.Confidence, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

type silence struct {
	start, end float64
}

func parseSilenceDetect(output string) []silence {
	var results []silence
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "silence_start:") {
			parts := strings.SplitAfter(line, "silence_start:")
			if len(parts) > 1 {
				start, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if err != nil {
					continue
				}
				end := start + 2
				if i+1 < len(lines) && strings.Contains(lines[i+1], "silence_end:") {
					endParts := strings.SplitAfter(lines[i+1], "silence_end:")
					if len(endParts) > 1 {
						fields := strings.Fields(endParts[1])
						if len(fields) > 0 {
							if e, err := strconv.ParseFloat(fields[0], 64); err == nil {
								end = e
							}
						}
					}
				}
				results = append(results, silence{start, end})
			}
		}
	}
	return results
}

func (d *Detector) DeleteSegments(mediaItemID string) error {
	_, err := d.db.Exec("DELETE FROM media_segments WHERE media_item_id=$1", mediaItemID)
	return err
}

func (d *Detector) DeleteSegment(id string) error {
	_, err := d.db.Exec("DELETE FROM media_segments WHERE id=$1", id)
	return err
}

func (d *Detector) BuildSkipInfo(mediaItemID string) (map[string]interface{}, error) {
	segments, err := d.GetSegments(mediaItemID)
	if err != nil {
		return nil, err
	}
	info := map[string]interface{}{"segments": segments}
	for _, s := range segments {
		info[fmt.Sprintf("has_%s", s.SegmentType)] = true
	}
	return info, nil
}
