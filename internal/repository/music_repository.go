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
		INSERT INTO artists (id, library_id, name, sort_name, description, poster_path, mbid, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, a.ID, a.LibraryID, a.Name, a.SortName,
		a.Description, a.PosterPath, a.MBID, a.SortPosition).
		Scan(&a.CreatedAt, &a.UpdatedAt)
}

func (r *MusicRepository) GetArtistByID(id uuid.UUID) (*models.Artist, error) {
	a := &models.Artist{}
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path, mbid,
		       sort_position, created_at, updated_at
		FROM artists WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&a.ID, &a.LibraryID, &a.Name, &a.SortName, &a.Description,
		&a.PosterPath, &a.MBID, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artist not found")
	}
	return a, err
}

func (r *MusicRepository) ListArtistsByLibrary(libraryID uuid.UUID) ([]*models.Artist, error) {
	query := `
		SELECT a.id, a.library_id, a.name, a.sort_name, a.description, a.poster_path, a.mbid,
		       a.sort_position, a.created_at, a.updated_at,
		       COUNT(DISTINCT al.id) AS album_count,
		       COUNT(DISTINCT m.id) AS track_count
		FROM artists a
		LEFT JOIN albums al ON al.artist_id = a.id
		LEFT JOIN media_items m ON m.artist_id = a.id
		WHERE a.library_id = $1
		GROUP BY a.id
		ORDER BY COALESCE(a.sort_name, a.name)`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []*models.Artist
	for rows.Next() {
		a := &models.Artist{}
		if err := rows.Scan(&a.ID, &a.LibraryID, &a.Name, &a.SortName,
			&a.Description, &a.PosterPath, &a.MBID, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
			&a.AlbumCount, &a.TrackCount); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (r *MusicRepository) FindArtistByName(libraryID uuid.UUID, name string) (*models.Artist, error) {
	a := &models.Artist{}
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path, mbid,
		       sort_position, created_at, updated_at
		FROM artists WHERE library_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, libraryID, name).Scan(
		&a.ID, &a.LibraryID, &a.Name, &a.SortName, &a.Description,
		&a.PosterPath, &a.MBID, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
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

func (r *MusicRepository) UpdateArtistMBID(artistID uuid.UUID, mbid string) error {
	_, err := r.db.Exec(`UPDATE artists SET mbid = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		mbid, artistID)
	return err
}

func (r *MusicRepository) UpdateArtistPosterPath(artistID uuid.UUID, posterPath string) error {
	_, err := r.db.Exec(`UPDATE artists SET poster_path = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		posterPath, artistID)
	return err
}

func (r *MusicRepository) ListArtistsWithoutPosters(libraryID uuid.UUID) ([]*models.Artist, error) {
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path, mbid,
		       sort_position, created_at, updated_at
		FROM artists
		WHERE library_id = $1 AND poster_path IS NULL
		ORDER BY COALESCE(sort_name, name)`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []*models.Artist
	for rows.Next() {
		a := &models.Artist{}
		if err := rows.Scan(&a.ID, &a.LibraryID, &a.Name, &a.SortName,
			&a.Description, &a.PosterPath, &a.MBID, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
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
		SELECT al.id, al.artist_id, al.library_id, al.title, al.sort_title, al.year,
		       al.release_date, al.description, al.genre, al.poster_path,
		       al.sort_position, al.created_at, al.updated_at,
		       COALESCE((SELECT COUNT(*) FROM media_items m WHERE m.album_id = al.id), 0) AS track_count,
		       COALESCE(ar.name, '') AS artist_name
		FROM albums al
		LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&a.ID, &a.ArtistID, &a.LibraryID, &a.Title, &a.SortTitle,
		&a.Year, &a.ReleaseDate, &a.Description, &a.Genre,
		&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
		&a.TrackCount, &a.ArtistName,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("album not found")
	}
	return a, err
}

func (r *MusicRepository) ListAlbumsByArtist(artistID uuid.UUID) ([]*models.Album, error) {
	query := `
		SELECT al.id, al.artist_id, al.library_id, al.title, al.sort_title, al.year,
		       al.release_date, al.description, al.genre, al.poster_path,
		       al.sort_position, al.created_at, al.updated_at,
		       COUNT(m.id) AS track_count,
		       COALESCE(ar.name, '') AS artist_name
		FROM albums al
		LEFT JOIN media_items m ON m.album_id = al.id
		LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.artist_id = $1
		GROUP BY al.id, ar.name
		ORDER BY COALESCE(al.year, 0), COALESCE(al.sort_title, al.title)`
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
			&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
			&a.TrackCount, &a.ArtistName); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (r *MusicRepository) ListTracksByAlbum(albumID uuid.UUID) ([]*models.MediaItem, error) {
	query := `
		SELECT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.year, m.duration_seconds, m.audio_codec, m.audio_format,
		       m.bitrate, m.container, m.artist_id, m.album_id,
		       m.track_number, m.disc_number, m.poster_path, m.added_at, m.updated_at,
		       COALESCE(m.album_artist, ar.name, '') AS artist_name,
		       COALESCE(al.title, '') AS album_title
		FROM media_items m
		LEFT JOIN artists ar ON ar.id = m.artist_id
		LEFT JOIN albums al ON al.id = m.album_id
		WHERE m.album_id = $1
		ORDER BY COALESCE(m.disc_number, 1), COALESCE(m.track_number, 0), m.title`
	rows, err := r.db.Query(query, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		m := &models.MediaItem{}
		if err := rows.Scan(
			&m.ID, &m.LibraryID, &m.MediaType, &m.FilePath, &m.FileName, &m.FileSize,
			&m.Title, &m.Year, &m.DurationSeconds, &m.AudioCodec, &m.AudioFormat,
			&m.Bitrate, &m.Container, &m.ArtistID, &m.AlbumID,
			&m.TrackNumber, &m.DiscNumber, &m.PosterPath, &m.AddedAt, &m.UpdatedAt,
			&m.ArtistName, &m.AlbumTitle,
		); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
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

func (r *MusicRepository) ListAlbumsByLibrary(libraryID uuid.UUID) ([]*models.Album, error) {
	query := `
		SELECT al.id, al.artist_id, al.library_id, al.title, al.sort_title, al.year,
		       al.release_date, al.description, al.genre, al.poster_path,
		       al.sort_position, al.created_at, al.updated_at,
		       COUNT(m.id) AS track_count,
		       COALESCE(ar.name, '') AS artist_name
		FROM albums al
		LEFT JOIN media_items m ON m.album_id = al.id
		LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.library_id = $1
		GROUP BY al.id, ar.name
		ORDER BY COALESCE(al.sort_title, al.title)`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*models.Album
	for rows.Next() {
		a := &models.Album{}
		if err := rows.Scan(&a.ID, &a.ArtistID, &a.LibraryID, &a.Title, &a.SortTitle,
			&a.Year, &a.ReleaseDate, &a.Description, &a.Genre,
			&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
			&a.TrackCount, &a.ArtistName); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
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

func (r *MusicRepository) UpdateAlbumPosterPath(albumID uuid.UUID, posterPath string) error {
	_, err := r.db.Exec(`UPDATE albums SET poster_path = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		posterPath, albumID)
	return err
}

func (r *MusicRepository) ListRecentlyAddedTracks(libraryID uuid.UUID, limit int) ([]*models.MediaItem, error) {
	query := `
		SELECT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.year, m.duration_seconds, m.audio_codec, m.audio_format,
		       m.bitrate, m.container, m.artist_id, m.album_id,
		       m.track_number, m.disc_number, m.poster_path, m.added_at, m.updated_at,
		       COALESCE(ar.name, '') AS artist_name,
		       COALESCE(al.title, '') AS album_title
		FROM media_items m
		LEFT JOIN artists ar ON ar.id = m.artist_id
		LEFT JOIN albums al ON al.id = m.album_id
		WHERE m.library_id = $1 AND m.media_type = 'music'
		ORDER BY m.added_at DESC
		LIMIT $2`
	return r.queryTracks(query, libraryID, limit)
}

func (r *MusicRepository) ListMostPlayedTracks(libraryID uuid.UUID, limit int) ([]*models.MediaItem, error) {
	query := `
		SELECT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.year, m.duration_seconds, m.audio_codec, m.audio_format,
		       m.bitrate, m.container, m.artist_id, m.album_id,
		       m.track_number, m.disc_number, m.poster_path, m.added_at, m.updated_at,
		       COALESCE(ar.name, '') AS artist_name,
		       COALESCE(al.title, '') AS album_title
		FROM media_items m
		LEFT JOIN artists ar ON ar.id = m.artist_id
		LEFT JOIN albums al ON al.id = m.album_id
		WHERE m.library_id = $1 AND m.media_type = 'music' AND m.play_count > 0
		ORDER BY m.play_count DESC
		LIMIT $2`
	return r.queryTracks(query, libraryID, limit)
}

func (r *MusicRepository) ListRecentlyPlayedTracks(libraryID uuid.UUID, limit int) ([]*models.MediaItem, error) {
	query := `
		SELECT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.year, m.duration_seconds, m.audio_codec, m.audio_format,
		       m.bitrate, m.container, m.artist_id, m.album_id,
		       m.track_number, m.disc_number, m.poster_path, m.added_at, m.updated_at,
		       COALESCE(ar.name, '') AS artist_name,
		       COALESCE(al.title, '') AS album_title
		FROM media_items m
		LEFT JOIN artists ar ON ar.id = m.artist_id
		LEFT JOIN albums al ON al.id = m.album_id
		WHERE m.library_id = $1 AND m.media_type = 'music' AND m.last_played_at IS NOT NULL
		ORDER BY m.last_played_at DESC
		LIMIT $2`
	return r.queryTracks(query, libraryID, limit)
}

func (r *MusicRepository) queryTracks(query string, libraryID uuid.UUID, limit int) ([]*models.MediaItem, error) {
	rows, err := r.db.Query(query, libraryID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.MediaItem
	for rows.Next() {
		m := &models.MediaItem{}
		if err := rows.Scan(
			&m.ID, &m.LibraryID, &m.MediaType, &m.FilePath, &m.FileName, &m.FileSize,
			&m.Title, &m.Year, &m.DurationSeconds, &m.AudioCodec, &m.AudioFormat,
			&m.Bitrate, &m.Container, &m.ArtistID, &m.AlbumID,
			&m.TrackNumber, &m.DiscNumber, &m.PosterPath, &m.AddedAt, &m.UpdatedAt,
			&m.ArtistName, &m.AlbumTitle,
		); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *MusicRepository) SearchArtists(libraryID uuid.UUID, query string, limit int) ([]*models.Artist, error) {
	q := `
		SELECT a.id, a.library_id, a.name, a.sort_name, a.description, a.poster_path, a.mbid,
		       a.sort_position, a.created_at, a.updated_at,
		       COUNT(DISTINCT al.id) AS album_count,
		       COUNT(DISTINCT m.id) AS track_count
		FROM artists a
		LEFT JOIN albums al ON al.artist_id = a.id
		LEFT JOIN media_items m ON m.artist_id = a.id
		WHERE a.library_id = $1 AND a.name ILIKE $2
		GROUP BY a.id
		ORDER BY a.name LIMIT $3`
	rows, err := r.db.Query(q, libraryID, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var artists []*models.Artist
	for rows.Next() {
		a := &models.Artist{}
		if err := rows.Scan(&a.ID, &a.LibraryID, &a.Name, &a.SortName,
			&a.Description, &a.PosterPath, &a.MBID, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
			&a.AlbumCount, &a.TrackCount); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (r *MusicRepository) SearchAlbums(libraryID uuid.UUID, query string, limit int) ([]*models.Album, error) {
	q := `
		SELECT al.id, al.artist_id, al.library_id, al.title, al.sort_title, al.year,
		       al.release_date, al.description, al.genre, al.poster_path,
		       al.sort_position, al.created_at, al.updated_at,
		       COUNT(m.id) AS track_count,
		       COALESCE(ar.name, '') AS artist_name
		FROM albums al
		LEFT JOIN media_items m ON m.album_id = al.id
		LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.library_id = $1 AND al.title ILIKE $2
		GROUP BY al.id, ar.name
		ORDER BY al.title LIMIT $3`
	rows, err := r.db.Query(q, libraryID, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var albums []*models.Album
	for rows.Next() {
		a := &models.Album{}
		if err := rows.Scan(&a.ID, &a.ArtistID, &a.LibraryID, &a.Title, &a.SortTitle,
			&a.Year, &a.ReleaseDate, &a.Description, &a.Genre,
			&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
			&a.TrackCount, &a.ArtistName); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (r *MusicRepository) SearchTracks(libraryID uuid.UUID, query string, limit int) ([]*models.MediaItem, error) {
	q := `
		SELECT m.id, m.library_id, m.media_type, m.file_path, m.file_name, m.file_size,
		       m.title, m.year, m.duration_seconds, m.audio_codec, m.audio_format,
		       m.bitrate, m.container, m.artist_id, m.album_id,
		       m.track_number, m.disc_number, m.poster_path, m.added_at, m.updated_at,
		       COALESCE(ar.name, '') AS artist_name,
		       COALESCE(al.title, '') AS album_title
		FROM media_items m
		LEFT JOIN artists ar ON ar.id = m.artist_id
		LEFT JOIN albums al ON al.id = m.album_id
		WHERE m.library_id = $1 AND m.media_type = 'music' AND m.title ILIKE $2
		ORDER BY m.title LIMIT $3`
	rows, err := r.db.Query(q, libraryID, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*models.MediaItem
	for rows.Next() {
		m := &models.MediaItem{}
		if err := rows.Scan(
			&m.ID, &m.LibraryID, &m.MediaType, &m.FilePath, &m.FileName, &m.FileSize,
			&m.Title, &m.Year, &m.DurationSeconds, &m.AudioCodec, &m.AudioFormat,
			&m.Bitrate, &m.Container, &m.ArtistID, &m.AlbumID,
			&m.TrackNumber, &m.DiscNumber, &m.PosterPath, &m.AddedAt, &m.UpdatedAt,
			&m.ArtistName, &m.AlbumTitle,
		); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// CleanupDuplicateAlbums merges albums that share the same title within a
// library but ended up under different artist records (e.g. "Onyx" vs
// "Onyx feat. Dope D.O.D."). For each group of duplicates the album with the
// most tracks is kept; tracks from the others are reassigned and the empty
// albums (and orphaned artists) are removed.
func (r *MusicRepository) CleanupDuplicateAlbums(libraryID uuid.UUID) (int, error) {
	// Step 1: reassign tracks from duplicate albums to the primary (most-tracked) album.
	reassign := `
		WITH ranked AS (
			SELECT al.id AS album_id,
			       LOWER(al.title) AS ltitle,
			       COUNT(m.id) AS tc,
			       ROW_NUMBER() OVER (
			           PARTITION BY al.library_id, LOWER(al.title)
			           ORDER BY COUNT(m.id) DESC, al.created_at
			       ) AS rn
			FROM albums al
			LEFT JOIN media_items m ON m.album_id = al.id
			WHERE al.library_id = $1
			GROUP BY al.id
		),
		primary_map AS (
			SELECT r2.album_id AS dup_id, p.album_id AS primary_id
			FROM ranked r2
			JOIN ranked p ON p.ltitle = r2.ltitle AND p.rn = 1
			WHERE r2.rn > 1
		)
		UPDATE media_items
		SET album_id = pm.primary_id, artist_id = (SELECT artist_id FROM albums WHERE id = pm.primary_id)
		FROM primary_map pm
		WHERE media_items.album_id = pm.dup_id`

	if _, err := r.db.Exec(reassign, libraryID); err != nil {
		return 0, fmt.Errorf("reassign tracks: %w", err)
	}

	// Step 2: delete albums that no longer have any tracks and are not the
	// primary (rank=1) within their title group.
	del := `
		WITH ranked AS (
			SELECT al.id AS album_id,
			       COUNT(m.id) AS tc,
			       ROW_NUMBER() OVER (
			           PARTITION BY al.library_id, LOWER(al.title)
			           ORDER BY COUNT(m.id) DESC, al.created_at
			       ) AS rn
			FROM albums al
			LEFT JOIN media_items m ON m.album_id = al.id
			WHERE al.library_id = $1
			GROUP BY al.id
		)
		DELETE FROM albums
		WHERE id IN (SELECT album_id FROM ranked WHERE rn > 1 AND tc = 0)`

	result, err := r.db.Exec(del, libraryID)
	if err != nil {
		return 0, fmt.Errorf("delete duplicate albums: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// CleanupOrphanedArtists removes artist records that have no albums and no
// tracks linked to them.
func (r *MusicRepository) CleanupOrphanedArtists(libraryID uuid.UUID) (int, error) {
	query := `
		DELETE FROM artists
		WHERE library_id = $1
		  AND id NOT IN (SELECT DISTINCT artist_id FROM albums WHERE library_id = $1)
		  AND id NOT IN (SELECT DISTINCT artist_id FROM media_items WHERE library_id = $1 AND artist_id IS NOT NULL)`
	result, err := r.db.Exec(query, libraryID)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (r *MusicRepository) ListAlbumsWithoutPosters(libraryID uuid.UUID) ([]*models.Album, error) {
	query := `
		SELECT al.id, al.artist_id, al.library_id, al.title, al.sort_title, al.year,
		       al.release_date, al.description, al.genre, al.poster_path,
		       al.sort_position, al.created_at, al.updated_at,
		       0 AS track_count,
		       COALESCE(ar.name, '') AS artist_name
		FROM albums al
		LEFT JOIN artists ar ON ar.id = al.artist_id
		WHERE al.library_id = $1 AND al.poster_path IS NULL
		ORDER BY COALESCE(al.sort_title, al.title)`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*models.Album
	for rows.Next() {
		a := &models.Album{}
		if err := rows.Scan(&a.ID, &a.ArtistID, &a.LibraryID, &a.Title, &a.SortTitle,
			&a.Year, &a.ReleaseDate, &a.Description, &a.Genre,
			&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
			&a.TrackCount, &a.ArtistName); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}
