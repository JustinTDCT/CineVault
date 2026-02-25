package media

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(m *MediaItem) error {
	if m.Metadata == nil {
		m.Metadata = json.RawMessage("{}")
	}
	return r.db.QueryRow(`
		INSERT INTO media_items (library_id, cache_id, parent_id, title, original_title, sort_title,
		       description, release_date, release_year, runtime_minutes, file_path, file_size,
		       file_hash, file_mod_time, video_codec, audio_codec, resolution, bitrate, phash,
		       match_confidence, metadata, season_number, episode_number)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
		RETURNING id, date_added, date_modified`,
		m.LibraryID, m.CacheID, m.ParentID, m.Title, m.OriginalTitle, m.SortTitle,
		m.Description, m.ReleaseDate, m.ReleaseYear, m.RuntimeMinutes, m.FilePath, m.FileSize,
		m.FileHash, m.FileModTime, m.VideoCodec, m.AudioCodec, m.Resolution, m.Bitrate, m.PHash,
		m.MatchConfidence, m.Metadata, m.SeasonNumber, m.EpisodeNumber,
	).Scan(&m.ID, &m.DateAdded, &m.DateModified)
}

func (r *Repository) GetByID(id string) (*MediaItem, error) {
	m := &MediaItem{}
	err := r.db.QueryRow(`
		SELECT id, library_id, cache_id, parent_id, title, original_title, sort_title,
		       description, release_date, release_year, runtime_minutes, file_path, file_size,
		       file_hash, file_mod_time, video_codec, audio_codec, resolution, bitrate, phash,
		       match_confidence, metadata_locked, manual_override_fields, metadata,
		       season_number, episode_number, date_added, date_modified
		FROM media_items WHERE id=$1`, id,
	).Scan(&m.ID, &m.LibraryID, &m.CacheID, &m.ParentID, &m.Title, &m.OriginalTitle, &m.SortTitle,
		&m.Description, &m.ReleaseDate, &m.ReleaseYear, &m.RuntimeMinutes, &m.FilePath, &m.FileSize,
		&m.FileHash, &m.FileModTime, &m.VideoCodec, &m.AudioCodec, &m.Resolution, &m.Bitrate, &m.PHash,
		&m.MatchConfidence, &m.MetadataLocked, pq.Array(&m.ManualOverrideFields), &m.Metadata,
		&m.SeasonNumber, &m.EpisodeNumber, &m.DateAdded, &m.DateModified)
	if err != nil {
		return nil, fmt.Errorf("media item not found: %w", err)
	}
	return m, nil
}

func (r *Repository) GetByFilePath(filePath string) (*MediaItem, error) {
	m := &MediaItem{}
	err := r.db.QueryRow(`
		SELECT id, library_id, file_path, file_size, file_hash, file_mod_time
		FROM media_items WHERE file_path=$1`, filePath,
	).Scan(&m.ID, &m.LibraryID, &m.FilePath, &m.FileSize, &m.FileHash, &m.FileModTime)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *Repository) ListByLibrary(params ListParams) ([]MediaItem, error) {
	if params.Limit <= 0 || params.Limit > 200 {
		params.Limit = 50
	}
	if params.SortBy == "" {
		params.SortBy = "sort_title"
	}
	if params.SortDir == "" {
		params.SortDir = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT id, library_id, cache_id, parent_id, title, sort_title, release_year,
		       runtime_minutes, file_path, resolution, match_confidence, metadata,
		       season_number, episode_number, date_added
		FROM media_items
		WHERE library_id=$1 AND parent_id IS NULL
		ORDER BY %s %s
		LIMIT $2`, params.SortBy, params.SortDir)

	rows, err := r.db.Query(query, params.LibraryID, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaItem
	for rows.Next() {
		var m MediaItem
		if err := rows.Scan(&m.ID, &m.LibraryID, &m.CacheID, &m.ParentID, &m.Title, &m.SortTitle,
			&m.ReleaseYear, &m.RuntimeMinutes, &m.FilePath, &m.Resolution, &m.MatchConfidence,
			&m.Metadata, &m.SeasonNumber, &m.EpisodeNumber, &m.DateAdded); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Repository) ListChildren(parentID string) ([]MediaItem, error) {
	rows, err := r.db.Query(`
		SELECT id, library_id, cache_id, parent_id, title, sort_title, release_year,
		       runtime_minutes, file_path, resolution, metadata,
		       season_number, episode_number, date_added
		FROM media_items WHERE parent_id=$1
		ORDER BY season_number, episode_number`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaItem
	for rows.Next() {
		var m MediaItem
		if err := rows.Scan(&m.ID, &m.LibraryID, &m.CacheID, &m.ParentID, &m.Title, &m.SortTitle,
			&m.ReleaseYear, &m.RuntimeMinutes, &m.FilePath, &m.Resolution, &m.Metadata,
			&m.SeasonNumber, &m.EpisodeNumber, &m.DateAdded); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Repository) UpdateMetadata(id string, cacheID *string, confidence float64, metadata json.RawMessage) error {
	_, err := r.db.Exec(`
		UPDATE media_items SET cache_id=$2, match_confidence=$3, metadata=$4, date_modified=NOW()
		WHERE id=$1`, id, cacheID, confidence, metadata)
	return err
}

func (r *Repository) UpdateTechnical(id string, videoCodec, audioCodec, resolution *string, bitrate *int, fileSize *int64) error {
	_, err := r.db.Exec(`
		UPDATE media_items SET video_codec=$2, audio_codec=$3, resolution=$4, bitrate=$5, file_size=$6, date_modified=NOW()
		WHERE id=$1`, id, videoCodec, audioCodec, resolution, bitrate, fileSize)
	return err
}

func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM media_items WHERE id=$1", id)
	return err
}

func (r *Repository) DeleteByLibrary(libraryID string) error {
	_, err := r.db.Exec("DELETE FROM media_items WHERE library_id=$1", libraryID)
	return err
}

func (r *Repository) CountByLibrary(libraryID string) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM media_items WHERE library_id=$1", libraryID).Scan(&count)
	return count, err
}

func (r *Repository) Search(query string, limit int) ([]MediaItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := r.db.Query(`
		SELECT id, library_id, title, sort_title, release_year, runtime_minutes,
		       resolution, metadata, date_added,
		       ts_rank(search_vector, plainto_tsquery('english', $1)) as rank
		FROM media_items
		WHERE search_vector @@ plainto_tsquery('english', $1)
		ORDER BY rank DESC LIMIT $2`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaItem
	for rows.Next() {
		var m MediaItem
		var rank float64
		if err := rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.SortTitle, &m.ReleaseYear,
			&m.RuntimeMinutes, &m.Resolution, &m.Metadata, &m.DateAdded, &rank); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Repository) FindDuplicates(phash string, threshold int) ([]MediaItem, error) {
	rows, err := r.db.Query(`
		SELECT id, library_id, title, file_path, phash, resolution
		FROM media_items WHERE phash IS NOT NULL AND phash=$1`, phash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MediaItem
	for rows.Next() {
		var m MediaItem
		if err := rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.FilePath, &m.PHash, &m.Resolution); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
