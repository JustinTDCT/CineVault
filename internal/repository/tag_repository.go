package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type TagRepository struct {
	db *sql.DB
}

func NewTagRepository(db *sql.DB) *TagRepository {
	return &TagRepository{db: db}
}

func slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, "\"", "")
	return slug
}

func (r *TagRepository) Create(t *models.Tag) error {
	if t.Slug == "" {
		t.Slug = slugify(t.Name)
	}
	query := `INSERT INTO tags (id, name, slug, parent_id, category, description, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at`
	return r.db.QueryRow(query, t.ID, t.Name, t.Slug, t.ParentID, t.Category,
		t.Description, t.SortPosition).Scan(&t.CreatedAt)
}

func (r *TagRepository) GetByID(id uuid.UUID) (*models.Tag, error) {
	t := &models.Tag{}
	query := `SELECT t.id, t.name, t.slug, t.parent_id, t.category, t.description, t.sort_position, t.created_at,
		COALESCE((SELECT COUNT(*) FROM media_tags mt WHERE mt.tag_id = t.id), 0) as media_count
		FROM tags t WHERE t.id = $1`
	err := r.db.QueryRow(query, id).Scan(&t.ID, &t.Name, &t.Slug, &t.ParentID,
		&t.Category, &t.Description, &t.SortPosition, &t.CreatedAt, &t.MediaCount)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tag not found")
	}
	return t, err
}

func (r *TagRepository) List(category string) ([]*models.Tag, error) {
	query := `SELECT t.id, t.name, t.slug, t.parent_id, t.category, t.description, t.sort_position, t.created_at,
		COALESCE((SELECT COUNT(*) FROM media_tags mt WHERE mt.tag_id = t.id), 0) as media_count
		FROM tags t`
	var args []interface{}
	if category != "" {
		query += ` WHERE t.category = $1`
		args = append(args, category)
	}
	query += ` ORDER BY t.sort_position, t.name`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		t := &models.Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.ParentID,
			&t.Category, &t.Description, &t.SortPosition, &t.CreatedAt, &t.MediaCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *TagRepository) BuildTree(category string) ([]*models.Tag, error) {
	allTags, err := r.List(category)
	if err != nil {
		return nil, err
	}
	tagMap := make(map[uuid.UUID]*models.Tag)
	for _, t := range allTags {
		tagMap[t.ID] = t
	}

	var roots []*models.Tag
	for _, t := range allTags {
		if t.ParentID != nil {
			if parent, ok := tagMap[*t.ParentID]; ok {
				parent.Children = append(parent.Children, t)
				continue
			}
		}
		roots = append(roots, t)
	}
	return roots, nil
}

func (r *TagRepository) Update(t *models.Tag) error {
	if t.Slug == "" {
		t.Slug = slugify(t.Name)
	}
	query := `UPDATE tags SET name=$1, slug=$2, parent_id=$3, category=$4, description=$5, sort_position=$6
		WHERE id=$7`
	result, err := r.db.Exec(query, t.Name, t.Slug, t.ParentID, t.Category, t.Description, t.SortPosition, t.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

func (r *TagRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

func (r *TagRepository) AssignToMedia(mediaItemID, tagID uuid.UUID) error {
	query := `INSERT INTO media_tags (id, media_item_id, tag_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(query, uuid.New(), mediaItemID, tagID)
	return err
}

func (r *TagRepository) RemoveFromMedia(mediaItemID, tagID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_tags WHERE media_item_id=$1 AND tag_id=$2`, mediaItemID, tagID)
	return err
}

func (r *TagRepository) GetMediaTags(mediaItemID uuid.UUID) ([]*models.Tag, error) {
	query := `SELECT t.id, t.name, t.slug, t.parent_id, t.category, t.description, t.sort_position, t.created_at, 0 as media_count
		FROM tags t JOIN media_tags mt ON t.id = mt.tag_id WHERE mt.media_item_id = $1 ORDER BY t.name`
	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		t := &models.Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.ParentID,
			&t.Category, &t.Description, &t.SortPosition, &t.CreatedAt, &t.MediaCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *TagRepository) ListGenresByLibrary(libraryID uuid.UUID) ([]*models.Tag, error) {
	query := `
		SELECT t.id, t.name, t.slug, t.parent_id, t.category, t.description,
		       t.sort_position, t.created_at,
		       COUNT(DISTINCT mt.media_item_id) AS media_count
		FROM tags t
		JOIN media_tags mt ON mt.tag_id = t.id
		JOIN media_items m ON m.id = mt.media_item_id
		WHERE m.library_id = $1 AND t.category = 'genre'
		GROUP BY t.id
		ORDER BY t.name`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		t := &models.Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.ParentID,
			&t.Category, &t.Description, &t.SortPosition, &t.CreatedAt, &t.MediaCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
