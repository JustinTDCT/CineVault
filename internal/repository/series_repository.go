package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type SeriesRepository struct {
	db *sql.DB
}

func NewSeriesRepository(db *sql.DB) *SeriesRepository {
	return &SeriesRepository{db: db}
}

// ──────────────────── Movie Series ────────────────────

func (r *SeriesRepository) Create(s *models.MovieSeries) error {
	query := `
		INSERT INTO movie_series (id, library_id, name, poster_path)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, s.ID, s.LibraryID, s.Name, s.PosterPath).
		Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *SeriesRepository) GetByID(id uuid.UUID) (*models.MovieSeries, error) {
	s := &models.MovieSeries{}
	query := `
		SELECT id, library_id, name, poster_path, created_at, updated_at
		FROM movie_series WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&s.ID, &s.LibraryID, &s.Name, &s.PosterPath,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("series not found")
	}
	if err != nil {
		return nil, err
	}

	// Load items with media details
	items, err := r.ListItems(id)
	if err != nil {
		return nil, err
	}
	s.Items = items
	s.ItemCount = len(items)
	return s, nil
}

func (r *SeriesRepository) ListByLibrary(libraryID uuid.UUID) ([]*models.MovieSeries, error) {
	query := `
		SELECT s.id, s.library_id, s.name, s.poster_path, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM movie_series_items si WHERE si.series_id = s.id) as item_count
		FROM movie_series s
		WHERE s.library_id = $1
		ORDER BY s.name`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.MovieSeries
	for rows.Next() {
		s := &models.MovieSeries{}
		if err := rows.Scan(&s.ID, &s.LibraryID, &s.Name, &s.PosterPath,
			&s.CreatedAt, &s.UpdatedAt, &s.ItemCount); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *SeriesRepository) Update(s *models.MovieSeries) error {
	query := `
		UPDATE movie_series
		SET name = $1, poster_path = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3`
	result, err := r.db.Exec(query, s.Name, s.PosterPath, s.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series not found")
	}
	return nil
}

func (r *SeriesRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM movie_series WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series not found")
	}
	return nil
}

// ──────────────────── Series Items ────────────────────

func (r *SeriesRepository) AddItem(item *models.MovieSeriesItem) error {
	query := `
		INSERT INTO movie_series_items (id, series_id, media_item_id, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING added_at`
	return r.db.QueryRow(query, item.ID, item.SeriesID, item.MediaItemID, item.SortOrder).
		Scan(&item.AddedAt)
}

func (r *SeriesRepository) UpdateItemOrder(id uuid.UUID, sortOrder int) error {
	result, err := r.db.Exec(
		`UPDATE movie_series_items SET sort_order = $1 WHERE id = $2`,
		sortOrder, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series item not found")
	}
	return nil
}

func (r *SeriesRepository) ListItems(seriesID uuid.UUID) ([]models.MovieSeriesItem, error) {
	query := `
		SELECT si.id, si.series_id, si.media_item_id, si.sort_order, si.added_at,
		       m.title, m.year, m.poster_path
		FROM movie_series_items si
		JOIN media_items m ON m.id = si.media_item_id
		WHERE si.series_id = $1
		ORDER BY si.sort_order`
	rows, err := r.db.Query(query, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MovieSeriesItem
	for rows.Next() {
		item := models.MovieSeriesItem{}
		if err := rows.Scan(&item.ID, &item.SeriesID, &item.MediaItemID,
			&item.SortOrder, &item.AddedAt,
			&item.Title, &item.Year, &item.PosterPath); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListItemsRich returns full MediaItem data for each series member, with
// edition grouping applied (non-default edition children are excluded).
// Each returned item carries SeriesItemID and SeriesOrder for the frontend.
func (r *SeriesRepository) ListItemsRich(seriesID uuid.UUID) ([]*models.MediaItem, error) {
	// Use the standard media column list prefixed with "m."
	cols := strings.Split(mediaColumns, ",")
	for i, c := range cols {
		cols[i] = "m." + strings.TrimSpace(c)
	}
	mCols := strings.Join(cols, ", ")

	query := fmt.Sprintf(`
		SELECT %s, si.id AS series_item_id, si.sort_order
		FROM movie_series_items si
		JOIN media_items m ON m.id = si.media_item_id
		WHERE si.series_id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM edition_items ei
		      WHERE ei.media_item_id = m.id AND ei.is_default = false
		  )
		ORDER BY si.sort_order`, mCols)

	rows, err := r.db.Query(query, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		var siID uuid.UUID
		var sortOrder int
		err := rows.Scan(
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
			&item.AlbumArtist, &item.RecordingMBID,
			&item.AuthorID, &item.BookID, &item.ChapterNumber,
			&item.ImageGalleryID, &item.SisterGroupID, &item.Phash, &item.AudioFingerprint,
			&item.IMDBRating, &item.RTRating, &item.AudienceScore,
			&item.EditionType, &item.ContentRating, &item.SortPosition, &item.ExternalIDs, &item.GeneratedPoster,
			&item.SourceType, &item.HDRFormat, &item.DynamicRange, &item.Keywords,
			&item.MetacriticScore, &item.ContentRatingsJSON, &item.TaglinesJSON, &item.TrailersJSON, &item.DescriptionsJSON,
			&item.CustomNotes, &item.CustomTags,
			&item.MetadataLocked, &item.LockedFields, &item.DuplicateStatus,
			&item.PreviewPath, &item.SpritePath,
			&item.LoudnessLUFS, &item.LoudnessGainDB,
			&item.ParentMediaID, &item.ExtraType,
			&item.PlayCount, &item.LastPlayedAt,
			&item.AddedAt, &item.UpdatedAt,
			// Extra series-specific columns
			&siID, &sortOrder,
		)
		if err != nil {
			return nil, err
		}
		item.SeriesItemID = &siID
		item.SeriesOrder = &sortOrder
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SeriesRepository) RemoveItem(seriesID, itemID uuid.UUID) error {
	result, err := r.db.Exec(
		`DELETE FROM movie_series_items WHERE series_id = $1 AND id = $2`,
		seriesID, itemID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("series item not found")
	}
	return nil
}

// FindByExternalID finds a movie series by its TMDB collection ID stored in external_ids.
func (r *SeriesRepository) FindByExternalID(libraryID uuid.UUID, tmdbCollectionID string) (*models.MovieSeries, error) {
	s := &models.MovieSeries{}
	query := `
		SELECT id, library_id, name, description, poster_path, backdrop_path, external_ids, created_at, updated_at
		FROM movie_series
		WHERE library_id = $1 AND external_ids->>'tmdb_collection_id' = $2
		LIMIT 1`
	err := r.db.QueryRow(query, libraryID, tmdbCollectionID).Scan(
		&s.ID, &s.LibraryID, &s.Name, &s.Description, &s.PosterPath, &s.BackdropPath,
		&s.ExternalIDs, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// FindByName finds a movie series by name within a library.
func (r *SeriesRepository) FindByName(libraryID uuid.UUID, name string) (*models.MovieSeries, error) {
	s := &models.MovieSeries{}
	query := `
		SELECT id, library_id, name, description, poster_path, backdrop_path, external_ids, created_at, updated_at
		FROM movie_series WHERE library_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, libraryID, name).Scan(
		&s.ID, &s.LibraryID, &s.Name, &s.Description, &s.PosterPath, &s.BackdropPath,
		&s.ExternalIDs, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// IsItemInSeries checks if a media item is already linked to any series.
func (r *SeriesRepository) IsItemInSeries(mediaItemID uuid.UUID) bool {
	var count int
	r.db.QueryRow(`SELECT COUNT(*) FROM movie_series_items WHERE media_item_id = $1`, mediaItemID).Scan(&count)
	return count > 0
}

// GetSeriesForMedia returns the series a media item belongs to (if any)
func (r *SeriesRepository) GetSeriesForMedia(mediaItemID uuid.UUID) (*models.MovieSeries, *models.MovieSeriesItem, error) {
	item := &models.MovieSeriesItem{}
	query := `
		SELECT si.id, si.series_id, si.media_item_id, si.sort_order, si.added_at
		FROM movie_series_items si
		WHERE si.media_item_id = $1
		LIMIT 1`
	err := r.db.QueryRow(query, mediaItemID).Scan(
		&item.ID, &item.SeriesID, &item.MediaItemID, &item.SortOrder, &item.AddedAt)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	s := &models.MovieSeries{}
	sQuery := `
		SELECT id, library_id, name, poster_path, created_at, updated_at
		FROM movie_series WHERE id = $1`
	err = r.db.QueryRow(sQuery, item.SeriesID).Scan(
		&s.ID, &s.LibraryID, &s.Name, &s.PosterPath, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, nil, err
	}
	return s, item, nil
}
