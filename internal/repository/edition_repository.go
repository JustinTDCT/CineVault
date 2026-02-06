package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type EditionRepository struct {
	db *sql.DB
}

func NewEditionRepository(db *sql.DB) *EditionRepository {
	return &EditionRepository{db: db}
}

// ──────────────────── Edition Groups ────────────────────

func (r *EditionRepository) CreateGroup(g *models.EditionGroup) error {
	query := `
		INSERT INTO edition_groups (id, library_id, media_type, title, sort_title, year,
		                            description, poster_path, backdrop_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, g.ID, g.LibraryID, g.MediaType, g.Title, g.SortTitle,
		g.Year, g.Description, g.PosterPath, g.BackdropPath).
		Scan(&g.CreatedAt, &g.UpdatedAt)
}

func (r *EditionRepository) GetGroupByID(id uuid.UUID) (*models.EditionGroup, error) {
	g := &models.EditionGroup{}
	query := `
		SELECT id, library_id, media_type, title, sort_title, year, description,
		       poster_path, backdrop_path, default_edition_id, created_at, updated_at
		FROM edition_groups WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&g.ID, &g.LibraryID, &g.MediaType, &g.Title, &g.SortTitle, &g.Year,
		&g.Description, &g.PosterPath, &g.BackdropPath, &g.DefaultEditionID,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("edition group not found")
	}
	if err != nil {
		return nil, err
	}

	// Load items
	items, err := r.ListItems(id)
	if err != nil {
		return nil, err
	}
	g.Items = items
	return g, nil
}

func (r *EditionRepository) ListGroups(limit, offset int) ([]*models.EditionGroup, error) {
	query := `
		SELECT id, library_id, media_type, title, sort_title, year, description,
		       poster_path, backdrop_path, default_edition_id, created_at, updated_at
		FROM edition_groups ORDER BY COALESCE(sort_title, title)
		LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*models.EditionGroup
	for rows.Next() {
		g := &models.EditionGroup{}
		if err := rows.Scan(&g.ID, &g.LibraryID, &g.MediaType, &g.Title, &g.SortTitle,
			&g.Year, &g.Description, &g.PosterPath, &g.BackdropPath,
			&g.DefaultEditionID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (r *EditionRepository) UpdateGroup(g *models.EditionGroup) error {
	query := `
		UPDATE edition_groups
		SET title = $1, sort_title = $2, year = $3, description = $4,
		    poster_path = $5, backdrop_path = $6, default_edition_id = $7,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $8`
	result, err := r.db.Exec(query, g.Title, g.SortTitle, g.Year, g.Description,
		g.PosterPath, g.BackdropPath, g.DefaultEditionID, g.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("edition group not found")
	}
	return nil
}

func (r *EditionRepository) DeleteGroup(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM edition_groups WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("edition group not found")
	}
	return nil
}

// ──────────────────── Edition Items ────────────────────

func (r *EditionRepository) AddItem(item *models.EditionItem) error {
	query := `
		INSERT INTO edition_items (id, edition_group_id, media_item_id, edition_type,
		                           custom_edition_name, quality_tier, display_name,
		                           is_default, sort_order, notes, added_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING added_at`
	return r.db.QueryRow(query, item.ID, item.EditionGroupID, item.MediaItemID,
		item.EditionType, item.CustomEditionName, item.QualityTier, item.DisplayName,
		item.IsDefault, item.SortOrder, item.Notes, item.AddedBy).
		Scan(&item.AddedAt)
}

func (r *EditionRepository) ListItems(groupID uuid.UUID) ([]models.EditionItem, error) {
	query := `
		SELECT id, edition_group_id, media_item_id, edition_type, custom_edition_name,
		       quality_tier, display_name, is_default, sort_order, notes, added_at, added_by
		FROM edition_items WHERE edition_group_id = $1 ORDER BY sort_order`
	rows, err := r.db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.EditionItem
	for rows.Next() {
		item := models.EditionItem{}
		if err := rows.Scan(&item.ID, &item.EditionGroupID, &item.MediaItemID,
			&item.EditionType, &item.CustomEditionName, &item.QualityTier,
			&item.DisplayName, &item.IsDefault, &item.SortOrder, &item.Notes,
			&item.AddedAt, &item.AddedBy); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *EditionRepository) RemoveItem(groupID, itemID uuid.UUID) error {
	result, err := r.db.Exec(
		`DELETE FROM edition_items WHERE edition_group_id = $1 AND id = $2`,
		groupID, itemID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("edition item not found")
	}
	return nil
}

func (r *EditionRepository) SetDefault(groupID, itemID uuid.UUID) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset all defaults
	if _, err := tx.Exec(
		`UPDATE edition_items SET is_default = false WHERE edition_group_id = $1`, groupID); err != nil {
		return err
	}
	// Set new default
	if _, err := tx.Exec(
		`UPDATE edition_items SET is_default = true WHERE edition_group_id = $1 AND id = $2`,
		groupID, itemID); err != nil {
		return err
	}
	// Update group default pointer
	if _, err := tx.Exec(
		`UPDATE edition_groups SET default_edition_id = $1 WHERE id = $2`,
		itemID, groupID); err != nil {
		return err
	}
	return tx.Commit()
}
