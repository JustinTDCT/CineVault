package repository

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
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

// seasonEpisodeRow holds aggregated episode data per season from a single query.
type seasonEpisodeRow struct {
	ShowID       uuid.UUID
	ShowTitle    string
	PosterPath   *string
	SeasonID     uuid.UUID
	SeasonNumber int
	EpisodeCount int
	EpisodeNums  []int
}

// GetMissingEpisodes detects missing episode numbers for all shows in a library.
// Uses a single aggregated query instead of N+1 per-season queries.
func (r *TVRepository) GetMissingEpisodes(libraryID uuid.UUID) ([]MissingEpisodesShowResult, error) {
	query := `
		SELECT s.id AS show_id, s.title AS show_title, s.poster_path,
		       sn.id AS season_id, sn.season_number, sn.episode_count,
		       ARRAY_AGG(m.episode_number ORDER BY m.episode_number) AS episode_nums
		FROM tv_shows s
		JOIN tv_seasons sn ON sn.tv_show_id = s.id
		JOIN media_items m ON m.tv_season_id = sn.id AND m.episode_number IS NOT NULL
		WHERE s.library_id = $1
		GROUP BY s.id, s.title, s.poster_path, sn.id, sn.season_number, sn.episode_count
		ORDER BY s.title, sn.season_number`

	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect results grouped by show
	showMap := make(map[uuid.UUID]*MissingEpisodesShowResult)
	var showOrder []uuid.UUID

	for rows.Next() {
		var row seasonEpisodeRow
		var epNums pq.Int64Array
		if err := rows.Scan(&row.ShowID, &row.ShowTitle, &row.PosterPath,
			&row.SeasonID, &row.SeasonNumber, &row.EpisodeCount,
			&epNums); err != nil {
			return nil, err
		}

		haveEps := make([]int, len(epNums))
		for i, v := range epNums {
			haveEps[i] = int(v)
		}

		if len(haveEps) == 0 {
			continue
		}

		sort.Ints(haveEps)
		maxEp := haveEps[len(haveEps)-1]
		expectedCount := maxEp
		if row.EpisodeCount > maxEp {
			expectedCount = row.EpisodeCount
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

		if len(missing) == 0 {
			continue
		}

		if _, exists := showMap[row.ShowID]; !exists {
			showMap[row.ShowID] = &MissingEpisodesShowResult{
				ShowID:     row.ShowID,
				ShowTitle:  row.ShowTitle,
				PosterPath: row.PosterPath,
			}
			showOrder = append(showOrder, row.ShowID)
		}

		sr := showMap[row.ShowID]
		sr.Seasons = append(sr.Seasons, MissingEpisodesResult{
			ShowID:         row.ShowID,
			ShowTitle:      row.ShowTitle,
			SeasonID:       row.SeasonID,
			SeasonNumber:   row.SeasonNumber,
			HaveCount:      len(haveEps),
			ExpectedCount:  expectedCount,
			MissingNumbers: missing,
		})
		sr.TotalMissing += len(missing)
	}

	var results []MissingEpisodesShowResult
	for _, id := range showOrder {
		results = append(results, *showMap[id])
	}
	return results, nil
}

// GetShowMissingEpisodes detects missing episodes for a single TV show (fast path).
func (r *TVRepository) GetShowMissingEpisodes(showID uuid.UUID) (*MissingEpisodesShowResult, error) {
	show, err := r.GetShowByID(showID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT sn.id AS season_id, sn.season_number, sn.episode_count,
		       ARRAY_AGG(m.episode_number ORDER BY m.episode_number) AS episode_nums
		FROM tv_seasons sn
		JOIN media_items m ON m.tv_season_id = sn.id AND m.episode_number IS NOT NULL
		WHERE sn.tv_show_id = $1
		GROUP BY sn.id, sn.season_number, sn.episode_count
		ORDER BY sn.season_number`

	rows, err := r.db.Query(query, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &MissingEpisodesShowResult{
		ShowID:     show.ID,
		ShowTitle:  show.Title,
		PosterPath: show.PosterPath,
	}

	for rows.Next() {
		var seasonID uuid.UUID
		var seasonNumber, episodeCount int
		var epNums pq.Int64Array

		if err := rows.Scan(&seasonID, &seasonNumber, &episodeCount, &epNums); err != nil {
			return nil, err
		}

		haveEps := make([]int, len(epNums))
		for i, v := range epNums {
			haveEps[i] = int(v)
		}

		if len(haveEps) == 0 {
			continue
		}

		sort.Ints(haveEps)
		maxEp := haveEps[len(haveEps)-1]
		expectedCount := maxEp
		if episodeCount > maxEp {
			expectedCount = episodeCount
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

		if len(missing) > 0 {
			result.Seasons = append(result.Seasons, MissingEpisodesResult{
				ShowID:         show.ID,
				ShowTitle:      show.Title,
				SeasonID:       seasonID,
				SeasonNumber:   seasonNumber,
				HaveCount:      len(haveEps),
				ExpectedCount:  expectedCount,
				MissingNumbers: missing,
			})
			result.TotalMissing += len(missing)
		}
	}

	return result, nil
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
