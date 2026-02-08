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
	imdb_rating, rt_rating, audience_score,
	edition_type, sort_position, metadata_locked, duplicate_status, added_at, updated_at`

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
		&item.IMDBRating, &item.RTRating, &item.AudienceScore,
		&item.EditionType, &item.SortPosition, &item.MetadataLocked, &item.DuplicateStatus, &item.AddedAt, &item.UpdatedAt,
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
			image_gallery_id, sister_group_id, edition_type, sort_position
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24,
			$25, $26, $27,
			$28, $29, $30,
			$31, $32, $33, $34,
			$35, $36, $37,
			$38, $39, $40, $41
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
		item.ImageGalleryID, item.SisterGroupID, item.EditionType, item.SortPosition,
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

// SearchInLibraries searches media only within the specified library IDs.
func (r *MediaRepository) SearchInLibraries(query string, libraryIDs []uuid.UUID, limit int) ([]*models.MediaItem, error) {
	if len(libraryIDs) == 0 {
		return []*models.MediaItem{}, nil
	}

	// Build parameterized IN clause
	params := []interface{}{"%"+query+"%"}
	inClause := ""
	for i, id := range libraryIDs {
		if i > 0 {
			inClause += ","
		}
		params = append(params, id)
		inClause += fmt.Sprintf("$%d", i+2)
	}
	params = append(params, limit)
	limitParam := fmt.Sprintf("$%d", len(params))

	searchQuery := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE (title ILIKE $1 OR file_name ILIKE $1)
		  AND library_id IN (` + inClause + `)
		ORDER BY title
		LIMIT ` + limitParam

	rows, err := r.db.Query(searchQuery, params...)
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

// LetterIndex returns the cumulative offset for each starting letter (sorted alphabetically).
// Result: [{"letter":"#","count":5,"offset":0},{"letter":"A","count":120,"offset":5}, ...]
func (r *MediaRepository) LetterIndex(libraryID uuid.UUID) ([]map[string]interface{}, error) {
	query := `
		SELECT
			CASE WHEN UPPER(LEFT(COALESCE(sort_title, title), 1)) BETWEEN 'A' AND 'Z'
			     THEN UPPER(LEFT(COALESCE(sort_title, title), 1))
			     ELSE '#' END AS letter,
			COUNT(*) AS cnt
		FROM media_items WHERE library_id = $1
		GROUP BY letter ORDER BY letter`

	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	cumOffset := 0
	for rows.Next() {
		var letter string
		var count int
		if err := rows.Scan(&letter, &count); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"letter": letter,
			"count":  count,
			"offset": cumOffset,
		})
		cumOffset += count
	}
	return result, rows.Err()
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

func (r *MediaRepository) UpdateRatings(id uuid.UUID, imdbRating *float64, rtRating *int, audienceScore *int) error {
	query := `UPDATE media_items SET imdb_rating = $1, rt_rating = $2, audience_score = $3,
		updated_at = CURRENT_TIMESTAMP WHERE id = $4`
	_, err := r.db.Exec(query, imdbRating, rtRating, audienceScore, id)
	return err
}

// UpdateMediaFields updates user-editable metadata fields and sets metadata_locked = true.
func (r *MediaRepository) UpdateMediaFields(id uuid.UUID, title string, sortTitle, originalTitle, description *string, year *int, releaseDate *string, rating *float64, editionType *string) error {
	query := `UPDATE media_items SET
		title = $1, sort_title = $2, original_title = $3, description = $4,
		year = $5, release_date = $6, rating = $7, edition_type = COALESCE($8, edition_type),
		metadata_locked = true, updated_at = CURRENT_TIMESTAMP
		WHERE id = $9`
	_, err := r.db.Exec(query, title, sortTitle, originalTitle, description, year, releaseDate, rating, editionType, id)
	return err
}

// ResetMetadataLock clears the metadata_locked flag so the next scan/auto-match can overwrite.
func (r *MediaRepository) ResetMetadataLock(id uuid.UUID) error {
	_, err := r.db.Exec(`UPDATE media_items SET metadata_locked = false, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}

// IsMetadataLocked returns whether the item has user-edited metadata that should be preserved.
func (r *MediaRepository) IsMetadataLocked(id uuid.UUID) (bool, error) {
	var locked bool
	err := r.db.QueryRow(`SELECT metadata_locked FROM media_items WHERE id = $1`, id).Scan(&locked)
	return locked, err
}

// ListUnlockedByLibrary returns all media items in a library that are not metadata-locked.
func (r *MediaRepository) ListUnlockedByLibrary(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items WHERE library_id = $1 AND metadata_locked = false
		ORDER BY COALESCE(sort_title, title)`
	rows, err := r.db.Query(query, libraryID)
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

func (r *MediaRepository) UpdateLastScanned(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE media_items SET last_scanned_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}

// ListByTVShow returns all episodes for a TV show, ordered by season and episode number.
func (r *MediaRepository) ListByTVShow(showID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items WHERE tv_show_id = $1
		ORDER BY COALESCE(episode_number, 0)`
	rows, err := r.db.Query(query, showID)
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

// ──────────────────── Duplicate / Hash helpers ────────────────────

// UpdateFileHash sets the MD5 hash for a media item.
func (r *MediaRepository) UpdateFileHash(id uuid.UUID, hash string) error {
	_, err := r.db.Exec(`UPDATE media_items SET file_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, hash, id)
	return err
}

// UpdatePhash sets the perceptual hash for a media item.
func (r *MediaRepository) UpdatePhash(id uuid.UUID, phash string) error {
	_, err := r.db.Exec(`UPDATE media_items SET phash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, phash, id)
	return err
}

// UpdateDuplicateStatus sets the duplicate_status flag on a media item.
func (r *MediaRepository) UpdateDuplicateStatus(id uuid.UUID, status string) error {
	_, err := r.db.Exec(`UPDATE media_items SET duplicate_status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, status, id)
	return err
}

// FindByFileHash returns all media items with a matching file_hash (excluding the given id).
func (r *MediaRepository) FindByFileHash(hash string, excludeID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + ` FROM media_items WHERE file_hash = $1 AND id != $2`
	rows, err := r.db.Query(query, hash, excludeID)
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

// ListItemsNeedingPhash returns media items in a library that have no phash and are video types.
func (r *MediaRepository) ListItemsNeedingPhash(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE library_id = $1
		  AND (phash IS NULL OR phash = '')
		  AND media_type IN ('movies','adult_movies','tv_shows','music_videos','home_videos','other_videos')
		ORDER BY added_at`
	rows, err := r.db.Query(query, libraryID)
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

// ListPhashesInLibrary returns all items in a library that have a phash.
func (r *MediaRepository) ListPhashesInLibrary(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE library_id = $1 AND phash IS NOT NULL AND phash != ''
		ORDER BY title`
	rows, err := r.db.Query(query, libraryID)
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

// ListDuplicateItems returns items where duplicate_status is 'exact' or 'potential'.
func (r *MediaRepository) ListDuplicateItems() ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE duplicate_status IN ('exact', 'potential')
		ORDER BY title`
	rows, err := r.db.Query(query)
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

// ListItemsNeedingEnrichment returns items in a library that have a title/description
// (i.e. were TMDB-matched) but are missing OMDb ratings or have no linked performers.
func (r *MediaRepository) ListItemsNeedingEnrichment(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items mi
		WHERE mi.library_id = $1
		  AND mi.metadata_locked = false
		  AND mi.description IS NOT NULL
		  AND mi.description != ''
		  AND (
		    mi.imdb_rating IS NULL
		    OR NOT EXISTS (SELECT 1 FROM media_performers mp WHERE mp.media_item_id = mi.id)
		  )
		  AND mi.media_type IN ('movies','tv_shows','music_videos','home_videos','other_videos','adult_movies')
		ORDER BY mi.title`
	rows, err := r.db.Query(query, libraryID)
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

// CountUnreviewedDuplicates returns how many items have unreviewed duplicate status.
func (r *MediaRepository) CountUnreviewedDuplicates() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM media_items WHERE duplicate_status IN ('exact','potential')`).Scan(&count)
	return count, err
}
