package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type SegmentRepository struct {
	db *sql.DB
}

func NewSegmentRepository(db *sql.DB) *SegmentRepository {
	return &SegmentRepository{db: db}
}

// ──────────────────── Media Segments ────────────────────

// GetByMediaID returns all segments for a media item.
func (r *SegmentRepository) GetByMediaID(mediaItemID uuid.UUID) ([]*models.MediaSegment, error) {
	query := `
		SELECT id, media_item_id, segment_type, start_seconds, end_seconds,
		       confidence, source, verified, created_at, updated_at
		FROM media_segments
		WHERE media_item_id = $1
		ORDER BY start_seconds`
	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []*models.MediaSegment
	for rows.Next() {
		seg := &models.MediaSegment{}
		if err := rows.Scan(&seg.ID, &seg.MediaItemID, &seg.SegmentType,
			&seg.StartSeconds, &seg.EndSeconds, &seg.Confidence,
			&seg.Source, &seg.Verified, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return nil, err
		}
		segments = append(segments, seg)
	}
	return segments, rows.Err()
}

// Upsert inserts or updates a segment (unique on media_item_id + segment_type).
func (r *SegmentRepository) Upsert(seg *models.MediaSegment) error {
	query := `
		INSERT INTO media_segments (id, media_item_id, segment_type, start_seconds, end_seconds,
		                            confidence, source, verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (media_item_id, segment_type) DO UPDATE SET
		    start_seconds = EXCLUDED.start_seconds,
		    end_seconds = EXCLUDED.end_seconds,
		    confidence = EXCLUDED.confidence,
		    source = EXCLUDED.source,
		    verified = EXCLUDED.verified,
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(query, seg.ID, seg.MediaItemID, seg.SegmentType,
		seg.StartSeconds, seg.EndSeconds, seg.Confidence, seg.Source, seg.Verified).
		Scan(&seg.ID, &seg.CreatedAt, &seg.UpdatedAt)
}

// Delete removes a segment by media item ID and type.
func (r *SegmentRepository) Delete(mediaItemID uuid.UUID, segType string) error {
	res, err := r.db.Exec(`DELETE FROM media_segments WHERE media_item_id = $1 AND segment_type = $2`,
		mediaItemID, segType)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("segment not found")
	}
	return nil
}

// DeleteAllForMedia removes all segments for a media item.
func (r *SegmentRepository) DeleteAllForMedia(mediaItemID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_segments WHERE media_item_id = $1`, mediaItemID)
	return err
}

// BulkUpsert inserts or updates multiple segments in a single transaction.
func (r *SegmentRepository) BulkUpsert(segments []*models.MediaSegment) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO media_segments (id, media_item_id, segment_type, start_seconds, end_seconds,
		                            confidence, source, verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (media_item_id, segment_type) DO UPDATE SET
		    start_seconds = EXCLUDED.start_seconds,
		    end_seconds = EXCLUDED.end_seconds,
		    confidence = EXCLUDED.confidence,
		    source = EXCLUDED.source,
		    verified = EXCLUDED.verified,
		    updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, seg := range segments {
		if seg.ID == uuid.Nil {
			seg.ID = uuid.New()
		}
		_, err = stmt.Exec(seg.ID, seg.MediaItemID, seg.SegmentType,
			seg.StartSeconds, seg.EndSeconds, seg.Confidence, seg.Source, seg.Verified)
		if err != nil {
			return fmt.Errorf("upsert segment %s: %w", seg.SegmentType, err)
		}
	}

	return tx.Commit()
}

// CountByLibrary returns the number of media items with detected segments in a library.
func (r *SegmentRepository) CountByLibrary(libraryID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(DISTINCT ms.media_item_id)
		FROM media_segments ms
		JOIN media_items mi ON mi.id = ms.media_item_id
		WHERE mi.library_id = $1`, libraryID).Scan(&count)
	return count, err
}

// ListItemsWithoutSegments returns media items in a library that have no detected segments.
func (r *SegmentRepository) ListItemsWithoutSegments(libraryID uuid.UUID, mediaTypes []string) ([]*models.MediaItem, error) {
	query := `
		SELECT mi.id, mi.library_id, mi.media_type, mi.file_path, mi.file_name,
		       mi.duration_seconds, mi.tv_show_id, mi.tv_season_id, mi.episode_number
		FROM media_items mi
		WHERE mi.library_id = $1
		  AND mi.duration_seconds IS NOT NULL
		  AND mi.duration_seconds > 60
		  AND NOT EXISTS (SELECT 1 FROM media_segments ms WHERE ms.media_item_id = mi.id)
		  AND mi.media_type = ANY($2)
		ORDER BY mi.title`
	rows, err := r.db.Query(query, libraryID, mediaTypes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName,
			&item.DurationSeconds, &item.TVShowID, &item.TVSeasonID, &item.EpisodeNumber); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListEpisodesBySeasonID returns all episodes in a season ordered by episode number.
func (r *SegmentRepository) ListEpisodesBySeasonID(seasonID uuid.UUID) ([]*models.MediaItem, error) {
	query := `
		SELECT mi.id, mi.library_id, mi.media_type, mi.file_path, mi.file_name,
		       mi.duration_seconds, mi.tv_show_id, mi.tv_season_id, mi.episode_number
		FROM media_items mi
		WHERE mi.tv_season_id = $1
		  AND mi.duration_seconds IS NOT NULL
		ORDER BY mi.episode_number`
	rows, err := r.db.Query(query, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName,
			&item.DurationSeconds, &item.TVShowID, &item.TVSeasonID, &item.EpisodeNumber); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListSeasonIDsInLibrary returns distinct season IDs for all episodes in a library.
func (r *SegmentRepository) ListSeasonIDsInLibrary(libraryID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT mi.tv_season_id
		FROM media_items mi
		WHERE mi.library_id = $1
		  AND mi.tv_season_id IS NOT NULL
		  AND mi.duration_seconds IS NOT NULL
		  AND mi.duration_seconds > 60`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ──────────────────── User Skip Preferences ────────────────────

// GetSkipPrefs returns skip preferences for a user. Returns defaults if none set.
func (r *SegmentRepository) GetSkipPrefs(userID uuid.UUID) (*models.UserSkipPreference, error) {
	pref := &models.UserSkipPreference{}
	query := `
		SELECT id, user_id, skip_intros, skip_credits, skip_recaps,
		       show_skip_button, created_at, updated_at
		FROM user_skip_preferences WHERE user_id = $1`
	err := r.db.QueryRow(query, userID).Scan(
		&pref.ID, &pref.UserID, &pref.SkipIntros, &pref.SkipCredits,
		&pref.SkipRecaps, &pref.ShowSkipButton, &pref.CreatedAt, &pref.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// Return defaults
		return &models.UserSkipPreference{
			UserID:         userID,
			SkipIntros:     false,
			SkipCredits:    false,
			SkipRecaps:     false,
			ShowSkipButton: true,
		}, nil
	}
	return pref, err
}

// UpsertSkipPrefs inserts or updates user skip preferences.
func (r *SegmentRepository) UpsertSkipPrefs(pref *models.UserSkipPreference) error {
	query := `
		INSERT INTO user_skip_preferences (id, user_id, skip_intros, skip_credits, skip_recaps, show_skip_button)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
		    skip_intros = EXCLUDED.skip_intros,
		    skip_credits = EXCLUDED.skip_credits,
		    skip_recaps = EXCLUDED.skip_recaps,
		    show_skip_button = EXCLUDED.show_skip_button,
		    updated_at = CURRENT_TIMESTAMP`
	_, err := r.db.Exec(query, uuid.New(), pref.UserID, pref.SkipIntros,
		pref.SkipCredits, pref.SkipRecaps, pref.ShowSkipButton)
	return err
}
