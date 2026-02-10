package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
		                         collection_type, visibility, item_sort_mode, sort_position, rules)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, c.ID, c.UserID, c.LibraryID, c.Name, c.Description,
		c.PosterPath, c.CollectionType, c.Visibility, c.ItemSortMode, c.SortPosition, c.Rules).
		Scan(&c.CreatedAt, &c.UpdatedAt)
}

func (r *CollectionRepository) GetByID(id uuid.UUID) (*models.Collection, error) {
	c := &models.Collection{}
	query := `
		SELECT id, user_id, library_id, name, description, poster_path,
		       collection_type, visibility, item_sort_mode, sort_position, rules,
		       created_at, updated_at
		FROM collections WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&c.ID, &c.UserID, &c.LibraryID, &c.Name, &c.Description, &c.PosterPath,
		&c.CollectionType, &c.Visibility, &c.ItemSortMode, &c.SortPosition, &c.Rules,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("collection not found")
	}
	if err != nil {
		return nil, err
	}

	// Load items (for manual collections)
	if c.CollectionType != "smart" {
		items, err := r.ListItems(id)
		if err != nil {
			return nil, err
		}
		c.Items = items
		c.ItemCount = len(items)
	}
	return c, nil
}

func (r *CollectionRepository) ListByUser(userID uuid.UUID) ([]*models.Collection, error) {
	query := `
		SELECT c.id, c.user_id, c.library_id, c.name, c.description, c.poster_path,
		       c.collection_type, c.visibility, c.item_sort_mode, c.sort_position, c.rules,
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
			&c.SortPosition, &c.Rules, &c.CreatedAt, &c.UpdatedAt, &c.ItemCount); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, rows.Err()
}

func (r *CollectionRepository) ListByUserAndLibrary(userID, libraryID uuid.UUID) ([]*models.Collection, error) {
	query := `
		SELECT c.id, c.user_id, c.library_id, c.name, c.description, c.poster_path,
		       c.collection_type, c.visibility, c.item_sort_mode, c.sort_position, c.rules,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id) as item_count
		FROM collections c
		WHERE c.user_id = $1 AND c.library_id = $2
		ORDER BY c.sort_position, c.name`
	rows, err := r.db.Query(query, userID, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []*models.Collection
	for rows.Next() {
		c := &models.Collection{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.LibraryID, &c.Name, &c.Description,
			&c.PosterPath, &c.CollectionType, &c.Visibility, &c.ItemSortMode,
			&c.SortPosition, &c.Rules, &c.CreatedAt, &c.UpdatedAt, &c.ItemCount); err != nil {
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
		    item_sort_mode = $5, rules = $6, updated_at = CURRENT_TIMESTAMP
		WHERE id = $7`
	result, err := r.db.Exec(query, c.Name, c.Description, c.PosterPath,
		c.Visibility, c.ItemSortMode, c.Rules, c.ID)
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

// ──────────────────── Smart Collection Evaluation ────────────────────

// EvaluateSmartCollection evaluates a smart collection's rules and returns matching media items.
func (r *CollectionRepository) EvaluateSmartCollection(rulesJSON string, libraryID *uuid.UUID) ([]*models.MediaItem, error) {
	var rules models.SmartCollectionRules
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return nil, fmt.Errorf("invalid smart collection rules: %w", err)
	}

	var conditions []string
	var args []interface{}
	paramIdx := 1

	// Library filter
	if libraryID != nil {
		conditions = append(conditions, fmt.Sprintf("m.library_id = $%d", paramIdx))
		args = append(args, *libraryID)
		paramIdx++
	}

	// Genre filter (uses tag system)
	if len(rules.Genres) > 0 {
		placeholders := make([]string, len(rules.Genres))
		for i, g := range rules.Genres {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, g)
			paramIdx++
		}
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM media_tags mt JOIN tags t ON t.id = mt.tag_id
			WHERE mt.media_item_id = m.id AND t.category = 'genre'
			  AND LOWER(t.name) IN (`+strings.Join(placeholders, ",")+`)
		)`)
	}

	// Mood filter (uses tag system with mood category)
	if len(rules.Moods) > 0 {
		placeholders := make([]string, len(rules.Moods))
		for i, mood := range rules.Moods {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, mood)
			paramIdx++
		}
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM media_tags mt JOIN tags t ON t.id = mt.tag_id
			WHERE mt.media_item_id = m.id AND t.category = 'mood'
			  AND LOWER(t.name) IN (`+strings.Join(placeholders, ",")+`)
		)`)
	}

	// Year range
	if rules.YearFrom != nil {
		conditions = append(conditions, fmt.Sprintf("m.year >= $%d", paramIdx))
		args = append(args, *rules.YearFrom)
		paramIdx++
	}
	if rules.YearTo != nil {
		conditions = append(conditions, fmt.Sprintf("m.year <= $%d", paramIdx))
		args = append(args, *rules.YearTo)
		paramIdx++
	}

	// Minimum rating
	if rules.MinRating != nil {
		conditions = append(conditions, fmt.Sprintf("m.rating >= $%d", paramIdx))
		args = append(args, *rules.MinRating)
		paramIdx++
	}

	// Content rating filter
	if len(rules.ContentRating) > 0 {
		placeholders := make([]string, len(rules.ContentRating))
		for i, cr := range rules.ContentRating {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, cr)
			paramIdx++
		}
		conditions = append(conditions, "(m.content_rating IN ("+strings.Join(placeholders, ",")+") OR m.content_rating IS NULL)")
	}

	// Keyword filter (JSON array search)
	if len(rules.Keywords) > 0 {
		var kwConds []string
		for _, kw := range rules.Keywords {
			kwConds = append(kwConds, fmt.Sprintf("m.keywords ILIKE '%%' || $%d || '%%'", paramIdx))
			args = append(args, kw)
			paramIdx++
		}
		conditions = append(conditions, "("+strings.Join(kwConds, " OR ")+")")
	}

	whereSQL := "l.is_enabled = true"
	if len(conditions) > 0 {
		whereSQL += " AND " + strings.Join(conditions, " AND ")
	}

	// Skip non-default editions
	whereSQL += " AND NOT EXISTS (SELECT 1 FROM edition_items ei WHERE ei.media_item_id = m.id AND ei.is_default = false)"

	maxResults := 100
	if rules.MaxResults > 0 && rules.MaxResults < 500 {
		maxResults = rules.MaxResults
	}
	args = append(args, maxResults)
	limitParam := fmt.Sprintf("$%d", paramIdx)

	query := `
		SELECT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating, m.content_rating,
		       m.description, m.backdrop_path
		FROM media_items m
		JOIN libraries l ON m.library_id = l.id
		WHERE ` + whereSQL + `
		ORDER BY m.rating DESC NULLS LAST, m.title ASC
		LIMIT ` + limitParam

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		if err := rows.Scan(
			&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName, &item.FileSize,
			&item.Title, &item.DurationSeconds, &item.Resolution, &item.Width, &item.Height,
			&item.Codec, &item.Container, &item.PosterPath, &item.Year, &item.Rating, &item.ContentRating,
			&item.Description, &item.BackdropPath,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
