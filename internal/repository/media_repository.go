package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// MediaFilter holds optional filter and sort parameters for media queries.
type MediaFilter struct {
	Genre         string // genre tag name
	Folder        string // folder path prefix
	ContentRating string // e.g. "PG-13"
	Edition       string // e.g. "Director's Cut"
	Source        string // e.g. "bluray", "web", "hdtv"
	DynamicRange  string // "SDR" or "HDR"
	Codec         string // e.g. "hevc", "h264", "av1"
	HDRFormat     string // e.g. "Dolby Vision", "HDR10", "HLG"
	Resolution    string // e.g. "4K", "1080p", "720p"
	AudioCodec    string // e.g. "truehd", "eac3", "aac"
	BitrateRange  string // "low" (<5Mbps), "medium" (5-15), "high" (15-30), "ultra" (30+)
	Country       string // e.g. "United States", "Canada"
	DurationRange string // "short" (<30min), "medium" (30-90), "long" (90-180), "vlong" (>180)
	WatchStatus   string // "watched", "unwatched"
	AddedDays     string // "1" (today), "7", "30", "90"
	YearFrom      string // e.g. "2000"
	YearTo        string // e.g. "2025"
	MinRating     string // e.g. "7"
	Sort          string // "title" (default), "year", "resolution", "duration", "rt_rating", "rating", "audience_score", "bitrate", "added_at"
	Order         string // "asc" (default), "desc"
}

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

// DB returns the underlying database connection for direct queries.
func (r *MediaRepository) DB() *sql.DB {
	return r.db
}

// mediaColumns is the standard SELECT list for media_items
const mediaColumns = `id, library_id, media_type, file_path, file_name, file_size,
	file_hash, title, sort_title, original_title, description, tagline, year, release_date,
	duration_seconds, rating, resolution, width, height, codec, container,
	bitrate, framerate, audio_codec, audio_channels, audio_format,
	original_language, country, trailer_url,
	poster_path, thumbnail_path, backdrop_path, logo_path,
	tv_show_id, tv_season_id, episode_number,
	artist_id, album_id, track_number, disc_number,
	author_id, book_id, chapter_number,
	image_gallery_id, sister_group_id,
	imdb_rating, rt_rating, audience_score,
	edition_type, content_rating, sort_position, external_ids, generated_poster,
	source_type, hdr_format, dynamic_range, custom_notes, custom_tags,
	metadata_locked, locked_fields, duplicate_status, parent_media_id, extra_type,
	added_at, updated_at`

// prefixedMediaColumns returns mediaColumns with each column prefixed by the given alias (e.g. "m.").
func prefixedMediaColumns(prefix string) string {
	cols := strings.Split(mediaColumns, ",")
	for i, c := range cols {
		cols[i] = prefix + strings.TrimSpace(c)
	}
	return strings.Join(cols, ", ")
}

func scanMediaItem(row interface{ Scan(dest ...interface{}) error }) (*models.MediaItem, error) {
	item := &models.MediaItem{}
	err := row.Scan(
		&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName,
		&item.FileSize, &item.FileHash, &item.Title, &item.SortTitle, &item.OriginalTitle,
		&item.Description, &item.Tagline, &item.Year, &item.ReleaseDate,
		&item.DurationSeconds, &item.Rating, &item.Resolution, &item.Width, &item.Height,
		&item.Codec, &item.Container, &item.Bitrate, &item.Framerate,
		&item.AudioCodec, &item.AudioChannels, &item.AudioFormat,
		&item.OriginalLanguage, &item.Country, &item.TrailerURL,
		&item.PosterPath, &item.ThumbnailPath, &item.BackdropPath, &item.LogoPath,
		&item.TVShowID, &item.TVSeasonID, &item.EpisodeNumber,
		&item.ArtistID, &item.AlbumID, &item.TrackNumber, &item.DiscNumber,
		&item.AuthorID, &item.BookID, &item.ChapterNumber,
		&item.ImageGalleryID, &item.SisterGroupID,
		&item.IMDBRating, &item.RTRating, &item.AudienceScore,
		&item.EditionType, &item.ContentRating, &item.SortPosition, &item.ExternalIDs, &item.GeneratedPoster,
		&item.SourceType, &item.HDRFormat, &item.DynamicRange, &item.CustomNotes, &item.CustomTags,
		&item.MetadataLocked, &item.LockedFields, &item.DuplicateStatus,
		&item.ParentMediaID, &item.ExtraType, &item.AddedAt, &item.UpdatedAt,
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
			image_gallery_id, sister_group_id, edition_type, sort_position,
			source_type, hdr_format, dynamic_range,
			parent_media_id, extra_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24,
			$25, $26, $27,
			$28, $29, $30,
			$31, $32, $33, $34,
			$35, $36, $37,
			$38, $39, $40, $41,
			$42, $43, $44,
			$45, $46
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
		item.SourceType, item.HDRFormat, item.DynamicRange,
		item.ParentMediaID, item.ExtraType,
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

// GetExtras returns extras (trailers, featurettes, etc.) linked to a parent media item.
func (r *MediaRepository) GetExtras(parentID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + ` FROM media_items WHERE parent_media_id = $1 ORDER BY extra_type, title`
	rows, err := r.db.Query(query, parentID)
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

// MarkUnavailable sets a media item as unavailable (file deleted from disk).
func (r *MediaRepository) MarkUnavailable(filePath string) error {
	query := `DELETE FROM media_items WHERE file_path = $1`
	_, err := r.db.Exec(query, filePath)
	return err
}

// UpdateParentMediaID sets the parent_media_id for an extras item.
func (r *MediaRepository) UpdateParentMediaID(id, parentID uuid.UUID) error {
	query := `UPDATE media_items SET parent_media_id = $1 WHERE id = $2`
	_, err := r.db.Exec(query, parentID, id)
	return err
}

// FindParentByDirectory tries to find a non-extra media item in the same library that
// could be the parent of an extra file (same directory minus the extras subfolder).
func (r *MediaRepository) FindParentByDirectory(libraryID uuid.UUID, dirPath string) (*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + ` FROM media_items
		WHERE library_id = $1 AND extra_type IS NULL
		AND file_path LIKE $2 || '%'
		ORDER BY added_at ASC LIMIT 1`
	item, err := scanMediaItem(r.db.QueryRow(query, libraryID, dirPath))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

// buildFilterClauses builds JOIN, WHERE, and ORDER BY fragments from a MediaFilter.
// paramStart is the next parameter index (e.g. 2 if $1 is libraryID).
// Returns (joinSQL, whereSQL, orderSQL, args).
func buildFilterClauses(f *MediaFilter, paramStart int) (string, string, string, []interface{}) {
	var joins []string
	var wheres []string
	var args []interface{}
	p := paramStart

	if f != nil {
		if f.Genre != "" {
			joins = append(joins, fmt.Sprintf(
				`JOIN media_tags mt ON mt.media_item_id = m.id JOIN tags t ON t.id = mt.tag_id AND t.category = 'genre' AND t.name = $%d`, p))
			args = append(args, f.Genre)
			p++
		}
		if f.Folder != "" {
			wheres = append(wheres, fmt.Sprintf(`m.file_path LIKE $%d`, p))
			args = append(args, f.Folder+"%")
			p++
		}
		if f.ContentRating != "" {
			wheres = append(wheres, fmt.Sprintf(`m.content_rating = $%d`, p))
			args = append(args, f.ContentRating)
			p++
		}
		if f.Edition != "" {
			wheres = append(wheres, fmt.Sprintf(`m.edition_type = $%d`, p))
			args = append(args, f.Edition)
			p++
		}
		if f.Source != "" {
			wheres = append(wheres, fmt.Sprintf(`m.source_type = $%d`, p))
			args = append(args, f.Source)
			p++
		}
		if f.DynamicRange != "" {
			wheres = append(wheres, fmt.Sprintf(`m.dynamic_range = $%d`, p))
			args = append(args, f.DynamicRange)
			p++
		}
		if f.Codec != "" {
			wheres = append(wheres, fmt.Sprintf(`m.codec = $%d`, p))
			args = append(args, f.Codec)
			p++
		}
		if f.HDRFormat != "" {
			wheres = append(wheres, fmt.Sprintf(`m.hdr_format = $%d`, p))
			args = append(args, f.HDRFormat)
			p++
		}
		if f.Resolution != "" {
			wheres = append(wheres, fmt.Sprintf(`m.resolution = $%d`, p))
			args = append(args, f.Resolution)
			p++
		}
		if f.AudioCodec != "" {
			wheres = append(wheres, fmt.Sprintf(`m.audio_codec = $%d`, p))
			args = append(args, f.AudioCodec)
			p++
		}
		if f.BitrateRange != "" {
			switch f.BitrateRange {
			case "low":
				wheres = append(wheres, `m.bitrate IS NOT NULL AND m.bitrate < 5000000`)
			case "medium":
				wheres = append(wheres, `m.bitrate IS NOT NULL AND m.bitrate >= 5000000 AND m.bitrate < 15000000`)
			case "high":
				wheres = append(wheres, `m.bitrate IS NOT NULL AND m.bitrate >= 15000000 AND m.bitrate < 30000000`)
			case "ultra":
				wheres = append(wheres, `m.bitrate IS NOT NULL AND m.bitrate >= 30000000`)
			}
		}
		if f.Country != "" {
			wheres = append(wheres, fmt.Sprintf(`m.country = $%d`, p))
			args = append(args, f.Country)
			p++
		}
		if f.DurationRange != "" {
			switch f.DurationRange {
			case "short":
				wheres = append(wheres, `m.duration_seconds IS NOT NULL AND m.duration_seconds > 0 AND m.duration_seconds < 1800`)
			case "medium":
				wheres = append(wheres, `m.duration_seconds IS NOT NULL AND m.duration_seconds >= 1800 AND m.duration_seconds < 5400`)
			case "long":
				wheres = append(wheres, `m.duration_seconds IS NOT NULL AND m.duration_seconds >= 5400 AND m.duration_seconds < 10800`)
			case "vlong":
				wheres = append(wheres, `m.duration_seconds IS NOT NULL AND m.duration_seconds >= 10800`)
			}
		}
		if f.WatchStatus != "" {
			switch f.WatchStatus {
			case "watched":
				wheres = append(wheres, `EXISTS (SELECT 1 FROM watch_history wh WHERE wh.media_item_id = m.id)`)
			case "unwatched":
				wheres = append(wheres, `NOT EXISTS (SELECT 1 FROM watch_history wh WHERE wh.media_item_id = m.id)`)
			}
		}
		if f.AddedDays != "" {
			wheres = append(wheres, fmt.Sprintf(`m.added_at >= NOW() - ($%d || ' days')::interval`, p))
			args = append(args, f.AddedDays)
			p++
		}
		if f.YearFrom != "" {
			wheres = append(wheres, fmt.Sprintf(`m.year >= $%d`, p))
			args = append(args, f.YearFrom)
			p++
		}
		if f.YearTo != "" {
			wheres = append(wheres, fmt.Sprintf(`m.year <= $%d`, p))
			args = append(args, f.YearTo)
			p++
		}
		if f.MinRating != "" {
			wheres = append(wheres, fmt.Sprintf(`m.rating >= $%d`, p))
			args = append(args, f.MinRating)
			p++
		}
	}

	// Always exclude child editions (non-default edition items) from library listings
	wheres = append(wheres, `NOT EXISTS (SELECT 1 FROM edition_items ei WHERE ei.media_item_id = m.id AND ei.is_default = false)`)

	joinSQL := ""
	if len(joins) > 0 {
		joinSQL = " " + strings.Join(joins, " ")
	}

	whereSQL := ""
	if len(wheres) > 0 {
		whereSQL = " AND " + strings.Join(wheres, " AND ")
	}

	// Build ORDER BY
	orderCol := "COALESCE(m.sort_title, m.title)"
	if f != nil {
		switch f.Sort {
		case "year":
			orderCol = "m.year"
		case "rt_rating":
			orderCol = "m.rt_rating"
		case "rating":
			orderCol = "m.rating"
		case "audience_score":
			orderCol = "m.audience_score"
		case "resolution":
			orderCol = "m.height"
		case "duration":
			orderCol = "m.duration_seconds"
		case "bitrate":
			orderCol = "m.bitrate"
		case "added_at":
			orderCol = "m.added_at"
		}
	}
	dir := "ASC"
	if f != nil && strings.EqualFold(f.Order, "desc") {
		dir = "DESC"
	}
	orderSQL := fmt.Sprintf(" ORDER BY %s %s NULLS LAST", orderCol, dir)

	return joinSQL, whereSQL, orderSQL, args
}

func (r *MediaRepository) ListByLibrary(libraryID uuid.UUID, limit, offset int) ([]*models.MediaItem, error) {
	return r.ListByLibraryFiltered(libraryID, limit, offset, nil)
}

func (r *MediaRepository) ListByLibraryFiltered(libraryID uuid.UUID, limit, offset int, f *MediaFilter) ([]*models.MediaItem, error) {
	joinSQL, whereSQL, orderSQL, filterArgs := buildFilterClauses(f, 2)

	// Build the column list with m. prefix for the main query
	query := `SELECT ` + prefixedMediaColumns("m.") + `
		FROM media_items m` + joinSQL + `
		WHERE m.library_id = $1` + whereSQL + orderSQL

	// Add LIMIT/OFFSET
	pLimit := len(filterArgs) + 2
	pOffset := pLimit + 1
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, pLimit, pOffset)

	allArgs := []interface{}{libraryID}
	allArgs = append(allArgs, filterArgs...)
	allArgs = append(allArgs, limit, offset)

	rows, err := r.db.Query(query, allArgs...)
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
		  AND NOT EXISTS (SELECT 1 FROM edition_items ei WHERE ei.media_item_id = media_items.id AND ei.is_default = false)
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
	return r.CountByLibraryFiltered(libraryID, nil)
}

func (r *MediaRepository) CountByLibraryFiltered(libraryID uuid.UUID, f *MediaFilter) (int, error) {
	joinSQL, whereSQL, _, filterArgs := buildFilterClauses(f, 2)
	query := `SELECT COUNT(*) FROM media_items m` + joinSQL + ` WHERE m.library_id = $1` + whereSQL

	allArgs := []interface{}{libraryID}
	allArgs = append(allArgs, filterArgs...)

	var count int
	err := r.db.QueryRow(query, allArgs...).Scan(&count)
	return count, err
}

// LetterIndex returns the cumulative offset for each starting letter (sorted alphabetically).
// Result: [{"letter":"#","count":5,"offset":0},{"letter":"A","count":120,"offset":5}, ...]
func (r *MediaRepository) LetterIndex(libraryID uuid.UUID) ([]map[string]interface{}, error) {
	return r.LetterIndexFiltered(libraryID, nil)
}

func (r *MediaRepository) LetterIndexFiltered(libraryID uuid.UUID, f *MediaFilter) ([]map[string]interface{}, error) {
	joinSQL, whereSQL, _, filterArgs := buildFilterClauses(f, 2)
	query := `
		SELECT
			CASE WHEN UPPER(LEFT(COALESCE(m.sort_title, m.title), 1)) BETWEEN 'A' AND 'Z'
			     THEN UPPER(LEFT(COALESCE(m.sort_title, m.title), 1))
			     ELSE '#' END AS letter,
			COUNT(*) AS cnt
		FROM media_items m` + joinSQL + `
		WHERE m.library_id = $1` + whereSQL + `
		GROUP BY letter ORDER BY letter`

	allArgs := []interface{}{libraryID}
	allArgs = append(allArgs, filterArgs...)

	rows, err := r.db.Query(query, allArgs...)
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

func (r *MediaRepository) UpdateMetadata(id uuid.UUID, title string, year *int, description *string, rating *float64, posterPath *string, contentRating *string) error {
	query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
		poster_path = $5, content_rating = $6,
		generated_poster = CASE WHEN ($5::text) IS NOT NULL THEN false ELSE generated_poster END,
		updated_at = CURRENT_TIMESTAMP WHERE id = $7`
	_, err := r.db.Exec(query, title, year, description, rating, posterPath, contentRating, id)
	return err
}

// UpdateMetadataWithLocks updates metadata fields but respects per-field locks.
// Locked fields retain their current database value.
func (r *MediaRepository) UpdateMetadataWithLocks(id uuid.UUID, title string, year *int, description *string, rating *float64, posterPath *string, contentRating *string, lockedFields pq.StringArray) error {
	query := `UPDATE media_items SET
		title = CASE WHEN $8::text[] IS NOT NULL AND ('title' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN title ELSE $1 END,
		year = CASE WHEN $8::text[] IS NOT NULL AND ('year' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN year ELSE $2 END,
		description = CASE WHEN $8::text[] IS NOT NULL AND ('description' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN description ELSE $3 END,
		rating = CASE WHEN $8::text[] IS NOT NULL AND ('rating' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN rating ELSE $4 END,
		poster_path = CASE WHEN $8::text[] IS NOT NULL AND ('poster_path' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN poster_path ELSE $5 END,
		content_rating = CASE WHEN $8::text[] IS NOT NULL AND ('content_rating' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN content_rating ELSE $6 END,
		generated_poster = CASE
			WHEN $8::text[] IS NOT NULL AND ('poster_path' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN generated_poster
			WHEN ($5::text) IS NOT NULL THEN false
			ELSE generated_poster END,
		updated_at = CURRENT_TIMESTAMP WHERE id = $7`
	_, err := r.db.Exec(query, title, year, description, rating, posterPath, contentRating, id, lockedFields)
	return err
}

func (r *MediaRepository) UpdateRatings(id uuid.UUID, imdbRating *float64, rtRating *int, audienceScore *int) error {
	query := `UPDATE media_items SET imdb_rating = $1, rt_rating = $2, audience_score = $3,
		updated_at = CURRENT_TIMESTAMP WHERE id = $4`
	_, err := r.db.Exec(query, imdbRating, rtRating, audienceScore, id)
	return err
}

// UpdateRatingsWithLocks updates rating fields but respects per-field locks.
func (r *MediaRepository) UpdateRatingsWithLocks(id uuid.UUID, imdbRating *float64, rtRating *int, audienceScore *int, lockedFields pq.StringArray) error {
	query := `UPDATE media_items SET
		imdb_rating = CASE WHEN $5::text[] IS NOT NULL AND ('imdb_rating' = ANY($5::text[]) OR '*' = ANY($5::text[])) THEN imdb_rating ELSE $1 END,
		rt_rating = CASE WHEN $5::text[] IS NOT NULL AND ('rt_rating' = ANY($5::text[]) OR '*' = ANY($5::text[])) THEN rt_rating ELSE $2 END,
		audience_score = CASE WHEN $5::text[] IS NOT NULL AND ('audience_score' = ANY($5::text[]) OR '*' = ANY($5::text[])) THEN audience_score ELSE $3 END,
		updated_at = CURRENT_TIMESTAMP WHERE id = $4`
	_, err := r.db.Exec(query, imdbRating, rtRating, audienceScore, id, lockedFields)
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

// UpdatePosterPath sets the poster image path for a media item.
func (r *MediaRepository) UpdatePosterPath(id uuid.UUID, posterPath string) error {
	_, err := r.db.Exec(`UPDATE media_items SET poster_path = $1, generated_poster = false, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, posterPath, id)
	return err
}

// SetGeneratedPoster marks a media item's poster as generated from a video screenshot.
func (r *MediaRepository) SetGeneratedPoster(id uuid.UUID, generated bool) error {
	_, err := r.db.Exec(`UPDATE media_items SET generated_poster = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, generated, id)
	return err
}

// UpdateExternalIDs stores the external source IDs JSON for a media item.
func (r *MediaRepository) UpdateExternalIDs(id uuid.UUID, externalIDsJSON string) error {
	_, err := r.db.Exec(`UPDATE media_items SET external_ids = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, externalIDsJSON, id)
	return err
}

// UpdateContentRating sets the content rating (e.g. PG-13, R) for a media item.
func (r *MediaRepository) UpdateContentRating(id uuid.UUID, contentRating string) error {
	_, err := r.db.Exec(`UPDATE media_items SET content_rating = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, contentRating, id)
	return err
}

// ExtendedMetadataUpdate holds optional fields for extended metadata updates.
// Only non-nil fields are written to the database.
type ExtendedMetadataUpdate struct {
	Tagline          *string
	OriginalLanguage *string
	Country          *string
	TrailerURL       *string
	LogoPath         *string
	OriginalTitle    *string
	SortTitle        *string
	ReleaseDate      *string
	BannerPath       *string
}

// UpdateExtendedMetadata sets extended metadata fields (tagline, language, country, trailer, logo).
// Only non-nil fields are updated; nil values are left unchanged.
func (r *MediaRepository) UpdateExtendedMetadata(id uuid.UUID, tagline, originalLang, country, trailerURL, logoPath *string) error {
	return r.UpdateExtendedMetadataFull(id, &ExtendedMetadataUpdate{
		Tagline:          tagline,
		OriginalLanguage: originalLang,
		Country:          country,
		TrailerURL:       trailerURL,
		LogoPath:         logoPath,
	})
}

// UpdateExtendedMetadataFull sets all extended metadata fields from an update struct.
// Only non-nil fields are updated; nil values are left unchanged.
func (r *MediaRepository) UpdateExtendedMetadataFull(id uuid.UUID, u *ExtendedMetadataUpdate) error {
	setClauses := []string{}
	args := []interface{}{}
	idx := 1

	if u.Tagline != nil {
		setClauses = append(setClauses, fmt.Sprintf("tagline = $%d", idx))
		args = append(args, *u.Tagline)
		idx++
	}
	if u.OriginalLanguage != nil {
		setClauses = append(setClauses, fmt.Sprintf("original_language = $%d", idx))
		args = append(args, *u.OriginalLanguage)
		idx++
	}
	if u.Country != nil {
		setClauses = append(setClauses, fmt.Sprintf("country = $%d", idx))
		args = append(args, *u.Country)
		idx++
	}
	if u.TrailerURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("trailer_url = $%d", idx))
		args = append(args, *u.TrailerURL)
		idx++
	}
	if u.LogoPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("logo_path = $%d", idx))
		args = append(args, *u.LogoPath)
		idx++
	}
	if u.OriginalTitle != nil {
		setClauses = append(setClauses, fmt.Sprintf("original_title = $%d", idx))
		args = append(args, *u.OriginalTitle)
		idx++
	}
	if u.SortTitle != nil {
		setClauses = append(setClauses, fmt.Sprintf("sort_title = $%d", idx))
		args = append(args, *u.SortTitle)
		idx++
	}
	if u.ReleaseDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("release_date = $%d", idx))
		args = append(args, *u.ReleaseDate)
		idx++
	}
	if u.BannerPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("banner_path = $%d", idx))
		args = append(args, *u.BannerPath)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), idx)
	args = append(args, id)
	_, err := r.db.Exec(query, args...)
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
// (i.e. were TMDB-matched) but are missing OMDb ratings, have no linked performers,
// or have a generated (screenshot) poster that should be replaced.
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
		    OR mi.generated_poster = true
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

// FilterOptions holds the distinct filter values available for a library.
type FilterOptions struct {
	Genres         []string `json:"genres"`
	Folders        []string `json:"folders"`
	ContentRatings []string `json:"content_ratings"`
	Editions       []string `json:"editions"`
	Sources        []string `json:"sources"`
	DynamicRanges  []string `json:"dynamic_ranges"`
	Codecs         []string `json:"codecs"`
	HDRFormats     []string `json:"hdr_formats"`
	Resolutions    []string `json:"resolutions"`
	AudioCodecs    []string `json:"audio_codecs"`
	Countries      []string `json:"countries"`
}

// GetLibraryFilterOptions returns the distinct values available for filtering a library.
func (r *MediaRepository) GetLibraryFilterOptions(libraryID uuid.UUID) (*FilterOptions, error) {
	opts := &FilterOptions{}

	// Genres from tags via media_tags
	genreRows, err := r.db.Query(`
		SELECT DISTINCT t.name FROM tags t
		JOIN media_tags mt ON mt.tag_id = t.id
		JOIN media_items m ON m.id = mt.media_item_id
		WHERE m.library_id = $1 AND t.category = 'genre'
		ORDER BY t.name`, libraryID)
	if err != nil {
		return nil, err
	}
	defer genreRows.Close()
	for genreRows.Next() {
		var name string
		if err := genreRows.Scan(&name); err != nil {
			return nil, err
		}
		opts.Genres = append(opts.Genres, name)
	}

	// Folders from library_folders
	folderRows, err := r.db.Query(`
		SELECT folder_path FROM library_folders
		WHERE library_id = $1 ORDER BY sort_position`, libraryID)
	if err != nil {
		return nil, err
	}
	defer folderRows.Close()
	for folderRows.Next() {
		var path string
		if err := folderRows.Scan(&path); err != nil {
			return nil, err
		}
		opts.Folders = append(opts.Folders, path)
	}

	// Content ratings
	crRows, err := r.db.Query(`
		SELECT DISTINCT content_rating FROM media_items
		WHERE library_id = $1 AND content_rating IS NOT NULL AND content_rating != ''
		ORDER BY content_rating`, libraryID)
	if err != nil {
		return nil, err
	}
	defer crRows.Close()
	for crRows.Next() {
		var cr string
		if err := crRows.Scan(&cr); err != nil {
			return nil, err
		}
		opts.ContentRatings = append(opts.ContentRatings, cr)
	}

	// Edition types
	edRows, err := r.db.Query(`
		SELECT DISTINCT edition_type FROM media_items
		WHERE library_id = $1 AND edition_type != ''
		ORDER BY edition_type`, libraryID)
	if err != nil {
		return nil, err
	}
	defer edRows.Close()
	for edRows.Next() {
		var ed string
		if err := edRows.Scan(&ed); err != nil {
			return nil, err
		}
		opts.Editions = append(opts.Editions, ed)
	}

	// Source types
	srcRows, err := r.db.Query(`
		SELECT DISTINCT source_type FROM media_items
		WHERE library_id = $1 AND source_type IS NOT NULL AND source_type != ''
		ORDER BY source_type`, libraryID)
	if err != nil {
		return nil, err
	}
	defer srcRows.Close()
	for srcRows.Next() {
		var src string
		if err := srcRows.Scan(&src); err != nil {
			return nil, err
		}
		opts.Sources = append(opts.Sources, src)
	}

	// Dynamic ranges
	drRows, err := r.db.Query(`
		SELECT DISTINCT dynamic_range FROM media_items
		WHERE library_id = $1 AND dynamic_range IS NOT NULL AND dynamic_range != ''
		ORDER BY dynamic_range`, libraryID)
	if err != nil {
		return nil, err
	}
	defer drRows.Close()
	for drRows.Next() {
		var dr string
		if err := drRows.Scan(&dr); err != nil {
			return nil, err
		}
		opts.DynamicRanges = append(opts.DynamicRanges, dr)
	}

	// Video codecs
	codecRows, err := r.db.Query(`
		SELECT DISTINCT codec FROM media_items
		WHERE library_id = $1 AND codec IS NOT NULL AND codec != ''
		ORDER BY codec`, libraryID)
	if err != nil {
		return nil, err
	}
	defer codecRows.Close()
	for codecRows.Next() {
		var c string
		if err := codecRows.Scan(&c); err != nil {
			return nil, err
		}
		opts.Codecs = append(opts.Codecs, c)
	}

	// HDR formats
	hdrRows, err := r.db.Query(`
		SELECT DISTINCT hdr_format FROM media_items
		WHERE library_id = $1 AND hdr_format IS NOT NULL AND hdr_format != ''
		ORDER BY hdr_format`, libraryID)
	if err != nil {
		return nil, err
	}
	defer hdrRows.Close()
	for hdrRows.Next() {
		var h string
		if err := hdrRows.Scan(&h); err != nil {
			return nil, err
		}
		opts.HDRFormats = append(opts.HDRFormats, h)
	}

	// Resolutions
	resRows, err := r.db.Query(`
		SELECT resolution FROM (
			SELECT DISTINCT resolution FROM media_items
			WHERE library_id = $1 AND resolution IS NOT NULL AND resolution != ''
		) sub
		ORDER BY CASE resolution WHEN '4K' THEN 1 WHEN '1080p' THEN 2 WHEN '720p' THEN 3 WHEN '480p' THEN 4 ELSE 5 END`, libraryID)
	if err != nil {
		return nil, err
	}
	defer resRows.Close()
	for resRows.Next() {
		var res string
		if err := resRows.Scan(&res); err != nil {
			return nil, err
		}
		opts.Resolutions = append(opts.Resolutions, res)
	}

	// Audio codecs
	acRows, err := r.db.Query(`
		SELECT DISTINCT audio_codec FROM media_items
		WHERE library_id = $1 AND audio_codec IS NOT NULL AND audio_codec != ''
		ORDER BY audio_codec`, libraryID)
	if err != nil {
		return nil, err
	}
	defer acRows.Close()
	for acRows.Next() {
		var ac string
		if err := acRows.Scan(&ac); err != nil {
			return nil, err
		}
		opts.AudioCodecs = append(opts.AudioCodecs, ac)
	}

	// Countries
	countryRows, err := r.db.Query(`
		SELECT DISTINCT country FROM media_items
		WHERE library_id = $1 AND country IS NOT NULL AND country != ''
		ORDER BY country`, libraryID)
	if err != nil {
		return nil, err
	}
	defer countryRows.Close()
	for countryRows.Next() {
		var c string
		if err := countryRows.Scan(&c); err != nil {
			return nil, err
		}
		opts.Countries = append(opts.Countries, c)
	}

	return opts, nil
}

// ClearItemMetadata resets all enriched metadata fields for a single item back to a
// clean state. Technical metadata (resolution, codec, duration, etc.) is preserved.
// The title is reset to the provided fileTitle (derived from the filename).
func (r *MediaRepository) ClearItemMetadata(id uuid.UUID, fileTitle string) error {
	query := `UPDATE media_items SET
		title = $1, sort_title = NULL, original_title = NULL, description = NULL,
		year = NULL, release_date = NULL, rating = NULL,
		poster_path = NULL, thumbnail_path = NULL, backdrop_path = NULL,
		generated_poster = false, imdb_rating = NULL, rt_rating = NULL, audience_score = NULL,
		content_rating = NULL, external_ids = NULL,
		updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`
	_, err := r.db.Exec(query, fileTitle, id)
	return err
}

// ClearItemMetadataWithLocks resets enriched metadata fields but preserves any
// per-field locked values. Extended metadata fields (tagline, language, etc.)
// are also cleared unless locked.
func (r *MediaRepository) ClearItemMetadataWithLocks(id uuid.UUID, fileTitle string, lockedFields pq.StringArray) error {
	query := `UPDATE media_items SET
		title = CASE WHEN $3::text[] IS NOT NULL AND ('title' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN title ELSE $1 END,
		sort_title = CASE WHEN $3::text[] IS NOT NULL AND ('title' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN sort_title ELSE NULL END,
		original_title = CASE WHEN $3::text[] IS NOT NULL AND ('title' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN original_title ELSE NULL END,
		description = CASE WHEN $3::text[] IS NOT NULL AND ('description' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN description ELSE NULL END,
		year = CASE WHEN $3::text[] IS NOT NULL AND ('year' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN year ELSE NULL END,
		release_date = CASE WHEN $3::text[] IS NOT NULL AND ('year' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN release_date ELSE NULL END,
		rating = CASE WHEN $3::text[] IS NOT NULL AND ('rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN rating ELSE NULL END,
		poster_path = CASE WHEN $3::text[] IS NOT NULL AND ('poster_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN poster_path ELSE NULL END,
		thumbnail_path = CASE WHEN $3::text[] IS NOT NULL AND ('poster_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN thumbnail_path ELSE NULL END,
		backdrop_path = CASE WHEN $3::text[] IS NOT NULL AND ('backdrop_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN backdrop_path ELSE NULL END,
		generated_poster = CASE WHEN $3::text[] IS NOT NULL AND ('poster_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN generated_poster ELSE false END,
		imdb_rating = CASE WHEN $3::text[] IS NOT NULL AND ('imdb_rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN imdb_rating ELSE NULL END,
		rt_rating = CASE WHEN $3::text[] IS NOT NULL AND ('rt_rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN rt_rating ELSE NULL END,
		audience_score = CASE WHEN $3::text[] IS NOT NULL AND ('audience_score' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN audience_score ELSE NULL END,
		content_rating = CASE WHEN $3::text[] IS NOT NULL AND ('content_rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN content_rating ELSE NULL END,
		external_ids = CASE WHEN $3::text[] IS NOT NULL AND ('external_ids' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN external_ids ELSE NULL END,
		tagline = CASE WHEN $3::text[] IS NOT NULL AND ('tagline' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN tagline ELSE NULL END,
		original_language = CASE WHEN $3::text[] IS NOT NULL AND ('original_language' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN original_language ELSE NULL END,
		country = CASE WHEN $3::text[] IS NOT NULL AND ('country' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN country ELSE NULL END,
		trailer_url = CASE WHEN $3::text[] IS NOT NULL AND ('trailer_url' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN trailer_url ELSE NULL END,
		logo_path = CASE WHEN $3::text[] IS NOT NULL AND ('logo_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN logo_path ELSE NULL END,
		custom_notes = CASE WHEN $3::text[] IS NOT NULL AND ('custom_notes' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN custom_notes ELSE NULL END,
		custom_tags = CASE WHEN $3::text[] IS NOT NULL AND ('custom_tags' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN custom_tags ELSE '{}' END,
		updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`
	_, err := r.db.Exec(query, fileTitle, id, lockedFields)
	return err
}

// RemoveAllMediaTags removes all tag links for a media item.
func (r *MediaRepository) RemoveAllMediaTags(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_tags WHERE media_item_id = $1`, id)
	return err
}

// RemoveAllMediaPerformers removes all performer links for a media item.
func (r *MediaRepository) RemoveAllMediaPerformers(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_performers WHERE media_item_id = $1`, id)
	return err
}

// ListAllByLibrary returns all media items in a library regardless of lock status.
func (r *MediaRepository) ListAllByLibrary(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items WHERE library_id = $1
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

// UpdateTechnicalMetadata sets source type, HDR format, and dynamic range for a media item.
func (r *MediaRepository) UpdateTechnicalMetadata(id uuid.UUID, sourceType, hdrFormat, dynamicRange *string) error {
	setClauses := []string{}
	args := []interface{}{}
	idx := 1

	if sourceType != nil {
		setClauses = append(setClauses, fmt.Sprintf("source_type = $%d", idx))
		args = append(args, *sourceType)
		idx++
	}
	if hdrFormat != nil {
		setClauses = append(setClauses, fmt.Sprintf("hdr_format = $%d", idx))
		args = append(args, *hdrFormat)
		idx++
	}
	if dynamicRange != nil {
		setClauses = append(setClauses, fmt.Sprintf("dynamic_range = $%d", idx))
		args = append(args, *dynamicRange)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), idx)
	args = append(args, id)
	_, err := r.db.Exec(query, args...)
	return err
}

// UpdateCustomNotes sets the custom_notes text for a media item.
func (r *MediaRepository) UpdateCustomNotes(id uuid.UUID, notes *string) error {
	_, err := r.db.Exec(`UPDATE media_items SET custom_notes = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, notes, id)
	return err
}

// UpdateCustomTags sets the custom_tags JSON for a media item.
func (r *MediaRepository) UpdateCustomTags(id uuid.UUID, tagsJSON string) error {
	_, err := r.db.Exec(`UPDATE media_items SET custom_tags = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, tagsJSON, id)
	return err
}

// BulkUpdateFields updates specific fields across multiple media items in a single transaction.
// The fields map should contain column names mapped to their new values.
func (r *MediaRepository) BulkUpdateFields(ids []uuid.UUID, fields map[string]interface{}) error {
	if len(ids) == 0 || len(fields) == 0 {
		return nil
	}

	// Whitelist of allowed columns
	allowed := map[string]bool{
		"rating": true, "edition_type": true, "custom_notes": true, "custom_tags": true,
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	for col, val := range fields {
		if !allowed[col] {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "metadata_locked = true", "updated_at = CURRENT_TIMESTAMP")

	// Build ID placeholders
	idPlaceholders := make([]string, len(ids))
	for i, id := range ids {
		idPlaceholders[i] = fmt.Sprintf("$%d", argIdx)
		args = append(args, id)
		argIdx++
	}

	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id IN (%s)",
		strings.Join(setClauses, ", "),
		strings.Join(idPlaceholders, ", "))

	_, err := r.db.Exec(query, args...)
	return err
}

// BulkDelete removes multiple media items by ID.
func (r *MediaRepository) BulkDelete(ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM media_items WHERE id IN (%s)", strings.Join(placeholders, ", "))
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// BulkGetCustomTags fetches custom_tags for multiple items (used for tag add/remove operations).
func (r *MediaRepository) BulkGetCustomTags(ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf("SELECT id, COALESCE(custom_tags, '') FROM media_items WHERE id IN (%s)", strings.Join(placeholders, ", "))
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uuid.UUID]string)
	for rows.Next() {
		var id uuid.UUID
		var tags string
		if err := rows.Scan(&id, &tags); err != nil {
			continue
		}
		result[id] = tags
	}
	return result, nil
}

// BulkGetCustomNotes fetches custom_notes for multiple items (used for note append operations).
func (r *MediaRepository) BulkGetCustomNotes(ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf("SELECT id, COALESCE(custom_notes, '') FROM media_items WHERE id IN (%s)", strings.Join(placeholders, ", "))
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uuid.UUID]string)
	for rows.Next() {
		var id uuid.UUID
		var notes string
		if err := rows.Scan(&id, &notes); err != nil {
			continue
		}
		result[id] = notes
	}
	return result, nil
}

// PopulateEditionCounts enriches a slice of MediaItems with edition group info.
// For each item that is the default edition in a group, sets EditionGroupID and EditionCount.
func (r *MediaRepository) PopulateEditionCounts(items []*models.MediaItem) error {
	if len(items) == 0 {
		return nil
	}

	// Build ID list
	ids := make([]interface{}, len(items))
	placeholders := make([]string, len(items))
	idMap := make(map[uuid.UUID]*models.MediaItem, len(items))
	for i, item := range items {
		ids[i] = item.ID
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		idMap[item.ID] = item
	}

	query := `SELECT ei.media_item_id, ei.edition_group_id, cnt.total
		FROM edition_items ei
		JOIN (SELECT edition_group_id, COUNT(*) AS total FROM edition_items GROUP BY edition_group_id) cnt
			ON cnt.edition_group_id = ei.edition_group_id
		WHERE ei.media_item_id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := r.db.Query(query, ids...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var mediaID, groupID uuid.UUID
		var total int
		if err := rows.Scan(&mediaID, &groupID, &total); err != nil {
			return err
		}
		if item, ok := idMap[mediaID]; ok {
			gid := groupID
			item.EditionGroupID = &gid
			item.EditionCount = total
		}
	}
	return rows.Err()
}
