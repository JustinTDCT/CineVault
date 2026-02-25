package watchhistory

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Upsert(e *WatchEntry) error {
	return r.db.QueryRow(`
		INSERT INTO watch_history (user_id, media_item_id, position_seconds, duration_seconds, completed, play_count)
		VALUES ($1,$2,$3,$4,$5,1)
		ON CONFLICT (user_id, media_item_id)
		DO UPDATE SET position_seconds=$3, duration_seconds=$4, completed=$5,
		              play_count=watch_history.play_count+1, last_watched=NOW()
		RETURNING id, last_watched`,
		e.UserID, e.MediaItemID, e.PositionSeconds, e.DurationSeconds, e.Completed,
	).Scan(&e.ID, &e.LastWatched)
}

func (r *Repository) GetByUserAndItem(userID, mediaItemID string) (*WatchEntry, error) {
	e := &WatchEntry{}
	err := r.db.QueryRow(`
		SELECT id, user_id, media_item_id, position_seconds, duration_seconds,
		       completed, last_watched, play_count
		FROM watch_history WHERE user_id=$1 AND media_item_id=$2`,
		userID, mediaItemID,
	).Scan(&e.ID, &e.UserID, &e.MediaItemID, &e.PositionSeconds, &e.DurationSeconds,
		&e.Completed, &e.LastWatched, &e.PlayCount)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *Repository) ContinueWatching(userID string, limit int) ([]WatchEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(`
		SELECT id, user_id, media_item_id, position_seconds, duration_seconds,
		       completed, last_watched, play_count
		FROM watch_history
		WHERE user_id=$1 AND completed=false AND position_seconds > 0
		ORDER BY last_watched DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WatchEntry
	for rows.Next() {
		var e WatchEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.MediaItemID, &e.PositionSeconds,
			&e.DurationSeconds, &e.Completed, &e.LastWatched, &e.PlayCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *Repository) History(userID string, limit int) ([]WatchEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(`
		SELECT id, user_id, media_item_id, position_seconds, duration_seconds,
		       completed, last_watched, play_count
		FROM watch_history WHERE user_id=$1
		ORDER BY last_watched DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WatchEntry
	for rows.Next() {
		var e WatchEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.MediaItemID, &e.PositionSeconds,
			&e.DurationSeconds, &e.Completed, &e.LastWatched, &e.PlayCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
