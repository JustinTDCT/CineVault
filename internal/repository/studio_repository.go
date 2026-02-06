package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type StudioRepository struct {
	db *sql.DB
}

func NewStudioRepository(db *sql.DB) *StudioRepository {
	return &StudioRepository{db: db}
}

func (r *StudioRepository) Create(s *models.Studio) error {
	query := `INSERT INTO studios (id, name, studio_type, logo_path, description, website, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at, updated_at`
	return r.db.QueryRow(query, s.ID, s.Name, s.StudioType, s.LogoPath,
		s.Description, s.Website, s.SortPosition).Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *StudioRepository) GetByID(id uuid.UUID) (*models.Studio, error) {
	s := &models.Studio{}
	query := `SELECT s.id, s.name, s.studio_type, s.logo_path, s.description, s.website,
		s.sort_position, s.created_at, s.updated_at,
		COALESCE((SELECT COUNT(*) FROM media_studios ms WHERE ms.studio_id = s.id), 0) as media_count
		FROM studios s WHERE s.id = $1`
	err := r.db.QueryRow(query, id).Scan(&s.ID, &s.Name, &s.StudioType, &s.LogoPath,
		&s.Description, &s.Website, &s.SortPosition, &s.CreatedAt, &s.UpdatedAt, &s.MediaCount)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("studio not found")
	}
	return s, err
}

func (r *StudioRepository) List(studioType string, limit, offset int) ([]*models.Studio, error) {
	query := `SELECT s.id, s.name, s.studio_type, s.logo_path, s.description, s.website,
		s.sort_position, s.created_at, s.updated_at,
		COALESCE((SELECT COUNT(*) FROM media_studios ms WHERE ms.studio_id = s.id), 0) as media_count
		FROM studios s`
	var args []interface{}
	argIdx := 1
	if studioType != "" {
		query += fmt.Sprintf(` WHERE s.studio_type = $%d`, argIdx)
		args = append(args, studioType)
		argIdx++
	}
	query += fmt.Sprintf(` ORDER BY s.name LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var studios []*models.Studio
	for rows.Next() {
		s := &models.Studio{}
		if err := rows.Scan(&s.ID, &s.Name, &s.StudioType, &s.LogoPath,
			&s.Description, &s.Website, &s.SortPosition, &s.CreatedAt, &s.UpdatedAt, &s.MediaCount); err != nil {
			return nil, err
		}
		studios = append(studios, s)
	}
	return studios, rows.Err()
}

func (r *StudioRepository) Update(s *models.Studio) error {
	query := `UPDATE studios SET name=$1, studio_type=$2, logo_path=$3, description=$4, website=$5,
		sort_position=$6, updated_at=CURRENT_TIMESTAMP WHERE id=$7`
	result, err := r.db.Exec(query, s.Name, s.StudioType, s.LogoPath,
		s.Description, s.Website, s.SortPosition, s.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("studio not found")
	}
	return nil
}

func (r *StudioRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM studios WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("studio not found")
	}
	return nil
}

func (r *StudioRepository) LinkMedia(mediaItemID, studioID uuid.UUID, role string) error {
	query := `INSERT INTO media_studios (id, media_item_id, studio_id, role) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(query, uuid.New(), mediaItemID, studioID, role)
	return err
}

func (r *StudioRepository) UnlinkMedia(mediaItemID, studioID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_studios WHERE media_item_id=$1 AND studio_id=$2`, mediaItemID, studioID)
	return err
}

func (r *StudioRepository) GetMediaStudios(mediaItemID uuid.UUID) ([]*models.Studio, error) {
	query := `SELECT s.id, s.name, s.studio_type, s.logo_path, s.description, s.website,
		s.sort_position, s.created_at, s.updated_at, 0 as media_count
		FROM studios s JOIN media_studios ms ON s.id = ms.studio_id
		WHERE ms.media_item_id = $1 ORDER BY s.name`
	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var studios []*models.Studio
	for rows.Next() {
		s := &models.Studio{}
		if err := rows.Scan(&s.ID, &s.Name, &s.StudioType, &s.LogoPath,
			&s.Description, &s.Website, &s.SortPosition, &s.CreatedAt, &s.UpdatedAt, &s.MediaCount); err != nil {
			return nil, err
		}
		studios = append(studios, s)
	}
	return studios, rows.Err()
}
