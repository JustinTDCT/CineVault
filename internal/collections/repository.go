package collections

import (
	"database/sql"
	"encoding/json"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(c *Collection) error {
	if c.SmartFilters == nil {
		c.SmartFilters = json.RawMessage("{}")
	}
	return r.db.QueryRow(`
		INSERT INTO collections (user_id, name, description, poster_path, is_smart, smart_filters, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at, updated_at`,
		c.UserID, c.Name, c.Description, c.PosterPath, c.IsSmart, c.SmartFilters, c.SortOrder,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *Repository) ListByUser(userID string) ([]Collection, error) {
	rows, err := r.db.Query(`
		SELECT id, user_id, name, description, poster_path, is_smart, smart_filters,
		       sort_order, created_at, updated_at
		FROM collections WHERE user_id=$1 ORDER BY sort_order, name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.PosterPath,
			&c.IsSmart, &c.SmartFilters, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM collections WHERE id=$1", id)
	return err
}

func (r *Repository) AddItem(collectionID, mediaItemID string, sortOrder int) error {
	_, err := r.db.Exec(`
		INSERT INTO collection_items (collection_id, media_item_id, sort_order)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		collectionID, mediaItemID, sortOrder)
	return err
}

func (r *Repository) RemoveItem(collectionID, mediaItemID string) error {
	_, err := r.db.Exec("DELETE FROM collection_items WHERE collection_id=$1 AND media_item_id=$2",
		collectionID, mediaItemID)
	return err
}

func (r *Repository) GetItems(collectionID string) ([]CollectionItem, error) {
	rows, err := r.db.Query(`
		SELECT id, collection_id, media_item_id, sort_order
		FROM collection_items WHERE collection_id=$1 ORDER BY sort_order`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CollectionItem
	for rows.Next() {
		var ci CollectionItem
		if err := rows.Scan(&ci.ID, &ci.CollectionID, &ci.MediaItemID, &ci.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, ci)
	}
	return out, nil
}
