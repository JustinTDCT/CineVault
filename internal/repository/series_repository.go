package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type SeriesRepository struct {
	db *sql.DB
}

func NewSeriesRepository(db *sql.DB) *SeriesRepository {
	return &SeriesRepository{db: db}
}

// ──────────────────── Movie Series ────────────────────

func (r *SeriesRepository) Create(s *models.MovieSeries) error {
	query := `
		INSERT INTO movie_series (id, library_id, name, poster_path)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, s.ID, s.LibraryID, s.Name, s.PosterPath).
		Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *SeriesRepository) GetByID(id uuid.UUID) (*models.MovieSeries, error) {
	s := &models.MovieSeries{}
	query := `
		SELECT id, library_id, name, poster_path, created_at, updated_at
		FROM movie_series WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&s.ID, &s.LibraryID, &s.Name, &s.PosterPath,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("series not found")
	}
	if err != nil {
		return nil, err
	}

	// Load items with media details
	items, err := r.ListItems(id)
	if err != nil {
		return nil, err
	}
	s.Items = items
	s.ItemCount = len(items)
	return s, nil
}

func (r *SeriesRepository) ListByLibrary(libraryID uuid.UUID) ([]*models.MovieSeries, error) {
	query := `
		SELECT s.id, s.library_id, s.name, s.poster_path, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM movie_series_items si WHERE si.series_id = s.id) as item_count
		FROM movie_series s
		WHERE s.library_id = $1
		ORDER BY s.name`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.MovieSeries
	for rows.Next() {
		s := &models.MovieSeries{}
		if err := rows.Scan(&s.ID, &s.LibraryID, &s.Name, &s.PosterPath,
			&s.CreatedAt, &s.UpdatedAt, &s.ItemCount); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *SeriesRepository) Update(s *models.MovieSeries) error {
	query := `
		UPDATE movie_series
		SET name = $1, poster_path = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3`
	result, err := r.db.Exec(query, s.Name, s.PosterPath, s.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series not found")
	}
	return nil
}

func (r *SeriesRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM movie_series WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series not found")
	}
	return nil
}

// ──────────────────── Series Items ────────────────────

func (r *SeriesRepository) AddItem(item *models.MovieSeriesItem) error {
	query := `
		INSERT INTO movie_series_items (id, series_id, media_item_id, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING added_at`
	return r.db.QueryRow(query, item.ID, item.SeriesID, item.MediaItemID, item.SortOrder).
		Scan(&item.AddedAt)
}

func (r *SeriesRepository) UpdateItemOrder(id uuid.UUID, sortOrder int) error {
	result, err := r.db.Exec(
		`UPDATE movie_series_items SET sort_order = $1 WHERE id = $2`,
		sortOrder, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series item not found")
	}
	return nil
}

func (r *SeriesRepository) ListItems(seriesID uuid.UUID) ([]models.MovieSeriesItem, error) {
	query := `
		SELECT si.id, si.series_id, si.media_item_id, si.sort_order, si.added_at,
		       m.title, m.year, m.poster_path
		FROM movie_series_items si
		JOIN media_items m ON m.id = si.media_item_id
		WHERE si.series_id = $1
		ORDER BY si.sort_order`
	rows, err := r.db.Query(query, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MovieSeriesItem
	for rows.Next() {
		item := models.MovieSeriesItem{}
		if err := rows.Scan(&item.ID, &item.SeriesID, &item.MediaItemID,
			&item.SortOrder, &item.AddedAt,
			&item.Title, &item.Year, &item.PosterPath); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SeriesRepository) RemoveItem(seriesID, itemID uuid.UUID) error {
	result, err := r.db.Exec(
		`DELETE FROM movie_series_items WHERE series_id = $1 AND id = $2`,
		seriesID, itemID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series item not found")
	}
	return nil
}

// GetSeriesForMedia returns the series a media item belongs to (if any)
func (r *SeriesRepository) GetSeriesForMedia(mediaItemID uuid.UUID) (*models.MovieSeries, *models.MovieSeriesItem, error) {
	item := &models.MovieSeriesItem{}
	query := `
		SELECT si.id, si.series_id, si.media_item_id, si.sort_order, si.added_at
		FROM movie_series_items si
		WHERE si.media_item_id = $1
		LIMIT 1`
	err := r.db.QueryRow(query, mediaItemID).Scan(
		&item.ID, &item.SeriesID, &item.MediaItemID, &item.SortOrder, &item.AddedAt)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	s := &models.MovieSeries{}
	sQuery := `
		SELECT id, library_id, name, poster_path, created_at, updated_at
		FROM movie_series WHERE id = $1`
	err = r.db.QueryRow(sQuery, item.SeriesID).Scan(
		&s.ID, &s.LibraryID, &s.Name, &s.PosterPath, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, nil, err
	}
	return s, item, nil
}
