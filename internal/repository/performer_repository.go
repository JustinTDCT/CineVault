package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type PerformerRepository struct {
	db *sql.DB
}

func NewPerformerRepository(db *sql.DB) *PerformerRepository {
	return &PerformerRepository{db: db}
}

func (r *PerformerRepository) Create(p *models.Performer) error {
	query := `INSERT INTO performers (id, name, sort_name, performer_type, photo_path, bio, birth_date, death_date, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, p.ID, p.Name, p.SortName, p.PerformerType,
		p.PhotoPath, p.Bio, p.BirthDate, p.DeathDate, p.SortPosition).
		Scan(&p.CreatedAt, &p.UpdatedAt)
}

func (r *PerformerRepository) GetByID(id uuid.UUID) (*models.Performer, error) {
	p := &models.Performer{}
	query := `SELECT p.id, p.name, p.sort_name, p.performer_type, p.photo_path, p.bio, p.birth_date, p.death_date,
		p.sort_position, p.created_at, p.updated_at,
		COALESCE((SELECT COUNT(*) FROM media_performers mp WHERE mp.performer_id = p.id), 0) as media_count
		FROM performers p WHERE p.id = $1`
	err := r.db.QueryRow(query, id).Scan(&p.ID, &p.Name, &p.SortName, &p.PerformerType,
		&p.PhotoPath, &p.Bio, &p.BirthDate, &p.DeathDate,
		&p.SortPosition, &p.CreatedAt, &p.UpdatedAt, &p.MediaCount)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("performer not found")
	}
	return p, err
}

func (r *PerformerRepository) List(search string, limit, offset int) ([]*models.Performer, error) {
	query := `SELECT p.id, p.name, p.sort_name, p.performer_type, p.photo_path, p.bio, p.birth_date, p.death_date,
		p.sort_position, p.created_at, p.updated_at,
		COALESCE((SELECT COUNT(*) FROM media_performers mp WHERE mp.performer_id = p.id), 0) as media_count
		FROM performers p`
	var args []interface{}
	argIdx := 1

	if search != "" {
		query += fmt.Sprintf(` WHERE p.name ILIKE $%d`, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY COALESCE(p.sort_name, p.name) LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var performers []*models.Performer
	for rows.Next() {
		p := &models.Performer{}
		if err := rows.Scan(&p.ID, &p.Name, &p.SortName, &p.PerformerType,
			&p.PhotoPath, &p.Bio, &p.BirthDate, &p.DeathDate,
			&p.SortPosition, &p.CreatedAt, &p.UpdatedAt, &p.MediaCount); err != nil {
			return nil, err
		}
		performers = append(performers, p)
	}
	return performers, rows.Err()
}

func (r *PerformerRepository) Update(p *models.Performer) error {
	query := `UPDATE performers SET name=$1, sort_name=$2, performer_type=$3, photo_path=$4, bio=$5,
		birth_date=$6, death_date=$7, sort_position=$8, updated_at=CURRENT_TIMESTAMP
		WHERE id=$9`
	result, err := r.db.Exec(query, p.Name, p.SortName, p.PerformerType,
		p.PhotoPath, p.Bio, p.BirthDate, p.DeathDate, p.SortPosition, p.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("performer not found")
	}
	return nil
}

func (r *PerformerRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM performers WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("performer not found")
	}
	return nil
}

func (r *PerformerRepository) LinkMedia(mediaItemID, performerID uuid.UUID, role, characterName string, sortOrder int) error {
	query := `INSERT INTO media_performers (id, media_item_id, performer_id, role, character_name, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (media_item_id, performer_id, role) DO UPDATE SET character_name=$5, sort_order=$6`
	_, err := r.db.Exec(query, uuid.New(), mediaItemID, performerID, role, characterName, sortOrder)
	return err
}

func (r *PerformerRepository) UnlinkMedia(mediaItemID, performerID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_performers WHERE media_item_id=$1 AND performer_id=$2`,
		mediaItemID, performerID)
	return err
}

func (r *PerformerRepository) GetMediaPerformers(mediaItemID uuid.UUID) ([]*models.Performer, error) {
	query := `SELECT p.id, p.name, p.sort_name, p.performer_type, p.photo_path, p.bio, p.birth_date, p.death_date,
		p.sort_position, p.created_at, p.updated_at, 0 as media_count
		FROM performers p JOIN media_performers mp ON p.id = mp.performer_id
		WHERE mp.media_item_id = $1 ORDER BY mp.sort_order`
	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var performers []*models.Performer
	for rows.Next() {
		p := &models.Performer{}
		if err := rows.Scan(&p.ID, &p.Name, &p.SortName, &p.PerformerType,
			&p.PhotoPath, &p.Bio, &p.BirthDate, &p.DeathDate,
			&p.SortPosition, &p.CreatedAt, &p.UpdatedAt, &p.MediaCount); err != nil {
			return nil, err
		}
		performers = append(performers, p)
	}
	return performers, rows.Err()
}

// GetMediaCast returns performers linked to a media item with their role and character info.
func (r *PerformerRepository) GetMediaCast(mediaItemID uuid.UUID) ([]*models.CastMember, error) {
	query := `SELECT p.id, p.name, p.performer_type, p.photo_path,
		mp.role, mp.character_name, mp.sort_order
		FROM performers p JOIN media_performers mp ON p.id = mp.performer_id
		WHERE mp.media_item_id = $1 ORDER BY mp.sort_order`
	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cast []*models.CastMember
	for rows.Next() {
		c := &models.CastMember{}
		if err := rows.Scan(&c.PerformerID, &c.Name, &c.PerformerType,
			&c.PhotoPath, &c.Role, &c.CharacterName, &c.SortOrder); err != nil {
			return nil, err
		}
		cast = append(cast, c)
	}
	return cast, rows.Err()
}

// FindByName returns a performer matching the exact name (case-insensitive), or nil if not found.
func (r *PerformerRepository) FindByName(name string) (*models.Performer, error) {
	p := &models.Performer{}
	query := `SELECT p.id, p.name, p.sort_name, p.performer_type, p.photo_path, p.bio, p.birth_date, p.death_date,
		p.sort_position, p.created_at, p.updated_at, 0 as media_count
		FROM performers p WHERE LOWER(p.name) = LOWER($1) LIMIT 1`
	err := r.db.QueryRow(query, name).Scan(&p.ID, &p.Name, &p.SortName, &p.PerformerType,
		&p.PhotoPath, &p.Bio, &p.BirthDate, &p.DeathDate,
		&p.SortPosition, &p.CreatedAt, &p.UpdatedAt, &p.MediaCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PerformerRepository) GetPerformerMedia(performerID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + ` FROM media_items
		WHERE id IN (SELECT media_item_id FROM media_performers WHERE performer_id = $1)
		ORDER BY title`
	rows, err := r.db.Query(query, performerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item, err := scanMediaItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
