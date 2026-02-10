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

// Recommendations returns unwatched media items ranked by the user's genre affinity.
// It finds genres the user has watched, then returns unwatched items in those genres.
// Optional allowedRatings filters by content rating (for parental controls).
// Optional libraryIDs limits to accessible libraries.
func (r *WatchHistoryRepository) Recommendations(userID uuid.UUID, limit int, allowedRatings []string, libraryIDs []uuid.UUID) ([]*models.MediaItem, error) {
	// Build the optional filters
	var extraJoins []string
	var extraWheres []string
	args := []interface{}{userID}
	paramIdx := 2

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
		for i, r := range allowedRatings {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, r)
			paramIdx++
		}
		extraWheres = append(extraWheres, "(m.content_rating IN ("+strings.Join(placeholders, ",")+") OR m.content_rating IS NULL)")
	}

	args = append(args, limit)
	limitParam := fmt.Sprintf("$%d", paramIdx)

	joinSQL := ""
	if len(extraJoins) > 0 {
		joinSQL = " " + strings.Join(extraJoins, " ")
	}

	whereSQL := ""
	if len(extraWheres) > 0 {
		whereSQL = " AND " + strings.Join(extraWheres, " AND ")
	}

	query := `
		WITH user_genres AS (
			SELECT t.id AS tag_id, t.name, COUNT(*) AS watch_count
			FROM watch_history wh
			JOIN media_tags mt ON mt.media_item_id = wh.media_item_id
			JOIN tags t ON t.id = mt.tag_id AND t.category = 'genre'
			WHERE wh.user_id = $1
			GROUP BY t.id, t.name
			ORDER BY watch_count DESC
			LIMIT 10
		)
		SELECT DISTINCT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating, m.content_rating,
		       m.description, m.backdrop_path,
		       COUNT(ug.tag_id) AS genre_matches
		FROM media_items m
		JOIN media_tags mt ON mt.media_item_id = m.id
		JOIN user_genres ug ON ug.tag_id = mt.tag_id
		JOIN libraries l ON m.library_id = l.id` + joinSQL + `
		WHERE l.is_enabled = true AND l.include_in_homepage = true
		  AND m.id NOT IN (SELECT media_item_id FROM watch_history WHERE user_id = $1)
		  AND NOT EXISTS (SELECT 1 FROM edition_items ei WHERE ei.media_item_id = m.id AND ei.is_default = false)` + whereSQL + `
		GROUP BY m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		         m.title, m.duration_seconds, m.resolution, m.width, m.height,
		         m.codec, m.container, m.poster_path, m.year, m.rating, m.content_rating,
		         m.description, m.backdrop_path
		ORDER BY genre_matches DESC, m.rating DESC NULLS LAST
		LIMIT ` + limitParam

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		var genreMatches int
		if err := rows.Scan(
			&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName, &item.FileSize,
			&item.Title, &item.DurationSeconds, &item.Resolution, &item.Width, &item.Height,
			&item.Codec, &item.Container, &item.PosterPath, &item.Year, &item.Rating, &item.ContentRating,
			&item.Description, &item.BackdropPath,
			&genreMatches,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
