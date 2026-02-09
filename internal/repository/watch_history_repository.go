package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type WatchHistoryRepository struct {
	db *sql.DB
}

func NewWatchHistoryRepository(db *sql.DB) *WatchHistoryRepository {
	return &WatchHistoryRepository{db: db}
}

func (r *WatchHistoryRepository) Upsert(wh *models.WatchHistory) error {
	query := `
		INSERT INTO watch_history (id, user_id, media_item_id, edition_group_id,
		                           progress_seconds, duration_seconds, completed, last_watched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, media_item_id) DO UPDATE SET
		    progress_seconds = EXCLUDED.progress_seconds,
		    duration_seconds = EXCLUDED.duration_seconds,
		    completed = EXCLUDED.completed,
		    edition_group_id = EXCLUDED.edition_group_id,
		    last_watched_at = CURRENT_TIMESTAMP
		RETURNING id, last_watched_at`
	return r.db.QueryRow(query, wh.ID, wh.UserID, wh.MediaItemID, wh.EditionGroupID,
		wh.ProgressSeconds, wh.DurationSeconds, wh.Completed).
		Scan(&wh.ID, &wh.LastWatchedAt)
}

func (r *WatchHistoryRepository) GetProgress(userID, mediaItemID uuid.UUID) (*models.WatchHistory, error) {
	wh := &models.WatchHistory{}
	query := `
		SELECT id, user_id, media_item_id, edition_group_id, progress_seconds,
		       duration_seconds, completed, last_watched_at
		FROM watch_history WHERE user_id = $1 AND media_item_id = $2`
	err := r.db.QueryRow(query, userID, mediaItemID).Scan(
		&wh.ID, &wh.UserID, &wh.MediaItemID, &wh.EditionGroupID,
		&wh.ProgressSeconds, &wh.DurationSeconds, &wh.Completed, &wh.LastWatchedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no watch history found")
	}
	return wh, err
}

func (r *WatchHistoryRepository) ContinueWatching(userID uuid.UUID, limit int) ([]*models.WatchHistory, error) {
	query := `
		SELECT wh.id, wh.user_id, wh.media_item_id, wh.edition_group_id,
		       wh.progress_seconds, wh.duration_seconds, wh.completed, wh.last_watched_at,
		       m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating
		FROM watch_history wh
		JOIN media_items m ON wh.media_item_id = m.id
		JOIN libraries l ON m.library_id = l.id
		WHERE wh.user_id = $1 AND wh.completed = false AND wh.progress_seconds > 0
		  AND l.is_enabled = true AND l.include_in_homepage = true
		ORDER BY wh.last_watched_at DESC
		LIMIT $2`

	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.WatchHistory
	for rows.Next() {
		wh := &models.WatchHistory{}
		mi := &models.MediaItem{}
		if err := rows.Scan(
			&wh.ID, &wh.UserID, &wh.MediaItemID, &wh.EditionGroupID,
			&wh.ProgressSeconds, &wh.DurationSeconds, &wh.Completed, &wh.LastWatchedAt,
			&mi.ID, &mi.LibraryID, &mi.MediaType, &mi.FilePath, &mi.FileName, &mi.FileSize,
			&mi.Title, &mi.DurationSeconds, &mi.Resolution, &mi.Width, &mi.Height,
			&mi.Codec, &mi.Container, &mi.PosterPath, &mi.Year, &mi.Rating,
		); err != nil {
			return nil, err
		}
		wh.MediaItem = mi
		results = append(results, wh)
	}
	return results, rows.Err()
}

func (r *WatchHistoryRepository) RecentlyWatched(userID uuid.UUID, limit int) ([]*models.WatchHistory, error) {
	query := `
		SELECT wh.id, wh.user_id, wh.media_item_id, wh.edition_group_id,
		       wh.progress_seconds, wh.duration_seconds, wh.completed, wh.last_watched_at,
		       m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating
		FROM watch_history wh
		JOIN media_items m ON wh.media_item_id = m.id
		JOIN libraries l ON m.library_id = l.id
		WHERE wh.user_id = $1
		  AND l.is_enabled = true AND l.include_in_homepage = true
		ORDER BY wh.last_watched_at DESC
		LIMIT $2`

	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.WatchHistory
	for rows.Next() {
		wh := &models.WatchHistory{}
		mi := &models.MediaItem{}
		if err := rows.Scan(
			&wh.ID, &wh.UserID, &wh.MediaItemID, &wh.EditionGroupID,
			&wh.ProgressSeconds, &wh.DurationSeconds, &wh.Completed, &wh.LastWatchedAt,
			&mi.ID, &mi.LibraryID, &mi.MediaType, &mi.FilePath, &mi.FileName, &mi.FileSize,
			&mi.Title, &mi.DurationSeconds, &mi.Resolution, &mi.Width, &mi.Height,
			&mi.Codec, &mi.Container, &mi.PosterPath, &mi.Year, &mi.Rating,
		); err != nil {
			return nil, err
		}
		wh.MediaItem = mi
		results = append(results, wh)
	}
	return results, rows.Err()
}
