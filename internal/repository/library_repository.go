package repository

import (
	"database/sql"
	"fmt"
	"time"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type LibraryRepository struct {
	db *sql.DB
}

func NewLibraryRepository(db *sql.DB) *LibraryRepository {
	return &LibraryRepository{db: db}
}

const libraryColumns = `id, name, media_type, path, is_enabled, scan_on_startup,
	season_grouping, access_level, include_in_homepage, include_in_search,
	retrieve_metadata, nfo_import, nfo_export, prefer_local_artwork,
	create_previews, create_thumbnails, audio_normalization,
	adult_content_type, scan_interval, next_scan_at, watch_enabled,
	last_scan_at, created_at, updated_at`

func scanLibrary(row interface{ Scan(dest ...interface{}) error }) (*models.Library, error) {
	lib := &models.Library{}
	err := row.Scan(
		&lib.ID, &lib.Name, &lib.MediaType, &lib.Path,
		&lib.IsEnabled, &lib.ScanOnStartup,
		&lib.SeasonGrouping, &lib.AccessLevel,
		&lib.IncludeInHomepage, &lib.IncludeInSearch,
		&lib.RetrieveMetadata, &lib.NFOImport, &lib.NFOExport, &lib.PreferLocalArtwork,
		&lib.CreatePreviews, &lib.CreateThumbnails, &lib.AudioNormalization,
		&lib.AdultContentType, &lib.ScanInterval, &lib.NextScanAt, &lib.WatchEnabled,
		&lib.LastScanAt, &lib.CreatedAt, &lib.UpdatedAt,
	)
	return lib, err
}

func (r *LibraryRepository) Create(library *models.Library) error {
	query := `
		INSERT INTO libraries (id, name, media_type, path, is_enabled, scan_on_startup,
			season_grouping, access_level, include_in_homepage, include_in_search,
			retrieve_metadata, nfo_import, nfo_export, prefer_local_artwork,
			create_previews, create_thumbnails, audio_normalization,
			adult_content_type, scan_interval, next_scan_at, watch_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING created_at, updated_at`

	return r.db.QueryRow(query, library.ID, library.Name, library.MediaType,
		library.Path, library.IsEnabled, library.ScanOnStartup,
		library.SeasonGrouping, library.AccessLevel,
		library.IncludeInHomepage, library.IncludeInSearch,
		library.RetrieveMetadata, library.NFOImport, library.NFOExport, library.PreferLocalArtwork,
		library.CreatePreviews, library.CreateThumbnails, library.AudioNormalization,
		library.AdultContentType, library.ScanInterval, library.NextScanAt, library.WatchEnabled).
		Scan(&library.CreatedAt, &library.UpdatedAt)
}

func (r *LibraryRepository) GetByID(id uuid.UUID) (*models.Library, error) {
	query := `SELECT ` + libraryColumns + ` FROM libraries WHERE id = $1`
	lib, err := scanLibrary(r.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("library not found")
	}
	if err != nil {
		return nil, err
	}

	// Load folders
	lib.Folders, _ = r.GetFolders(id)
	return lib, nil
}

func (r *LibraryRepository) List() ([]*models.Library, error) {
	query := `SELECT ` + libraryColumns + ` FROM libraries ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	libraries := []*models.Library{}
	for rows.Next() {
		lib, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		lib.Folders, _ = r.GetFolders(lib.ID)
		libraries = append(libraries, lib)
	}
	return libraries, rows.Err()
}

// ListForUser returns libraries visible to a specific user based on access_level.
// Admins see all libraries. Regular users see 'everyone' + libraries they have explicit permission for.
func (r *LibraryRepository) ListForUser(userID uuid.UUID, role models.UserRole) ([]*models.Library, error) {
	if role == models.RoleAdmin {
		return r.List()
	}

	query := `
		SELECT DISTINCT l.` + libraryColumns + `
		FROM libraries l
		LEFT JOIN library_permissions lp ON l.id = lp.library_id AND lp.user_id = $1
		WHERE l.access_level = 'everyone'
		   OR (l.access_level = 'select_users' AND lp.user_id IS NOT NULL)
		ORDER BY l.created_at DESC`

	// The column references need table alias, rebuild with alias
	aliasedColumns := `l.id, l.name, l.media_type, l.path, l.is_enabled, l.scan_on_startup,
		l.season_grouping, l.access_level, l.include_in_homepage, l.include_in_search,
		l.retrieve_metadata, l.nfo_import, l.nfo_export, l.prefer_local_artwork,
		l.create_previews, l.create_thumbnails, l.audio_normalization,
		l.adult_content_type, l.scan_interval, l.next_scan_at, l.watch_enabled,
		l.last_scan_at, l.created_at, l.updated_at`

	query = `
		SELECT DISTINCT ` + aliasedColumns + `
		FROM libraries l
		LEFT JOIN library_permissions lp ON l.id = lp.library_id AND lp.user_id = $1
		WHERE l.access_level = 'everyone'
		   OR (l.access_level = 'select_users' AND lp.user_id IS NOT NULL)
		ORDER BY l.created_at DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	libraries := []*models.Library{}
	for rows.Next() {
		lib, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		lib.Folders, _ = r.GetFolders(lib.ID)
		libraries = append(libraries, lib)
	}
	return libraries, rows.Err()
}

func (r *LibraryRepository) ListByType(mediaType models.MediaType) ([]*models.Library, error) {
	query := `SELECT ` + libraryColumns + ` FROM libraries WHERE media_type = $1 ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query, mediaType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	libraries := []*models.Library{}
	for rows.Next() {
		lib, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		lib.Folders, _ = r.GetFolders(lib.ID)
		libraries = append(libraries, lib)
	}
	return libraries, rows.Err()
}

// ListHomepageLibraries returns only libraries with include_in_homepage = true, filtered by user access.
func (r *LibraryRepository) ListHomepageLibraries(userID uuid.UUID, role models.UserRole) ([]*models.Library, error) {
	if role == models.RoleAdmin {
		query := `SELECT ` + libraryColumns + ` FROM libraries WHERE include_in_homepage = true ORDER BY created_at DESC`
		rows, err := r.db.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		libraries := []*models.Library{}
		for rows.Next() {
			lib, err := scanLibrary(rows)
			if err != nil {
				return nil, err
			}
			libraries = append(libraries, lib)
		}
		return libraries, rows.Err()
	}

	aliasedColumns := `l.id, l.name, l.media_type, l.path, l.is_enabled, l.scan_on_startup,
		l.season_grouping, l.access_level, l.include_in_homepage, l.include_in_search,
		l.retrieve_metadata, l.nfo_import, l.nfo_export, l.prefer_local_artwork,
		l.create_previews, l.create_thumbnails, l.audio_normalization,
		l.adult_content_type, l.scan_interval, l.next_scan_at, l.watch_enabled,
		l.last_scan_at, l.created_at, l.updated_at`

	query := `
		SELECT DISTINCT ` + aliasedColumns + `
		FROM libraries l
		LEFT JOIN library_permissions lp ON l.id = lp.library_id AND lp.user_id = $1
		WHERE l.include_in_homepage = true
		  AND (l.access_level = 'everyone'
		       OR (l.access_level = 'select_users' AND lp.user_id IS NOT NULL))
		ORDER BY l.created_at DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	libraries := []*models.Library{}
	for rows.Next() {
		lib, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, lib)
	}
	return libraries, rows.Err()
}

// ListSearchableLibraryIDs returns IDs of libraries where include_in_search = true,
// filtered by user access.
func (r *LibraryRepository) ListSearchableLibraryIDs(userID uuid.UUID, role models.UserRole) ([]uuid.UUID, error) {
	var query string
	var rows *sql.Rows
	var err error

	if role == models.RoleAdmin {
		query = `SELECT id FROM libraries WHERE include_in_search = true`
		rows, err = r.db.Query(query)
	} else {
		query = `
			SELECT DISTINCT l.id
			FROM libraries l
			LEFT JOIN library_permissions lp ON l.id = lp.library_id AND lp.user_id = $1
			WHERE l.include_in_search = true
			  AND (l.access_level = 'everyone'
			       OR (l.access_level = 'select_users' AND lp.user_id IS NOT NULL))
		`
		rows, err = r.db.Query(query, userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *LibraryRepository) Update(library *models.Library) error {
	query := `
		UPDATE libraries 
		SET name = $1, path = $2, is_enabled = $3, scan_on_startup = $4,
		    season_grouping = $5, access_level = $6,
		    include_in_homepage = $7, include_in_search = $8,
		    retrieve_metadata = $9, nfo_import = $10, nfo_export = $11, prefer_local_artwork = $12,
		    create_previews = $13, create_thumbnails = $14, audio_normalization = $15,
		    adult_content_type = $16, scan_interval = $17, next_scan_at = $18, watch_enabled = $19,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $20`

	result, err := r.db.Exec(query, library.Name, library.Path,
		library.IsEnabled, library.ScanOnStartup,
		library.SeasonGrouping, library.AccessLevel,
		library.IncludeInHomepage, library.IncludeInSearch,
		library.RetrieveMetadata, library.NFOImport, library.NFOExport, library.PreferLocalArtwork,
		library.CreatePreviews, library.CreateThumbnails, library.AudioNormalization,
		library.AdultContentType, library.ScanInterval, library.NextScanAt, library.WatchEnabled,
		library.ID)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("library not found")
	}
	return nil
}

func (r *LibraryRepository) UpdateLastScan(id uuid.UUID) error {
	query := `UPDATE libraries SET last_scan_at = $1 WHERE id = $2`
	_, err := r.db.Exec(query, time.Now(), id)
	return err
}

func (r *LibraryRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM libraries WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("library not found")
	}
	return nil
}

// ──── Library Permissions ────

func (r *LibraryRepository) SetPermissions(libraryID uuid.UUID, userIDs []uuid.UUID) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing permissions
	if _, err := tx.Exec(`DELETE FROM library_permissions WHERE library_id = $1`, libraryID); err != nil {
		return err
	}

	// Insert new permissions
	for _, uid := range userIDs {
		if _, err := tx.Exec(
			`INSERT INTO library_permissions (id, library_id, user_id) VALUES ($1, $2, $3)`,
			uuid.New(), libraryID, uid,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *LibraryRepository) GetPermissions(libraryID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT user_id FROM library_permissions WHERE library_id = $1`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, uid)
	}
	return userIDs, rows.Err()
}

// ──── Library Folders ────

func (r *LibraryRepository) GetFolders(libraryID uuid.UUID) ([]models.LibraryFolder, error) {
	query := `SELECT id, library_id, folder_path, sort_position, created_at
		FROM library_folders WHERE library_id = $1 ORDER BY sort_position`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []models.LibraryFolder
	for rows.Next() {
		var f models.LibraryFolder
		if err := rows.Scan(&f.ID, &f.LibraryID, &f.FolderPath, &f.SortPosition, &f.CreatedAt); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

// GetDueForScan returns enabled libraries whose next_scan_at is in the past.
func (r *LibraryRepository) GetDueForScan() ([]*models.Library, error) {
	query := `SELECT ` + libraryColumns + ` FROM libraries
		WHERE is_enabled = true AND scan_interval != 'disabled'
		AND next_scan_at IS NOT NULL AND next_scan_at <= NOW()
		ORDER BY next_scan_at ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libs []*models.Library
	for rows.Next() {
		lib, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		libs = append(libs, lib)
	}
	return libs, rows.Err()
}

// GetWatchEnabled returns all enabled libraries with watch_enabled = true.
func (r *LibraryRepository) GetWatchEnabled() ([]*models.Library, error) {
	query := `SELECT ` + libraryColumns + ` FROM libraries
		WHERE is_enabled = true AND watch_enabled = true`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libs []*models.Library
	for rows.Next() {
		lib, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		lib.Folders, _ = r.GetFolders(lib.ID)
		libs = append(libs, lib)
	}
	return libs, rows.Err()
}

// AdvanceNextScan sets next_scan_at based on the scan_interval.
func (r *LibraryRepository) AdvanceNextScan(id uuid.UUID) error {
	query := `UPDATE libraries SET next_scan_at = NOW() + CASE scan_interval
		WHEN '1h' THEN INTERVAL '1 hour'
		WHEN '6h' THEN INTERVAL '6 hours'
		WHEN '12h' THEN INTERVAL '12 hours'
		WHEN '24h' THEN INTERVAL '24 hours'
		WHEN 'weekly' THEN INTERVAL '7 days'
		ELSE NULL END
		WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *LibraryRepository) SetFolders(libraryID uuid.UUID, paths []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing folders
	if _, err := tx.Exec(`DELETE FROM library_folders WHERE library_id = $1`, libraryID); err != nil {
		return err
	}

	// Insert new folders
	for i, p := range paths {
		if p == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO library_folders (id, library_id, folder_path, sort_position) VALUES ($1, $2, $3, $4)`,
			uuid.New(), libraryID, p, i,
		); err != nil {
			return err
		}
	}

	// Update the library's primary path to the first folder
	if len(paths) > 0 {
		if _, err := tx.Exec(`UPDATE libraries SET path = $1 WHERE id = $2`, paths[0], libraryID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
