package repository

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// MissingEpisodesResult holds gap detection results for a single season.
type MissingEpisodesResult struct {
	ShowID         uuid.UUID `json:"show_id"`
	ShowTitle      string    `json:"show_title"`
	SeasonID       uuid.UUID `json:"season_id"`
	SeasonNumber   int       `json:"season_number"`
	HaveCount      int       `json:"have_count"`
	ExpectedCount  int       `json:"expected_count"`
	MissingNumbers []int     `json:"missing_numbers"`
}

// MissingEpisodesShowResult groups missing episode data for a TV show.
type MissingEpisodesShowResult struct {
	ShowID        uuid.UUID                `json:"show_id"`
	ShowTitle     string                   `json:"show_title"`
	PosterPath    *string                  `json:"poster_path,omitempty"`
	TotalMissing  int                      `json:"total_missing"`
	Seasons       []MissingEpisodesResult  `json:"seasons"`
}

type TVRepository struct {
	db *sql.DB
}

func NewTVRepository(db *sql.DB) *TVRepository {
	return &TVRepository{db: db}
}

// ──────────────────── TV Shows ────────────────────

func (r *TVRepository) CreateShow(show *models.TVShow) error {
	query := `
		INSERT INTO tv_shows (id, library_id, title, sort_title, original_title, description,
		                      year, first_air_date, last_air_date, status, poster_path, backdrop_path, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, show.ID, show.LibraryID, show.Title, show.SortTitle,
		show.OriginalTitle, show.Description, show.Year, show.FirstAirDate, show.LastAirDate,
		show.Status, show.PosterPath, show.BackdropPath, show.SortPosition).
		Scan(&show.CreatedAt, &show.UpdatedAt)
}

func (r *TVRepository) GetShowByID(id uuid.UUID) (*models.TVShow, error) {
	show := &models.TVShow{}
	query := `
		SELECT id, library_id, title, sort_title, original_title, description,
		       year, first_air_date, last_air_date, status, poster_path, backdrop_path,
		       sort_position, created_at, updated_at
		FROM tv_shows WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&show.ID, &show.LibraryID, &show.Title, &show.SortTitle, &show.OriginalTitle,
		&show.Description, &show.Year, &show.FirstAirDate, &show.LastAirDate,
		&show.Status, &show.PosterPath, &show.BackdropPath,
		&show.SortPosition, &show.CreatedAt, &show.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tv show not found")
	}
	return show, err
}

func (r *TVRepository) ListShowsByLibrary(libraryID uuid.UUID) ([]*models.TVShow, error) {
	query := `
		SELECT id, library_id, title, sort_title, original_title, description,
		       year, first_air_date, last_air_date, status, poster_path, backdrop_path,
		       sort_position, created_at, updated_at
		FROM tv_shows WHERE library_id = $1 ORDER BY COALESCE(sort_title, title)`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shows []*models.TVShow
	for rows.Next() {
		show := &models.TVShow{}
		if err := rows.Scan(&show.ID, &show.LibraryID, &show.Title, &show.SortTitle,
			&show.OriginalTitle, &show.Description, &show.Year, &show.FirstAirDate,
			&show.LastAirDate, &show.Status, &show.PosterPath, &show.BackdropPath,
			&show.SortPosition, &show.CreatedAt, &show.UpdatedAt); err != nil {
			return nil, err
		}
		shows = append(shows, show)
	}
	return shows, rows.Err()
}

func (r *TVRepository) FindShowByTitle(libraryID uuid.UUID, title string) (*models.TVShow, error) {
	show := &models.TVShow{}
	query := `
		SELECT id, library_id, title, sort_title, original_title, description,
		       year, first_air_date, last_air_date, status, poster_path, backdrop_path,
		       sort_position, created_at, updated_at
		FROM tv_shows WHERE library_id = $1 AND LOWER(title) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, libraryID, title).Scan(
		&show.ID, &show.LibraryID, &show.Title, &show.SortTitle, &show.OriginalTitle,
		&show.Description, &show.Year, &show.FirstAirDate, &show.LastAirDate,
		&show.Status, &show.PosterPath, &show.BackdropPath,
		&show.SortPosition, &show.CreatedAt, &show.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return show, err
}

func (r *TVRepository) UpdateShowMetadata(id uuid.UUID, title string, year *int, description *string, rating *float64, posterPath *string) error {
	query := `UPDATE tv_shows SET title = $1, year = $2, description = $3,
		poster_path = $4, updated_at = CURRENT_TIMESTAMP WHERE id = $5`
	_, err := r.db.Exec(query, title, year, description, posterPath, id)
	return err
}

func (r *TVRepository) DeleteShow(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM tv_shows WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tv show not found")
	}
	return nil
}

// ──────────────────── TV Seasons ────────────────────

func (r *TVRepository) CreateSeason(season *models.TVSeason) error {
	query := `
		INSERT INTO tv_seasons (id, tv_show_id, season_number, title, description, air_date,
		                        episode_count, poster_path, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, season.ID, season.TVShowID, season.SeasonNumber,
		season.Title, season.Description, season.AirDate, season.EpisodeCount,
		season.PosterPath, season.SortPosition).
		Scan(&season.CreatedAt, &season.UpdatedAt)
}

func (r *TVRepository) GetSeasonByID(id uuid.UUID) (*models.TVSeason, error) {
	season := &models.TVSeason{}
	query := `
		SELECT id, tv_show_id, season_number, title, description, air_date,
		       episode_count, poster_path, sort_position, created_at, updated_at
		FROM tv_seasons WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&season.ID, &season.TVShowID, &season.SeasonNumber, &season.Title,
		&season.Description, &season.AirDate, &season.EpisodeCount,
		&season.PosterPath, &season.SortPosition, &season.CreatedAt, &season.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tv season not found")
	}
	return season, err
}

func (r *TVRepository) ListSeasonsByShow(showID uuid.UUID) ([]*models.TVSeason, error) {
	query := `
		SELECT id, tv_show_id, season_number, title, description, air_date,
		       episode_count, poster_path, sort_position, created_at, updated_at
		FROM tv_seasons WHERE tv_show_id = $1 ORDER BY season_number`
	rows, err := r.db.Query(query, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []*models.TVSeason
	for rows.Next() {
		season := &models.TVSeason{}
		if err := rows.Scan(&season.ID, &season.TVShowID, &season.SeasonNumber,
			&season.Title, &season.Description, &season.AirDate, &season.EpisodeCount,
			&season.PosterPath, &season.SortPosition, &season.CreatedAt, &season.UpdatedAt); err != nil {
			return nil, err
		}
		seasons = append(seasons, season)
	}
	return seasons, rows.Err()
}

func (r *TVRepository) FindSeason(showID uuid.UUID, seasonNumber int) (*models.TVSeason, error) {
	season := &models.TVSeason{}
	query := `
		SELECT id, tv_show_id, season_number, title, description, air_date,
		       episode_count, poster_path, sort_position, created_at, updated_at
		FROM tv_seasons WHERE tv_show_id = $1 AND season_number = $2`
	err := r.db.QueryRow(query, showID, seasonNumber).Scan(
		&season.ID, &season.TVShowID, &season.SeasonNumber, &season.Title,
		&season.Description, &season.AirDate, &season.EpisodeCount,
		&season.PosterPath, &season.SortPosition, &season.CreatedAt, &season.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return season, err
}

func (r *TVRepository) IncrementEpisodeCount(seasonID uuid.UUID) error {
	_, err := r.db.Exec(`UPDATE tv_seasons SET episode_count = episode_count + 1 WHERE id = $1`, seasonID)
	return err
}

func (r *TVRepository) UpdateSeasonMetadata(id uuid.UUID, title *string, description *string, posterPath *string) error {
	query := `UPDATE tv_seasons SET title = COALESCE($1, title), description = COALESCE($2, description),
		poster_path = COALESCE($3, poster_path), updated_at = CURRENT_TIMESTAMP WHERE id = $4`
	_, err := r.db.Exec(query, title, description, posterPath, id)
	return err
}

// ListEpisodesBySeason returns media items for a given season, ordered by episode number.
func (r *TVRepository) ListEpisodesBySeason(seasonID uuid.UUID) ([]*models.MediaItem, error) {
	query := `
		SELECT id, library_id, media_type, file_path, file_name, file_size, file_hash,
		       title, sort_title, original_title, description, year, release_date,
		       duration_seconds, rating, resolution, width, height, codec, container,
		       bitrate, framerate, audio_codec, audio_channels, poster_path, thumbnail_path,
		       backdrop_path, tv_show_id, tv_season_id, episode_number,
		       artist_id, album_id, track_number, disc_number,
		       author_id, book_id, chapter_number, image_gallery_id,
		       sister_group_id, phash, audio_fingerprint, sort_position, added_at, updated_at, last_scanned_at
		FROM media_items WHERE tv_season_id = $1
		ORDER BY COALESCE(episode_number, 0), title`
	rows, err := r.db.Query(query, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		item := &models.MediaItem{}
		if err := rows.Scan(
			&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName,
			&item.FileSize, &item.FileHash, &item.Title, &item.SortTitle, &item.OriginalTitle,
			&item.Description, &item.Year, &item.ReleaseDate, &item.DurationSeconds,
			&item.Rating, &item.Resolution, &item.Width, &item.Height, &item.Codec,
			&item.Container, &item.Bitrate, &item.Framerate, &item.AudioCodec,
			&item.AudioChannels, &item.PosterPath, &item.ThumbnailPath, &item.BackdropPath,
			&item.TVShowID, &item.TVSeasonID, &item.EpisodeNumber,
			&item.ArtistID, &item.AlbumID, &item.TrackNumber, &item.DiscNumber,
			&item.AuthorID, &item.BookID, &item.ChapterNumber, &item.ImageGalleryID,
			&item.SisterGroupID, &item.Phash, &item.AudioFingerprint,
			&item.SortPosition, &item.AddedAt, &item.UpdatedAt, &item.LastScannedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetMissingEpisodes detects missing episode numbers for all shows in a library.
// It compares actual episode numbers against the expected range (1..max episode number found).
func (r *TVRepository) GetMissingEpisodes(libraryID uuid.UUID) ([]MissingEpisodesShowResult, error) {
	// Get all shows in library
	shows, err := r.ListShowsByLibrary(libraryID)
	if err != nil {
		return nil, err
	}

	var results []MissingEpisodesShowResult

	for _, show := range shows {
		seasons, err := r.ListSeasonsByShow(show.ID)
		if err != nil {
			return nil, err
		}

		var showSeasons []MissingEpisodesResult
		totalMissing := 0

		for _, season := range seasons {
			// Get actual episode numbers for this season
			rows, err := r.db.Query(`
				SELECT episode_number FROM media_items
				WHERE tv_season_id = $1 AND episode_number IS NOT NULL
				ORDER BY episode_number`, season.ID)
			if err != nil {
				return nil, err
			}

			var haveEps []int
			for rows.Next() {
				var epNum int
				if err := rows.Scan(&epNum); err != nil {
					rows.Close()
					return nil, err
				}
				haveEps = append(haveEps, epNum)
			}
			rows.Close()

			if len(haveEps) == 0 {
				continue
			}

			sort.Ints(haveEps)
			maxEp := haveEps[len(haveEps)-1]

			// Use the season's stored episode_count as expected if it's higher than max found
			expectedCount := maxEp
			if season.EpisodeCount > maxEp {
				expectedCount = season.EpisodeCount
			}

			// Build set of present episodes for fast lookup
			haveSet := make(map[int]bool, len(haveEps))
			for _, ep := range haveEps {
				haveSet[ep] = true
			}

			// Find missing numbers from 1..expectedCount
			var missing []int
			for i := 1; i <= expectedCount; i++ {
				if !haveSet[i] {
					missing = append(missing, i)
				}
			}

			if len(missing) > 0 {
				showSeasons = append(showSeasons, MissingEpisodesResult{
					ShowID:         show.ID,
					ShowTitle:      show.Title,
					SeasonID:       season.ID,
					SeasonNumber:   season.SeasonNumber,
					HaveCount:      len(haveEps),
					ExpectedCount:  expectedCount,
					MissingNumbers: missing,
				})
				totalMissing += len(missing)
			}
		}

		if len(showSeasons) > 0 {
			results = append(results, MissingEpisodesShowResult{
				ShowID:       show.ID,
				ShowTitle:    show.Title,
				PosterPath:   show.PosterPath,
				TotalMissing: totalMissing,
				Seasons:      showSeasons,
			})
		}
	}

	return results, nil
}

// GetSeasonMissingEpisodes detects missing episode numbers for a specific season.
func (r *TVRepository) GetSeasonMissingEpisodes(seasonID uuid.UUID) (*MissingEpisodesResult, error) {
	season, err := r.GetSeasonByID(seasonID)
	if err != nil {
		return nil, err
	}

	show, err := r.GetShowByID(season.TVShowID)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(`
		SELECT episode_number FROM media_items
		WHERE tv_season_id = $1 AND episode_number IS NOT NULL
		ORDER BY episode_number`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var haveEps []int
	for rows.Next() {
		var epNum int
		if err := rows.Scan(&epNum); err != nil {
			return nil, err
		}
		haveEps = append(haveEps, epNum)
	}

	if len(haveEps) == 0 {
		return &MissingEpisodesResult{
			ShowID:       show.ID,
			ShowTitle:    show.Title,
			SeasonID:     season.ID,
			SeasonNumber: season.SeasonNumber,
			HaveCount:    0,
			ExpectedCount: season.EpisodeCount,
		}, nil
	}

	sort.Ints(haveEps)
	maxEp := haveEps[len(haveEps)-1]
	expectedCount := maxEp
	if season.EpisodeCount > maxEp {
		expectedCount = season.EpisodeCount
	}

	haveSet := make(map[int]bool, len(haveEps))
	for _, ep := range haveEps {
		haveSet[ep] = true
	}

	var missing []int
	for i := 1; i <= expectedCount; i++ {
		if !haveSet[i] {
			missing = append(missing, i)
		}
	}

	return &MissingEpisodesResult{
		ShowID:         show.ID,
		ShowTitle:      show.Title,
		SeasonID:       season.ID,
		SeasonNumber:   season.SeasonNumber,
		HaveCount:      len(haveEps),
		ExpectedCount:  expectedCount,
		MissingNumbers: missing,
	}, nil
}
