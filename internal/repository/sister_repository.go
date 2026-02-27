package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type SisterRepository struct {
	db *sql.DB
}

func NewSisterRepository(db *sql.DB) *SisterRepository {
	return &SisterRepository{db: db}
}

func (r *SisterRepository) Create(g *models.SisterGroup) error {
	query := `
		INSERT INTO sister_groups (id, name, notes, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`
	return r.db.QueryRow(query, g.ID, g.Name, g.Notes, g.CreatedBy).
		Scan(&g.CreatedAt)
}

func (r *SisterRepository) GetByID(id uuid.UUID) (*models.SisterGroup, error) {
	g := &models.SisterGroup{}
	query := `
		SELECT id, name, notes, created_at, created_by
		FROM sister_groups WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&g.ID, &g.Name, &g.Notes, &g.CreatedAt, &g.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sister group not found")
	}
	if err != nil {
		return nil, err
	}

	// Load members
	members, err := r.ListMembers(id)
	if err != nil {
		return nil, err
	}
	g.Members = members
	return g, nil
}

func (r *SisterRepository) List(limit, offset int) ([]*models.SisterGroup, error) {
	query := `
		SELECT id, name, notes, created_at, created_by
		FROM sister_groups ORDER BY name
		LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*models.SisterGroup
	for rows.Next() {
		g := &models.SisterGroup{}
		if err := rows.Scan(&g.ID, &g.Name, &g.Notes, &g.CreatedAt, &g.CreatedBy); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (r *SisterRepository) Delete(id uuid.UUID) error {
	// First unset sister_group_id on all members
	if _, err := r.db.Exec(
		`UPDATE media_items SET sister_group_id = NULL WHERE sister_group_id = $1`, id); err != nil {
		return err
	}
	result, err := r.db.Exec(`DELETE FROM sister_groups WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("sister group not found")
	}
	return nil
}

// ──────────────────── Members ────────────────────

func (r *SisterRepository) AddMember(groupID, mediaItemID uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE media_items SET sister_group_id = $1 WHERE id = $2`,
		groupID, mediaItemID)
	return err
}

func (r *SisterRepository) AddMemberWithPosition(groupID, mediaItemID uuid.UUID, position int) error {
	_, err := r.db.Exec(
		`UPDATE media_items SET sister_group_id = $1, sort_position = $2 WHERE id = $3`,
		groupID, position, mediaItemID)
	return err
}

func (r *SisterRepository) RemoveMember(groupID, mediaItemID uuid.UUID) error {
	result, err := r.db.Exec(
		`UPDATE media_items SET sister_group_id = NULL WHERE id = $1 AND sister_group_id = $2`,
		mediaItemID, groupID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("media item not in this sister group")
	}
	return nil
}

func (r *SisterRepository) ListMembers(groupID uuid.UUID) ([]*models.MediaItem, error) {
	query := `
		SELECT id, library_id, media_type, file_path, file_name, file_size, title,
		       duration_seconds, resolution, width, height, codec, container,
		       poster_path, year, rating, added_at, updated_at
		FROM media_items WHERE sister_group_id = $1 ORDER BY sort_position ASC, title ASC`
	rows, err := r.db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.MediaType,
			&item.FilePath, &item.FileName, &item.FileSize, &item.Title,
			&item.DurationSeconds, &item.Resolution, &item.Width, &item.Height,
			&item.Codec, &item.Container, &item.PosterPath,
			&item.Year, &item.Rating, &item.AddedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
