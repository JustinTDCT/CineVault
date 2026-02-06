package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

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
