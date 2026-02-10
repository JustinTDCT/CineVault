package repository

import (
	"database/sql"
	"fmt"
	"strings"

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

// Recommendations returns unwatched media items ranked by a weighted scoring algorithm.
// Factors: genre affinity (recency-weighted), cast/director affinity, rating floor,
// skip detection (items started but abandoned early are excluded).
func (r *WatchHistoryRepository) Recommendations(userID uuid.UUID, limit int, allowedRatings []string, libraryIDs []uuid.UUID) ([]*models.MediaItem, error) {
	args := []interface{}{userID}
	paramIdx := 2

	var extraWheres []string

	if len(libraryIDs) > 0 {
		placeholders := make([]string, len(libraryIDs))
		for i, id := range libraryIDs {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, id)
			paramIdx++
		}
		extraWheres = append(extraWheres, "m.library_id IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(allowedRatings) > 0 {
		placeholders := make([]string, len(allowedRatings))
		for i, rating := range allowedRatings {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, rating)
			paramIdx++
		}
		extraWheres = append(extraWheres, "(m.content_rating IN ("+strings.Join(placeholders, ",")+") OR m.content_rating IS NULL)")
	}

	args = append(args, limit)
	limitParam := fmt.Sprintf("$%d", paramIdx)

	whereSQL := ""
	if len(extraWheres) > 0 {
		whereSQL = " AND " + strings.Join(extraWheres, " AND ")
	}

	// Enhanced recommendation query:
	// 1. user_genres: Weighted by recency (items watched recently score higher) and completion
	//    - Completed items get full weight, abandoned items (<10% progress) get zero weight
	//    - Recency decay: 1/(1 + days_since_watch/30) gives higher weight to recent watches
	// 2. user_performers: Top directors/actors from completed watches
	// 3. Final score = genre_score + performer_score, filtered by rating floor (5.0+)
	query := `
		WITH user_genres AS (
			SELECT t.id AS tag_id, t.name,
			       SUM(
			           CASE WHEN wh.completed THEN 1.0
			                WHEN wh.duration_seconds > 0 AND wh.progress_seconds::float / wh.duration_seconds > 0.5 THEN 0.5
			                ELSE 0.0
			           END
			           / (1.0 + EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - wh.last_watched_at)) / 2592000.0)
			       ) AS affinity_score
			FROM watch_history wh
			JOIN media_tags mt ON mt.media_item_id = wh.media_item_id
			JOIN tags t ON t.id = mt.tag_id AND t.category = 'genre'
			WHERE wh.user_id = $1
			  AND NOT (wh.completed = false AND wh.duration_seconds > 0
			           AND wh.progress_seconds::float / wh.duration_seconds < 0.1
			           AND wh.progress_seconds > 0)
			GROUP BY t.id, t.name
			ORDER BY affinity_score DESC
			LIMIT 15
		),
		user_performers AS (
			SELECT p.id AS performer_id,
			       COUNT(*) AS watch_count
			FROM watch_history wh
			JOIN media_performers mp ON mp.media_item_id = wh.media_item_id
			JOIN performers p ON p.id = mp.performer_id
			WHERE wh.user_id = $1
			  AND wh.completed = true
			  AND p.performer_type IN ('director', 'actor')
			GROUP BY p.id
			ORDER BY watch_count DESC
			LIMIT 20
		),
		skipped_items AS (
			SELECT media_item_id FROM watch_history
			WHERE user_id = $1
			  AND completed = false AND duration_seconds > 0
			  AND progress_seconds::float / duration_seconds < 0.1
			  AND progress_seconds > 0
		)
		SELECT DISTINCT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating, m.content_rating,
		       m.description, m.backdrop_path,
		       COALESCE(gs.genre_score, 0) + COALESCE(ps.performer_score, 0) AS total_score
		FROM media_items m
		JOIN libraries l ON m.library_id = l.id
		LEFT JOIN (
			SELECT mt.media_item_id, SUM(ug.affinity_score) AS genre_score
			FROM media_tags mt
			JOIN user_genres ug ON ug.tag_id = mt.tag_id
			GROUP BY mt.media_item_id
		) gs ON gs.media_item_id = m.id
		LEFT JOIN (
			SELECT mp.media_item_id, COUNT(DISTINCT up.performer_id) * 0.5 AS performer_score
			FROM media_performers mp
			JOIN user_performers up ON up.performer_id = mp.performer_id
			GROUP BY mp.media_item_id
		) ps ON ps.media_item_id = m.id
		WHERE l.is_enabled = true AND l.include_in_homepage = true
		  AND m.id NOT IN (SELECT media_item_id FROM watch_history WHERE user_id = $1)
		  AND m.id NOT IN (SELECT media_item_id FROM skipped_items)
		  AND NOT EXISTS (SELECT 1 FROM edition_items ei WHERE ei.media_item_id = m.id AND ei.is_default = false)
		  AND (COALESCE(gs.genre_score, 0) + COALESCE(ps.performer_score, 0)) > 0
		  AND (m.rating IS NULL OR m.rating >= 5.0)` + whereSQL + `
		ORDER BY total_score DESC, m.rating DESC NULLS LAST
		LIMIT ` + limitParam

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		var totalScore float64
		if err := rows.Scan(
			&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName, &item.FileSize,
			&item.Title, &item.DurationSeconds, &item.Resolution, &item.Width, &item.Height,
			&item.Codec, &item.Container, &item.PosterPath, &item.Year, &item.Rating, &item.ContentRating,
			&item.Description, &item.BackdropPath,
			&totalScore,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// BecauseYouWatched returns "Because you watched X" recommendation rows.
// For each of the user's last N completed items, it finds similar unwatched items
// based on shared genres, cast, and director.
func (r *WatchHistoryRepository) BecauseYouWatched(userID uuid.UUID, sourceLimit int, similarLimit int, allowedRatings []string, libraryIDs []uuid.UUID) ([]*models.BecauseYouWatchedRow, error) {
	// Step 1: Get the user's most recently completed items
	sourceQuery := `
		SELECT wh.media_item_id, m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating, m.content_rating,
		       m.description, m.backdrop_path
		FROM watch_history wh
		JOIN media_items m ON wh.media_item_id = m.id
		JOIN libraries l ON m.library_id = l.id
		WHERE wh.user_id = $1 AND wh.completed = true
		  AND l.is_enabled = true AND l.include_in_homepage = true
		ORDER BY wh.last_watched_at DESC
		LIMIT $2`

	sourceRows, err := r.db.Query(sourceQuery, userID, sourceLimit)
	if err != nil {
		return nil, err
	}
	defer sourceRows.Close()

	type sourceItem struct {
		mediaItemID uuid.UUID
		item        *models.MediaItem
	}
	var sources []sourceItem
	for sourceRows.Next() {
		var mediaItemID uuid.UUID
		item := &models.MediaItem{}
		if err := sourceRows.Scan(
			&mediaItemID,
			&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName, &item.FileSize,
			&item.Title, &item.DurationSeconds, &item.Resolution, &item.Width, &item.Height,
			&item.Codec, &item.Container, &item.PosterPath, &item.Year, &item.Rating, &item.ContentRating,
			&item.Description, &item.BackdropPath,
		); err != nil {
			return nil, err
		}
		sources = append(sources, sourceItem{mediaItemID: mediaItemID, item: item})
	}
	if err := sourceRows.Err(); err != nil {
		return nil, err
	}

	// Step 2: For each source item, find similar unwatched items
	var results []*models.BecauseYouWatchedRow
	for _, src := range sources {
		similar, err := r.findSimilar(userID, src.mediaItemID, similarLimit, allowedRatings, libraryIDs)
		if err != nil {
			continue
		}
		if len(similar) >= 2 { // Only show rows with at least 2 similar items
			results = append(results, &models.BecauseYouWatchedRow{
				SourceItem:   src.item,
				SimilarItems: similar,
			})
		}
	}
	return results, nil
}

// findSimilar finds unwatched items similar to the given media item based on
// shared genres, cast (actors), and director.
func (r *WatchHistoryRepository) findSimilar(userID, sourceMediaID uuid.UUID, limit int, allowedRatings []string, libraryIDs []uuid.UUID) ([]*models.MediaItem, error) {
	args := []interface{}{sourceMediaID, userID}
	paramIdx := 3

	var extraWheres []string

	if len(libraryIDs) > 0 {
		placeholders := make([]string, len(libraryIDs))
		for i, id := range libraryIDs {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, id)
			paramIdx++
		}
		extraWheres = append(extraWheres, "m.library_id IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(allowedRatings) > 0 {
		placeholders := make([]string, len(allowedRatings))
		for i, rating := range allowedRatings {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, rating)
			paramIdx++
		}
		extraWheres = append(extraWheres, "(m.content_rating IN ("+strings.Join(placeholders, ",")+") OR m.content_rating IS NULL)")
	}

	args = append(args, limit)
	limitParam := fmt.Sprintf("$%d", paramIdx)

	whereSQL := ""
	if len(extraWheres) > 0 {
		whereSQL = " AND " + strings.Join(extraWheres, " AND ")
	}

	query := `
		WITH source_genres AS (
			SELECT mt.tag_id FROM media_tags mt
			JOIN tags t ON t.id = mt.tag_id AND t.category = 'genre'
			WHERE mt.media_item_id = $1
		),
		source_performers AS (
			SELECT mp.performer_id FROM media_performers mp
			JOIN performers p ON p.id = mp.performer_id
			WHERE mp.media_item_id = $1 AND p.performer_type IN ('director', 'actor')
		)
		SELECT DISTINCT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating, m.content_rating,
		       m.description, m.backdrop_path,
		       (
		           SELECT COUNT(*) FROM media_tags mt2
		           JOIN source_genres sg ON sg.tag_id = mt2.tag_id
		           WHERE mt2.media_item_id = m.id
		       ) +
		       (
		           SELECT COUNT(*) FROM media_performers mp2
		           JOIN source_performers sp ON sp.performer_id = mp2.performer_id
		           WHERE mp2.media_item_id = m.id
		       ) * 2 AS similarity_score
		FROM media_items m
		JOIN libraries l ON m.library_id = l.id
		WHERE m.id != $1
		  AND l.is_enabled = true AND l.include_in_homepage = true
		  AND m.id NOT IN (SELECT media_item_id FROM watch_history WHERE user_id = $2)
		  AND NOT EXISTS (SELECT 1 FROM edition_items ei WHERE ei.media_item_id = m.id AND ei.is_default = false)
		  AND (m.rating IS NULL OR m.rating >= 5.0)
		  AND (
		      EXISTS (SELECT 1 FROM media_tags mt3 JOIN source_genres sg2 ON sg2.tag_id = mt3.tag_id WHERE mt3.media_item_id = m.id)
		      OR EXISTS (SELECT 1 FROM media_performers mp3 JOIN source_performers sp2 ON sp2.performer_id = mp3.performer_id WHERE mp3.media_item_id = m.id)
		  )` + whereSQL + `
		ORDER BY similarity_score DESC, m.rating DESC NULLS LAST
		LIMIT ` + limitParam

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		var simScore int
		if err := rows.Scan(
			&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName, &item.FileSize,
			&item.Title, &item.DurationSeconds, &item.Resolution, &item.Width, &item.Height,
			&item.Codec, &item.Container, &item.PosterPath, &item.Year, &item.Rating, &item.ContentRating,
			&item.Description, &item.BackdropPath,
			&simScore,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
