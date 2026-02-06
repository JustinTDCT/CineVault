package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type MusicRepository struct {
	db *sql.DB
}

func NewMusicRepository(db *sql.DB) *MusicRepository {
	return &MusicRepository{db: db}
}

// ──────────────────── Artists ────────────────────

func (r *MusicRepository) CreateArtist(a *models.Artist) error {
	query := `
		INSERT INTO artists (id, library_id, name, sort_name, description, poster_path, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, a.ID, a.LibraryID, a.Name, a.SortName,
		a.Description, a.PosterPath, a.SortPosition).
		Scan(&a.CreatedAt, &a.UpdatedAt)
}

func (r *MusicRepository) GetArtistByID(id uuid.UUID) (*models.Artist, error) {
	a := &models.Artist{}
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path,
		       sort_position, created_at, updated_at
		FROM artists WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&a.ID, &a.LibraryID, &a.Name, &a.SortName, &a.Description,
		&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artist not found")
	}
	return a, err
}

func (r *MusicRepository) ListArtistsByLibrary(libraryID uuid.UUID) ([]*models.Artist, error) {
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path,
		       sort_position, created_at, updated_at
		FROM artists WHERE library_id = $1 ORDER BY COALESCE(sort_name, name)`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []*models.Artist
	for rows.Next() {
		a := &models.Artist{}
		if err := rows.Scan(&a.ID, &a.LibraryID, &a.Name, &a.SortName,
			&a.Description, &a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (r *MusicRepository) FindArtistByName(libraryID uuid.UUID, name string) (*models.Artist, error) {
	a := &models.Artist{}
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path,
		       sort_position, created_at, updated_at
		FROM artists WHERE library_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, libraryID, name).Scan(
		&a.ID, &a.LibraryID, &a.Name, &a.SortName, &a.Description,
		&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (r *MusicRepository) DeleteArtist(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM artists WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("artist not found")
	}
	return nil
}

// ──────────────────── Albums ────────────────────

func (r *MusicRepository) CreateAlbum(a *models.Album) error {
	query := `
		INSERT INTO albums (id, artist_id, library_id, title, sort_title, year, release_date,
		                    description, genre, poster_path, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, a.ID, a.ArtistID, a.LibraryID, a.Title, a.SortTitle,
		a.Year, a.ReleaseDate, a.Description, a.Genre, a.PosterPath, a.SortPosition).
		Scan(&a.CreatedAt, &a.UpdatedAt)
}

func (r *MusicRepository) GetAlbumByID(id uuid.UUID) (*models.Album, error) {
	a := &models.Album{}
	query := `
		SELECT id, artist_id, library_id, title, sort_title, year, release_date,
		       description, genre, poster_path, sort_position, created_at, updated_at
		FROM albums WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&a.ID, &a.ArtistID, &a.LibraryID, &a.Title, &a.SortTitle,
		&a.Year, &a.ReleaseDate, &a.Description, &a.Genre,
		&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("album not found")
	}
	return a, err
}

func (r *MusicRepository) ListAlbumsByArtist(artistID uuid.UUID) ([]*models.Album, error) {
	query := `
		SELECT id, artist_id, library_id, title, sort_title, year, release_date,
		       description, genre, poster_path, sort_position, created_at, updated_at
		FROM albums WHERE artist_id = $1 ORDER BY COALESCE(year, 0), COALESCE(sort_title, title)`
	rows, err := r.db.Query(query, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*models.Album
	for rows.Next() {
		a := &models.Album{}
		if err := rows.Scan(&a.ID, &a.ArtistID, &a.LibraryID, &a.Title, &a.SortTitle,
			&a.Year, &a.ReleaseDate, &a.Description, &a.Genre,
			&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (r *MusicRepository) FindAlbumByTitle(artistID uuid.UUID, title string) (*models.Album, error) {
	a := &models.Album{}
	query := `
		SELECT id, artist_id, library_id, title, sort_title, year, release_date,
		       description, genre, poster_path, sort_position, created_at, updated_at
		FROM albums WHERE artist_id = $1 AND LOWER(title) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, artistID, title).Scan(
		&a.ID, &a.ArtistID, &a.LibraryID, &a.Title, &a.SortTitle,
		&a.Year, &a.ReleaseDate, &a.Description, &a.Genre,
		&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (r *MusicRepository) DeleteAlbum(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM albums WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("album not found")
	}
	return nil
}
