package repository

import (
	"database/sql"
	"fmt"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Create(item *models.MediaItem) error {
	query := `
		INSERT INTO media_items (id, library_id, media_type, file_path, file_name, file_size,
		                         title, duration_seconds, resolution, width, height, codec, container)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING added_at, updated_at`
	
	return r.db.QueryRow(query, item.ID, item.LibraryID, item.MediaType, item.FilePath,
		item.FileName, item.FileSize, item.Title, item.DurationSeconds, item.Resolution,
		item.Width, item.Height, item.Codec, item.Container).
		Scan(&item.AddedAt, &item.UpdatedAt)
}

func (r *MediaRepository) GetByID(id uuid.UUID) (*models.MediaItem, error) {
	item := &models.MediaItem{}
	query := `
		SELECT id, library_id, media_type, file_path, file_name, file_size, title,
		       duration_seconds, resolution, width, height, codec, container,
		       added_at, updated_at
		FROM media_items WHERE id = $1`
	
	err := r.db.QueryRow(query, id).Scan(
		&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName,
		&item.FileSize, &item.Title, &item.DurationSeconds, &item.Resolution,
		&item.Width, &item.Height, &item.Codec, &item.Container,
		&item.AddedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("media item not found")
	}
	return item, err
}

func (r *MediaRepository) ListByLibrary(libraryID uuid.UUID, limit, offset int) ([]*models.MediaItem, error) {
	query := `
		SELECT id, library_id, media_type, file_path, file_name, file_size, title,
		       duration_seconds, resolution, width, height, codec, container,
		       added_at, updated_at
		FROM media_items 
		WHERE library_id = $1
		ORDER BY added_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.Query(query, libraryID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*models.MediaItem{}
	for rows.Next() {
		item := &models.MediaItem{}
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath,
			&item.FileName, &item.FileSize, &item.Title, &item.DurationSeconds,
			&item.Resolution, &item.Width, &item.Height, &item.Codec, &item.Container,
			&item.AddedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *MediaRepository) Search(query string, limit int) ([]*models.MediaItem, error) {
	searchQuery := `
		SELECT id, library_id, media_type, file_path, file_name, file_size, title,
		       duration_seconds, resolution, width, height, codec, container,
		       added_at, updated_at
		FROM media_items 
		WHERE title ILIKE $1 OR file_name ILIKE $1
		ORDER BY added_at DESC
		LIMIT $2`
	
	rows, err := r.db.Query(searchQuery, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*models.MediaItem{}
	for rows.Next() {
		item := &models.MediaItem{}
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath,
			&item.FileName, &item.FileSize, &item.Title, &item.DurationSeconds,
			&item.Resolution, &item.Width, &item.Height, &item.Codec, &item.Container,
			&item.AddedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *MediaRepository) CountByLibrary(libraryID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM media_items WHERE library_id = $1`
	err := r.db.QueryRow(query, libraryID).Scan(&count)
	return count, err
}

func (r *MediaRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM media_items WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("media item not found")
	}
	return nil
}
