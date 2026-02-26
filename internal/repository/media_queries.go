package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

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

	// Collapse sister groups: only show the primary item (lowest sort_position) per group
	wheres = append(wheres, `(m.sister_group_id IS NULL OR m.id = (
		SELECT id FROM media_items
		WHERE sister_group_id = m.sister_group_id
		ORDER BY sort_position ASC, title ASC
		LIMIT 1
	))`)

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
		case "track_number":
			orderCol = "m.track_number"
		case "artist":
			joins = append(joins, `LEFT JOIN artists _sort_ar ON _sort_ar.id = m.artist_id`)
			orderCol = "COALESCE(_sort_ar.name, '')"
		case "album":
			joins = append(joins, `LEFT JOIN albums _sort_al ON _sort_al.id = m.album_id`)
			orderCol = "COALESCE(_sort_al.title, '')"
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
	params := []interface{}{"%" + query + "%"}
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

// ListItemsNeedingLoudness returns media items in a library that haven't been analyzed yet.
func (r *MediaRepository) ListItemsNeedingLoudness(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE library_id = $1
		  AND loudness_lufs IS NULL
		  AND media_type NOT IN ('images')
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

// ListItemsNeedingSprites returns video items in a library that have no sprite_path.
func (r *MediaRepository) ListItemsNeedingSprites(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE library_id = $1
		  AND (sprite_path IS NULL OR sprite_path = '')
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

// ListItemsNeedingPreviews returns video items in a library that have no preview_path and duration >= 30s.
func (r *MediaRepository) ListItemsNeedingPreviews(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE library_id = $1
		  AND (preview_path IS NULL OR preview_path = '')
		  AND duration_seconds >= 30
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

// ListAllItemsForPreviews returns all video items eligible for previews (duration >= 30s),
// including those that already have a preview_path. Used for rebuild operations.
func (r *MediaRepository) ListAllItemsForPreviews(libraryID uuid.UUID) ([]*models.MediaItem, error) {
	query := `SELECT ` + mediaColumns + `
		FROM media_items
		WHERE library_id = $1
		  AND duration_seconds >= 30
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

// PopulateSisterInfo enriches items that belong to a sister group with part count,
// combined duration, and the group name.
func (r *MediaRepository) PopulateSisterInfo(items []*models.MediaItem) error {
	if len(items) == 0 {
		return nil
	}

	// Collect sister group IDs
	groupIDs := make(map[uuid.UUID]struct{})
	idMap := make(map[uuid.UUID]*models.MediaItem, len(items))
	for _, item := range items {
		idMap[item.ID] = item
		if item.SisterGroupID != nil {
			groupIDs[*item.SisterGroupID] = struct{}{}
		}
	}
	if len(groupIDs) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(groupIDs))
	args := make([]interface{}, 0, len(groupIDs))
	i := 1
	for gid := range groupIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		args = append(args, gid)
		i++
	}

	query := `SELECT sg.id, sg.name, COUNT(mi.id), COALESCE(SUM(mi.duration_seconds), 0)
		FROM sister_groups sg
		JOIN media_items mi ON mi.sister_group_id = sg.id
		WHERE sg.id IN (` + strings.Join(placeholders, ",") + `)
		GROUP BY sg.id, sg.name`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type sisterInfo struct {
		name          string
		partCount     int
		totalDuration int
	}
	infoMap := make(map[uuid.UUID]sisterInfo)
	for rows.Next() {
		var gid uuid.UUID
		var info sisterInfo
		if err := rows.Scan(&gid, &info.name, &info.partCount, &info.totalDuration); err != nil {
			return err
		}
		infoMap[gid] = info
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		if item.SisterGroupID == nil {
			continue
		}
		if info, ok := infoMap[*item.SisterGroupID]; ok {
			item.SisterPartCount = info.partCount
			item.SisterTotalDuration = info.totalDuration
			item.SisterGroupName = info.name
		}
	}
	return nil
}

// PopulateMusicInfo enriches music items with artist name and album title.
func (r *MediaRepository) PopulateMusicInfo(items []*models.MediaItem) error {
	if len(items) == 0 {
		return nil
	}

	artistIDs := make(map[uuid.UUID]struct{})
	albumIDs := make(map[uuid.UUID]struct{})
	for _, item := range items {
		if item.ArtistID != nil {
			artistIDs[*item.ArtistID] = struct{}{}
		}
		if item.AlbumID != nil {
			albumIDs[*item.AlbumID] = struct{}{}
		}
	}

	artistNames := make(map[uuid.UUID]string)
	if len(artistIDs) > 0 {
		ph := make([]string, 0, len(artistIDs))
		args := make([]interface{}, 0, len(artistIDs))
		i := 1
		for id := range artistIDs {
			ph = append(ph, fmt.Sprintf("$%d", i))
			args = append(args, id)
			i++
		}
		rows, err := r.db.Query(`SELECT id, name FROM artists WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				return err
			}
			artistNames[id] = name
		}
	}

	albumTitles := make(map[uuid.UUID]string)
	if len(albumIDs) > 0 {
		ph := make([]string, 0, len(albumIDs))
		args := make([]interface{}, 0, len(albumIDs))
		i := 1
		for id := range albumIDs {
			ph = append(ph, fmt.Sprintf("$%d", i))
			args = append(args, id)
			i++
		}
		rows, err := r.db.Query(`SELECT id, title FROM albums WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var title string
			if err := rows.Scan(&id, &title); err != nil {
				return err
			}
			albumTitles[id] = title
		}
	}

	for _, item := range items {
		if item.ArtistID != nil {
			if name, ok := artistNames[*item.ArtistID]; ok {
				item.ArtistName = name
			}
		}
		if item.AlbumID != nil {
			if title, ok := albumTitles[*item.AlbumID]; ok {
				item.AlbumTitle = title
			}
		}
	}
	return nil
}
