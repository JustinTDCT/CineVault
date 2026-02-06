package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type CollectionRepository struct {
	db *sql.DB
}

func NewCollectionRepository(db *sql.DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

// ──────────────────── Collections ────────────────────

func (r *CollectionRepository) Create(c *models.Collection) error {
	query := `
		INSERT INTO collections (id, user_id, library_id, name, description, poster_path,
		                         collection_type, visibility, item_sort_mode, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, c.ID, c.UserID, c.LibraryID, c.Name, c.Description,
		c.PosterPath, c.CollectionType, c.Visibility, c.ItemSortMode, c.SortPosition).
		Scan(&c.CreatedAt, &c.UpdatedAt)
}

func (r *CollectionRepository) GetByID(id uuid.UUID) (*models.Collection, error) {
	c := &models.Collection{}
	query := `
		SELECT id, user_id, library_id, name, description, poster_path,
		       collection_type, visibility, item_sort_mode, sort_position,
		       created_at, updated_at
		FROM collections WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&c.ID, &c.UserID, &c.LibraryID, &c.Name, &c.Description, &c.PosterPath,
		&c.CollectionType, &c.Visibility, &c.ItemSortMode, &c.SortPosition,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("collection not found")
	}
	if err != nil {
		return nil, err
	}

	// Load items
	items, err := r.ListItems(id)
	if err != nil {
		return nil, err
	}
	c.Items = items
	c.ItemCount = len(items)
	return c, nil
}

func (r *CollectionRepository) ListByUser(userID uuid.UUID) ([]*models.Collection, error) {
	query := `
		SELECT c.id, c.user_id, c.library_id, c.name, c.description, c.poster_path,
		       c.collection_type, c.visibility, c.item_sort_mode, c.sort_position,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id) as item_count
		FROM collections c
		WHERE c.user_id = $1
		ORDER BY c.sort_position, c.name`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []*models.Collection
	for rows.Next() {
		c := &models.Collection{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.LibraryID, &c.Name, &c.Description,
			&c.PosterPath, &c.CollectionType, &c.Visibility, &c.ItemSortMode,
			&c.SortPosition, &c.CreatedAt, &c.UpdatedAt, &c.ItemCount); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, rows.Err()
}

func (r *CollectionRepository) Update(c *models.Collection) error {
	query := `
		UPDATE collections
		SET name = $1, description = $2, poster_path = $3, visibility = $4,
		    item_sort_mode = $5, updated_at = CURRENT_TIMESTAMP
		WHERE id = $6`
	result, err := r.db.Exec(query, c.Name, c.Description, c.PosterPath,
		c.Visibility, c.ItemSortMode, c.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("collection not found")
	}
	return nil
}

func (r *CollectionRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM collections WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("collection not found")
	}
	return nil
}

// ──────────────────── Collection Items ────────────────────

func (r *CollectionRepository) AddItem(item *models.CollectionItem) error {
	query := `
		INSERT INTO collection_items (id, collection_id, media_item_id, edition_group_id,
		                              tv_show_id, album_id, book_id, sort_position, notes, added_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING added_at`
	return r.db.QueryRow(query, item.ID, item.CollectionID, item.MediaItemID,
		item.EditionGroupID, item.TVShowID, item.AlbumID, item.BookID,
		item.SortPosition, item.Notes, item.AddedBy).
		Scan(&item.AddedAt)
}

func (r *CollectionRepository) ListItems(collectionID uuid.UUID) ([]models.CollectionItem, error) {
	query := `
		SELECT id, collection_id, media_item_id, edition_group_id, tv_show_id,
		       album_id, book_id, sort_position, notes, added_at, added_by
		FROM collection_items WHERE collection_id = $1 ORDER BY sort_position`
	rows, err := r.db.Query(query, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CollectionItem
	for rows.Next() {
		item := models.CollectionItem{}
		if err := rows.Scan(&item.ID, &item.CollectionID, &item.MediaItemID,
			&item.EditionGroupID, &item.TVShowID, &item.AlbumID, &item.BookID,
			&item.SortPosition, &item.Notes, &item.AddedAt, &item.AddedBy); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *CollectionRepository) RemoveItem(collectionID, itemID uuid.UUID) error {
	result, err := r.db.Exec(
		`DELETE FROM collection_items WHERE collection_id = $1 AND id = $2`,
		collectionID, itemID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("collection item not found")
	}
	return nil
}
