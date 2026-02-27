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

// GetEditionItemByMediaID returns the edition_item row for a given media item, or nil if none.
func (r *EditionRepository) GetEditionItemByMediaID(mediaItemID uuid.UUID) (*models.EditionItem, error) {
	item := &models.EditionItem{}
	query := `
		SELECT id, edition_group_id, media_item_id, edition_type, custom_edition_name,
		       quality_tier, display_name, is_default, sort_order, notes, added_at, added_by
		FROM edition_items WHERE media_item_id = $1 LIMIT 1`
	err := r.db.QueryRow(query, mediaItemID).Scan(
		&item.ID, &item.EditionGroupID, &item.MediaItemID,
		&item.EditionType, &item.CustomEditionName, &item.QualityTier,
		&item.DisplayName, &item.IsDefault, &item.SortOrder, &item.Notes,
		&item.AddedAt, &item.AddedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateEditionType updates the edition_type on an existing edition_item for a media item.
func (r *EditionRepository) UpdateEditionType(mediaItemID uuid.UUID, editionType string) error {
	result, err := r.db.Exec(
		`UPDATE edition_items SET edition_type = $1 WHERE media_item_id = $2`,
		editionType, mediaItemID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no edition item found for this media item")
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

// ──────────────────── Edition Parent/Child Helpers ────────────────────

// EditionMediaDetail holds edition item info with joined media item details.
type EditionMediaDetail struct {
	EditionItemID     uuid.UUID  `json:"edition_item_id"`
	EditionGroupID    uuid.UUID  `json:"edition_group_id"`
	MediaItemID       uuid.UUID  `json:"media_item_id"`
	EditionType       string     `json:"edition_type"`
	CustomEditionName *string    `json:"custom_edition_name,omitempty"`
	DisplayName       *string    `json:"display_name,omitempty"`
	IsDefault         bool       `json:"is_default"`
	SortOrder         int        `json:"sort_order"`
	// Joined media fields
	Title           string  `json:"title"`
	Resolution      *string `json:"resolution,omitempty"`
	Width           *int    `json:"width,omitempty"`
	Height          *int    `json:"height,omitempty"`
	DurationSeconds *int    `json:"duration_seconds,omitempty"`
	Codec           *string `json:"codec,omitempty"`
	Container       *string `json:"container,omitempty"`
	AudioCodec      *string `json:"audio_codec,omitempty"`
	AudioChannels   *int    `json:"audio_channels,omitempty"`
	FileSize        int64   `json:"file_size"`
	PosterPath      *string `json:"poster_path,omitempty"`
}

// ListItemsWithMedia returns edition items for a group with joined media details.
func (r *EditionRepository) ListItemsWithMedia(groupID uuid.UUID) ([]EditionMediaDetail, error) {
	query := `
		SELECT ei.id, ei.edition_group_id, ei.media_item_id, ei.edition_type,
		       ei.custom_edition_name, ei.display_name, ei.is_default, ei.sort_order,
		       m.title, m.resolution, m.width, m.height, m.duration_seconds,
		       m.codec, m.container, m.audio_codec, m.audio_channels,
		       m.file_size, m.poster_path
		FROM edition_items ei
		JOIN media_items m ON m.id = ei.media_item_id
		WHERE ei.edition_group_id = $1
		ORDER BY ei.is_default DESC, ei.sort_order`
	rows, err := r.db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []EditionMediaDetail
	for rows.Next() {
		var d EditionMediaDetail
		if err := rows.Scan(
			&d.EditionItemID, &d.EditionGroupID, &d.MediaItemID, &d.EditionType,
			&d.CustomEditionName, &d.DisplayName, &d.IsDefault, &d.SortOrder,
			&d.Title, &d.Resolution, &d.Width, &d.Height, &d.DurationSeconds,
			&d.Codec, &d.Container, &d.AudioCodec, &d.AudioChannels,
			&d.FileSize, &d.PosterPath,
		); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// GetGroupByMediaID finds the edition group containing a given media item.
func (r *EditionRepository) GetGroupByMediaID(mediaItemID uuid.UUID) (*uuid.UUID, error) {
	var groupID uuid.UUID
	err := r.db.QueryRow(
		`SELECT edition_group_id FROM edition_items WHERE media_item_id = $1 LIMIT 1`,
		mediaItemID).Scan(&groupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &groupID, nil
}

// SetParent links childID as a child edition of parentID.
// If the parent is already in a group, adds child to that group.
// Otherwise creates a new group with parent as default and adds child.
func (r *EditionRepository) SetParent(childID, parentID uuid.UUID, editionType string, userID uuid.UUID) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if child is already in a group — remove first
	_, _ = tx.Exec(`DELETE FROM edition_items WHERE media_item_id = $1`, childID)

	// Find parent's existing group
	var groupID uuid.UUID
	err = tx.QueryRow(
		`SELECT edition_group_id FROM edition_items WHERE media_item_id = $1 LIMIT 1`,
		parentID).Scan(&groupID)

	if err == sql.ErrNoRows {
		// Create new group using parent's metadata
		groupID = uuid.New()
		_, err = tx.Exec(`INSERT INTO edition_groups
			(id, library_id, media_type, title, sort_title, year, description, poster_path, backdrop_path)
			SELECT $1, library_id, media_type, title, sort_title, year, description, poster_path, backdrop_path
			FROM media_items WHERE id = $2`,
			groupID, parentID)
		if err != nil {
			return fmt.Errorf("create edition group: %w", err)
		}

		// Add parent as default item
		parentItemID := uuid.New()
		_, err = tx.Exec(`INSERT INTO edition_items
			(id, edition_group_id, media_item_id, edition_type, is_default, sort_order, added_by)
			VALUES ($1, $2, $3, (SELECT edition_type FROM media_items WHERE id = $3), true, 0, $4)`,
			parentItemID, groupID, parentID, userID)
		if err != nil {
			return fmt.Errorf("add parent to group: %w", err)
		}

		// Set default edition pointer
		_, _ = tx.Exec(`UPDATE edition_groups SET default_edition_id = $1 WHERE id = $2`,
			parentItemID, groupID)
	} else if err != nil {
		return err
	}

	// Determine sort order
	var sortOrder int
	tx.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM edition_items WHERE edition_group_id = $1`,
		groupID).Scan(&sortOrder)

	// Add child to group
	if editionType == "" {
		editionType = "Alternate"
	}
	childItemID := uuid.New()
	_, err = tx.Exec(`INSERT INTO edition_items
		(id, edition_group_id, media_item_id, edition_type, is_default, sort_order, added_by)
		VALUES ($1, $2, $3, $4, false, $5, $6)
		ON CONFLICT (edition_group_id, media_item_id) DO NOTHING`,
		childItemID, groupID, childID, editionType, sortOrder, userID)
	if err != nil {
		return fmt.Errorf("add child to group: %w", err)
	}

	return tx.Commit()
}

// RemoveFromGroup removes a media item from its edition group.
// If the group drops to 1 or 0 items, deletes the group entirely.
func (r *EditionRepository) RemoveFromGroup(mediaItemID uuid.UUID) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Find the group
	var groupID uuid.UUID
	err = tx.QueryRow(
		`SELECT edition_group_id FROM edition_items WHERE media_item_id = $1 LIMIT 1`,
		mediaItemID).Scan(&groupID)
	if err == sql.ErrNoRows {
		return nil // not in a group
	}
	if err != nil {
		return err
	}

	// Remove the item
	_, err = tx.Exec(`DELETE FROM edition_items WHERE media_item_id = $1`, mediaItemID)
	if err != nil {
		return err
	}

	// Check remaining count
	var remaining int
	tx.QueryRow(`SELECT COUNT(*) FROM edition_items WHERE edition_group_id = $1`, groupID).Scan(&remaining)

	if remaining <= 1 {
		// Delete the remaining item and the group
		_, _ = tx.Exec(`DELETE FROM edition_items WHERE edition_group_id = $1`, groupID)
		_, _ = tx.Exec(`DELETE FROM edition_groups WHERE id = $1`, groupID)
	}

	return tx.Commit()
}
