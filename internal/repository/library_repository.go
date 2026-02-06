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

func (r *LibraryRepository) Create(library *models.Library) error {
	query := `
		INSERT INTO libraries (id, name, media_type, path, is_enabled, scan_on_startup, season_grouping, access_level)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`
	
	return r.db.QueryRow(query, library.ID, library.Name, library.MediaType,
		library.Path, library.IsEnabled, library.ScanOnStartup,
		library.SeasonGrouping, library.AccessLevel).
		Scan(&library.CreatedAt, &library.UpdatedAt)
}

func (r *LibraryRepository) GetByID(id uuid.UUID) (*models.Library, error) {
	library := &models.Library{}
	query := `
		SELECT id, name, media_type, path, is_enabled, scan_on_startup,
		       season_grouping, access_level, last_scan_at, created_at, updated_at
		FROM libraries WHERE id = $1`
	
	err := r.db.QueryRow(query, id).Scan(
		&library.ID, &library.Name, &library.MediaType, &library.Path,
		&library.IsEnabled, &library.ScanOnStartup,
		&library.SeasonGrouping, &library.AccessLevel,
		&library.LastScanAt, &library.CreatedAt, &library.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("library not found")
	}
	return library, err
}

func (r *LibraryRepository) List() ([]*models.Library, error) {
	query := `
		SELECT id, name, media_type, path, is_enabled, scan_on_startup,
		       season_grouping, access_level, last_scan_at, created_at, updated_at
		FROM libraries ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	libraries := []*models.Library{}
	for rows.Next() {
		library := &models.Library{}
		if err := rows.Scan(&library.ID, &library.Name, &library.MediaType, &library.Path,
			&library.IsEnabled, &library.ScanOnStartup,
			&library.SeasonGrouping, &library.AccessLevel,
			&library.LastScanAt, &library.CreatedAt, &library.UpdatedAt); err != nil {
			return nil, err
		}
		libraries = append(libraries, library)
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
		SELECT DISTINCT l.id, l.name, l.media_type, l.path, l.is_enabled, l.scan_on_startup,
		       l.season_grouping, l.access_level, l.last_scan_at, l.created_at, l.updated_at
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
		library := &models.Library{}
		if err := rows.Scan(&library.ID, &library.Name, &library.MediaType, &library.Path,
			&library.IsEnabled, &library.ScanOnStartup,
			&library.SeasonGrouping, &library.AccessLevel,
			&library.LastScanAt, &library.CreatedAt, &library.UpdatedAt); err != nil {
			return nil, err
		}
		libraries = append(libraries, library)
	}
	return libraries, rows.Err()
}

func (r *LibraryRepository) ListByType(mediaType models.MediaType) ([]*models.Library, error) {
	query := `
		SELECT id, name, media_type, path, is_enabled, scan_on_startup,
		       season_grouping, access_level, last_scan_at, created_at, updated_at
		FROM libraries WHERE media_type = $1 ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query, mediaType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	libraries := []*models.Library{}
	for rows.Next() {
		library := &models.Library{}
		if err := rows.Scan(&library.ID, &library.Name, &library.MediaType, &library.Path,
			&library.IsEnabled, &library.ScanOnStartup,
			&library.SeasonGrouping, &library.AccessLevel,
			&library.LastScanAt, &library.CreatedAt, &library.UpdatedAt); err != nil {
			return nil, err
		}
		libraries = append(libraries, library)
	}
	return libraries, rows.Err()
}

func (r *LibraryRepository) Update(library *models.Library) error {
	query := `
		UPDATE libraries 
		SET name = $1, path = $2, is_enabled = $3, scan_on_startup = $4,
		    season_grouping = $5, access_level = $6, updated_at = CURRENT_TIMESTAMP
		WHERE id = $7`
	
	result, err := r.db.Exec(query, library.Name, library.Path, 
		library.IsEnabled, library.ScanOnStartup,
		library.SeasonGrouping, library.AccessLevel, library.ID)
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
