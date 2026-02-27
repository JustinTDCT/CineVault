package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type GalleryRepository struct {
	db *sql.DB
}

func NewGalleryRepository(db *sql.DB) *GalleryRepository {
	return &GalleryRepository{db: db}
}

func (r *GalleryRepository) Create(g *models.ImageGallery) error {
	query := `
		INSERT INTO image_galleries (id, library_id, title, description, poster_path, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, g.ID, g.LibraryID, g.Title, g.Description,
		g.PosterPath, g.SortPosition).
		Scan(&g.CreatedAt, &g.UpdatedAt)
}

func (r *GalleryRepository) GetByID(id uuid.UUID) (*models.ImageGallery, error) {
	g := &models.ImageGallery{}
	query := `
		SELECT id, library_id, title, description, poster_path, sort_position,
		       created_at, updated_at
		FROM image_galleries WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&g.ID, &g.LibraryID, &g.Title, &g.Description, &g.PosterPath,
		&g.SortPosition, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("gallery not found")
	}
	return g, err
}

func (r *GalleryRepository) ListByLibrary(libraryID uuid.UUID) ([]*models.ImageGallery, error) {
	query := `
		SELECT id, library_id, title, description, poster_path, sort_position,
		       created_at, updated_at
		FROM image_galleries WHERE library_id = $1 ORDER BY title`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var galleries []*models.ImageGallery
	for rows.Next() {
		g := &models.ImageGallery{}
		if err := rows.Scan(&g.ID, &g.LibraryID, &g.Title, &g.Description,
			&g.PosterPath, &g.SortPosition, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		galleries = append(galleries, g)
	}
	return galleries, rows.Err()
}

func (r *GalleryRepository) FindByTitle(libraryID uuid.UUID, title string) (*models.ImageGallery, error) {
	g := &models.ImageGallery{}
	query := `
		SELECT id, library_id, title, description, poster_path, sort_position,
		       created_at, updated_at
		FROM image_galleries WHERE library_id = $1 AND LOWER(title) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, libraryID, title).Scan(
		&g.ID, &g.LibraryID, &g.Title, &g.Description, &g.PosterPath,
		&g.SortPosition, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (r *GalleryRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM image_galleries WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("gallery not found")
	}
	return nil
}
