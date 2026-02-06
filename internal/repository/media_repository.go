package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

// mediaColumns is the standard SELECT list for media_items
const mediaColumns = `id, library_id, media_type, file_path, file_name, file_size,
	file_hash, title, sort_title, original_title, description, year, release_date,
	duration_seconds, rating, resolution, width, height, codec, container,
	bitrate, framerate, audio_codec, audio_channels,
	poster_path, thumbnail_path, backdrop_path,
	tv_show_id, tv_season_id, episode_number,
	artist_id, album_id, track_number, disc_number,
	author_id, book_id, chapter_number,
	image_gallery_id, sister_group_id,
	sort_position, added_at, updated_at`

func scanMediaItem(row interface{ Scan(dest ...interface{}) error }) (*models.MediaItem, error) {
	item := &models.MediaItem{}
	err := row.Scan(
		&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName,
		&item.FileSize, &item.FileHash, &item.Title, &item.SortTitle, &item.OriginalTitle,
		&item.Description, &item.Year, &item.ReleaseDate,
		&item.DurationSeconds, &item.Rating, &item.Resolution, &item.Width, &item.Height,
		&item.Codec, &item.Container, &item.Bitrate, &item.Framerate,
		&item.AudioCodec, &item.AudioChannels,
		&item.PosterPath, &item.ThumbnailPath, &item.BackdropPath,
		&item.TVShowID, &item.TVSeasonID, &item.EpisodeNumber,
		&item.ArtistID, &item.AlbumID, &item.TrackNumber, &item.DiscNumber,
		&item.AuthorID, &item.BookID, &item.ChapterNumber,
		&item.ImageGalleryID, &item.SisterGroupID,
		&item.SortPosition, &item.AddedAt, &item.UpdatedAt,
	)
	return item, err
}

func (r *MediaRepository) Create(item *models.MediaItem) error {
	query := `
		INSERT INTO media_items (
			id, library_id, media_type, file_path, file_name, file_size, file_hash,
			title, sort_title, original_title, description, year, release_date,
			duration_seconds, rating, resolution, width, height, codec, container,
			bitrate, framerate, audio_codec, audio_channels,
			poster_path, thumbnail_path, backdrop_path,
			tv_show_id, tv_season_id, episode_number,
			artist_id, album_id, track_number, disc_number,
			author_id, book_id, chapter_number,
			image_gallery_id, sister_group_id, sort_position
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24,
			$25, $26, $27,
			$28, $29, $30,
			$31, $32, $33, $34,
			$35, $36, $37,
			$38, $39, $40
		)
		RETURNING added_at, updated_at`

	return r.db.QueryRow(query,
		item.ID, item.LibraryID, item.MediaType, item.FilePath, item.FileName,
		item.FileSize, item.FileHash,
		item.Title, item.SortTitle, item.OriginalTitle, item.Description, item.Year, item.ReleaseDate,
		item.DurationSeconds, item.Rating, item.Resolution, item.Width, item.Height,
		item.Codec, item.Container, item.Bitrate, item.Framerate,
		item.AudioCodec, item.AudioChannels,
		item.PosterPath, item.ThumbnailPath, item.BackdropPath,
		item.TVShowID, item.TVSeasonID, item.EpisodeNumber,
		item.ArtistID, item.AlbumID, item.TrackNumber, item.DiscNumber,
		item.AuthorID, item.BookID, item.ChapterNumber,
		item.ImageGalleryID, item.SisterGroupID, item.SortPosition,
	).Scan(&item.AddedAt, &item.UpdatedAt)
}

func (r *MediaRepository) GetByID(id uuid.UUID) (*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + ` FROM media_items WHERE id = $1`
	item, err := scanMediaItem(r.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("media item not found")
	}
	return item, err
}

func (r *MediaRepository) GetByFilePath(filePath string) (*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + ` FROM media_items WHERE file_path = $1`
	item, err := scanMediaItem(r.db.QueryRow(query, filePath))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *MediaRepository) ListByLibrary(libraryID uuid.UUID, limit, offset int) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items WHERE library_id = $1
		ORDER BY COALESCE(sort_title, title)
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, libraryID, limit, offset)
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

func (r *MediaRepository) Search(query string, limit int) ([]*models.MediaItem, error) {
	searchQuery := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE title ILIKE $1 OR file_name ILIKE $1
		ORDER BY title
		LIMIT $2`

	rows, err := r.db.Query(searchQuery, "%"+query+"%", limit)
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

func (r *MediaRepository) CountByLibrary(libraryID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM media_items WHERE library_id = $1`, libraryID).Scan(&count)
	return count, err
}

func (r *MediaRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM media_items WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("media item not found")
	}
	return nil
}

func (r *MediaRepository) UpdateMetadata(id uuid.UUID, title string, year *int, description *string, rating *float64, posterPath *string) error {
	query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
		poster_path = $5, updated_at = CURRENT_TIMESTAMP WHERE id = $6`
	_, err := r.db.Exec(query, title, year, description, rating, posterPath, id)
	return err
}

func (r *MediaRepository) UpdateLastScanned(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE media_items SET last_scanned_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}
