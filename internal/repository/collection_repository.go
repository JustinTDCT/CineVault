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
		                         collection_type, visibility, item_sort_mode, sort_position, rules, parent_collection_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, c.ID, c.UserID, c.LibraryID, c.Name, c.Description,
		c.PosterPath, c.CollectionType, c.Visibility, c.ItemSortMode, c.SortPosition, c.Rules, c.ParentCollectionID).
		Scan(&c.CreatedAt, &c.UpdatedAt)
}

func (r *CollectionRepository) GetByID(id uuid.UUID) (*models.Collection, error) {
	c := &models.Collection{}
	query := `
		SELECT id, user_id, library_id, name, description, poster_path,
		       collection_type, visibility, item_sort_mode, sort_position, rules,
		       parent_collection_id, created_at, updated_at
		FROM collections WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&c.ID, &c.UserID, &c.LibraryID, &c.Name, &c.Description, &c.PosterPath,
		&c.CollectionType, &c.Visibility, &c.ItemSortMode, &c.SortPosition, &c.Rules,
		&c.ParentCollectionID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("collection not found")
	}
	if err != nil {
		return nil, err
	}

	// Load child count
	r.db.QueryRow(`SELECT COUNT(*) FROM collections WHERE parent_collection_id = $1`, id).Scan(&c.ChildCount)

	// Load items (for manual collections)
	if c.CollectionType != "smart" {
		items, err := r.ListItems(id, c.ItemSortMode)
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
		       c.parent_collection_id, c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id) as item_count,
		       (SELECT COUNT(*) FROM collections ch WHERE ch.parent_collection_id = c.id) as child_count
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
			&c.SortPosition, &c.Rules, &c.ParentCollectionID, &c.CreatedAt, &c.UpdatedAt,
			&c.ItemCount, &c.ChildCount); err != nil {
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
		       c.parent_collection_id, c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id) as item_count,
		       (SELECT COUNT(*) FROM collections ch WHERE ch.parent_collection_id = c.id) as child_count
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
			&c.SortPosition, &c.Rules, &c.ParentCollectionID, &c.CreatedAt, &c.UpdatedAt,
			&c.ItemCount, &c.ChildCount); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, rows.Err()
}

// ListChildren returns child collections of a parent collection.
func (r *CollectionRepository) ListChildren(parentID uuid.UUID) ([]*models.Collection, error) {
	query := `
		SELECT c.id, c.user_id, c.library_id, c.name, c.description, c.poster_path,
		       c.collection_type, c.visibility, c.item_sort_mode, c.sort_position, c.rules,
		       c.parent_collection_id, c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id) as item_count,
		       (SELECT COUNT(*) FROM collections ch WHERE ch.parent_collection_id = c.id) as child_count
		FROM collections c
		WHERE c.parent_collection_id = $1
		ORDER BY c.sort_position, c.name`
	rows, err := r.db.Query(query, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []*models.Collection
	for rows.Next() {
		c := &models.Collection{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.LibraryID, &c.Name, &c.Description,
			&c.PosterPath, &c.CollectionType, &c.Visibility, &c.ItemSortMode,
			&c.SortPosition, &c.Rules, &c.ParentCollectionID, &c.CreatedAt, &c.UpdatedAt,
			&c.ItemCount, &c.ChildCount); err != nil {
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
		    item_sort_mode = $5, rules = $6, parent_collection_id = $7, updated_at = CURRENT_TIMESTAMP
		WHERE id = $8`
	result, err := r.db.Exec(query, c.Name, c.Description, c.PosterPath,
		c.Visibility, c.ItemSortMode, c.Rules, c.ParentCollectionID, c.ID)
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

// BulkAddItems adds multiple items to a collection in a single transaction.
func (r *CollectionRepository) BulkAddItems(collectionID uuid.UUID, items []models.CollectionItem, addedBy *uuid.UUID) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO collection_items (id, collection_id, media_item_id, edition_group_id,
		                              tv_show_id, album_id, book_id, sort_position, notes, added_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range items {
		items[i].ID = uuid.New()
		items[i].CollectionID = collectionID
		items[i].AddedBy = addedBy
		if items[i].SortPosition == 0 {
			items[i].SortPosition = i
		}
		_, err := stmt.Exec(items[i].ID, items[i].CollectionID, items[i].MediaItemID,
			items[i].EditionGroupID, items[i].TVShowID, items[i].AlbumID, items[i].BookID,
			items[i].SortPosition, items[i].Notes, items[i].AddedBy)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// sortOrderSQL returns the ORDER BY clause for a given sort mode, applied to collection items.
func sortOrderSQL(sortMode string) string {
	switch sortMode {
	case "title":
		return "ORDER BY COALESCE(m.title, ts.title, al.title, bk.title, '') ASC"
	case "year":
		return "ORDER BY COALESCE(m.year, 0) DESC, COALESCE(m.title, ts.title, al.title, bk.title, '') ASC"
	case "rating":
		return "ORDER BY COALESCE(m.rating, 0) DESC, COALESCE(m.title, ts.title, al.title, bk.title, '') ASC"
	case "added":
		return "ORDER BY ci.added_at DESC"
	case "duration":
		return "ORDER BY COALESCE(m.duration_seconds, 0) DESC"
	default: // "custom"
		return "ORDER BY ci.sort_position ASC"
	}
}

// ListItems returns collection items with joined media metadata, sorted by the given mode.
func (r *CollectionRepository) ListItems(collectionID uuid.UUID, sortMode string) ([]models.CollectionItem, error) {
	query := `
		SELECT ci.id, ci.collection_id, ci.media_item_id, ci.edition_group_id, ci.tv_show_id,
		       ci.album_id, ci.book_id, ci.sort_position, ci.notes, ci.added_at, ci.added_by,
		       COALESCE(m.title, ts.title, al.title, bk.title, '') AS item_title,
		       m.year, COALESCE(m.poster_path, ts.poster_path, al.poster_path, bk.poster_path, '') AS item_poster,
		       m.rating, m.duration_seconds, m.resolution,
		       COALESCE(m.media_type::text, '') AS item_media_type
		FROM collection_items ci
		LEFT JOIN media_items m ON m.id = ci.media_item_id
		LEFT JOIN tv_shows ts ON ts.id = ci.tv_show_id
		LEFT JOIN albums al ON al.id = ci.album_id
		LEFT JOIN books bk ON bk.id = ci.book_id
		WHERE ci.collection_id = $1
		` + sortOrderSQL(sortMode)

	rows, err := r.db.Query(query, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CollectionItem
	for rows.Next() {
		item := models.CollectionItem{}
		var posterStr string
		if err := rows.Scan(&item.ID, &item.CollectionID, &item.MediaItemID,
			&item.EditionGroupID, &item.TVShowID, &item.AlbumID, &item.BookID,
			&item.SortPosition, &item.Notes, &item.AddedAt, &item.AddedBy,
			&item.Title, &item.Year, &posterStr,
			&item.Rating, &item.DurationSeconds, &item.Resolution,
			&item.MediaType); err != nil {
			return nil, err
		}
		if posterStr != "" {
			item.PosterPath = &posterStr
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

// ──────────────────── Collection Statistics ────────────────────

// GetStats returns aggregate statistics for a collection's items.
func (r *CollectionRepository) GetStats(collectionID uuid.UUID) (*models.CollectionStats, error) {
	stats := &models.CollectionStats{}

	// Basic aggregates
	err := r.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(m.duration_seconds), 0),
		       COALESCE(AVG(m.rating), 0)
		FROM collection_items ci
		LEFT JOIN media_items m ON m.id = ci.media_item_id
		WHERE ci.collection_id = $1`, collectionID).
		Scan(&stats.TotalItems, &stats.TotalRuntime, &stats.AvgRating)
	if err != nil {
		return nil, err
	}

	// Genre breakdown
	rows, err := r.db.Query(`
		SELECT t.name, COUNT(*) as cnt
		FROM collection_items ci
		JOIN media_tags mt ON mt.media_item_id = ci.media_item_id
		JOIN tags t ON t.id = mt.tag_id AND t.category = 'genre'
		WHERE ci.collection_id = $1
		GROUP BY t.name
		ORDER BY cnt DESC
		LIMIT 10`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		g := models.CollectionGenre{}
		if err := rows.Scan(&g.Name, &g.Count); err != nil {
			return nil, err
		}
		stats.Genres = append(stats.Genres, g)
	}
	return stats, rows.Err()
}

// ──────────────────── Smart Collection Evaluation ────────────────────

// smartSortSQL returns the ORDER BY clause for smart collection evaluation.
func smartSortSQL(sortBy, sortOrder string) string {
	dir := "DESC"
	if sortOrder == "asc" {
		dir = "ASC"
	}
	switch sortBy {
	case "title":
		return "ORDER BY m.title " + dir
	case "year":
		return "ORDER BY m.year " + dir + " NULLS LAST, m.title ASC"
	case "rating":
		return "ORDER BY m.rating " + dir + " NULLS LAST, m.title ASC"
	case "added":
		return "ORDER BY m.created_at " + dir
	case "duration":
		return "ORDER BY m.duration_seconds " + dir + " NULLS LAST"
	case "random":
		return "ORDER BY RANDOM()"
	default:
		return "ORDER BY m.rating DESC NULLS LAST, m.title ASC"
	}
}

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

	// Exclude genres (NOT filter)
	if len(rules.ExcludeGenres) > 0 {
		placeholders := make([]string, len(rules.ExcludeGenres))
		for i, g := range rules.ExcludeGenres {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, g)
			paramIdx++
		}
		conditions = append(conditions, `NOT EXISTS (
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

	// Performer filter (cast/director name match)
	if len(rules.Performers) > 0 {
		placeholders := make([]string, len(rules.Performers))
		for i, p := range rules.Performers {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, strings.ToLower(p))
			paramIdx++
		}
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM media_performers mp JOIN performers p ON p.id = mp.performer_id
			WHERE mp.media_item_id = m.id AND LOWER(p.name) IN (`+strings.Join(placeholders, ",")+`)
		)`)
	}

	// Studio filter (uses studios/media_studios join tables)
	if len(rules.Studios) > 0 {
		placeholders := make([]string, len(rules.Studios))
		for i, s := range rules.Studios {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			args = append(args, strings.ToLower(s))
			paramIdx++
		}
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM media_studios ms JOIN studios st ON st.id = ms.studio_id
			WHERE ms.media_item_id = m.id AND LOWER(st.name) IN (`+strings.Join(placeholders, ",")+`)
		)`)
	}

	// Duration filters (stored as minutes in rules, seconds in DB)
	if rules.MinDuration != nil {
		conditions = append(conditions, fmt.Sprintf("m.duration_seconds >= $%d", paramIdx))
		args = append(args, *rules.MinDuration*60)
		paramIdx++
	}
	if rules.MaxDuration != nil {
		conditions = append(conditions, fmt.Sprintf("m.duration_seconds <= $%d", paramIdx))
		args = append(args, *rules.MaxDuration*60)
		paramIdx++
	}

	// Added within N days
	if rules.AddedWithin != nil {
		conditions = append(conditions, fmt.Sprintf("m.created_at >= CURRENT_TIMESTAMP - ($%d || ' days')::INTERVAL", paramIdx))
		args = append(args, *rules.AddedWithin)
		paramIdx++
	}

	// Released within N days (approximate: current year minus threshold)
	if rules.ReleasedWithin != nil {
		conditions = append(conditions, fmt.Sprintf("m.year >= EXTRACT(YEAR FROM CURRENT_DATE) - $%d", paramIdx))
		args = append(args, *rules.ReleasedWithin)
		paramIdx++
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

	orderSQL := smartSortSQL(rules.SortBy, rules.SortOrder)

	query := `
		SELECT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.duration_seconds, m.resolution, m.width, m.height,
		       m.codec, m.container, m.poster_path, m.year, m.rating, m.content_rating,
		       m.description, m.backdrop_path
		FROM media_items m
		JOIN libraries l ON m.library_id = l.id
		WHERE ` + whereSQL + `
		` + orderSQL + `
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
