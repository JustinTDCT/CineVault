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
		INSERT INTO libraries (id, name, media_type, path, is_enabled, scan_on_startup)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`
	
	return r.db.QueryRow(query, library.ID, library.Name, library.MediaType,
		library.Path, library.IsEnabled, library.ScanOnStartup).
		Scan(&library.CreatedAt, &library.UpdatedAt)
}

func (r *LibraryRepository) GetByID(id uuid.UUID) (*models.Library, error) {
	library := &models.Library{}
	query := `
		SELECT id, name, media_type, path, is_enabled, scan_on_startup, 
		       last_scan_at, created_at, updated_at
		FROM libraries WHERE id = $1`
	
	err := r.db.QueryRow(query, id).Scan(
		&library.ID, &library.Name, &library.MediaType, &library.Path,
		&library.IsEnabled, &library.ScanOnStartup, &library.LastScanAt,
		&library.CreatedAt, &library.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("library not found")
	}
	return library, err
}

func (r *LibraryRepository) List() ([]*models.Library, error) {
	query := `
		SELECT id, name, media_type, path, is_enabled, scan_on_startup,
		       last_scan_at, created_at, updated_at
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
			&library.IsEnabled, &library.ScanOnStartup, &library.LastScanAt,
			&library.CreatedAt, &library.UpdatedAt); err != nil {
			return nil, err
		}
		libraries = append(libraries, library)
	}
	return libraries, rows.Err()
}

func (r *LibraryRepository) ListByType(mediaType models.MediaType) ([]*models.Library, error) {
	query := `
		SELECT id, name, media_type, path, is_enabled, scan_on_startup,
		       last_scan_at, created_at, updated_at
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
			&library.IsEnabled, &library.ScanOnStartup, &library.LastScanAt,
			&library.CreatedAt, &library.UpdatedAt); err != nil {
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
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $5`
	
	result, err := r.db.Exec(query, library.Name, library.Path, 
		library.IsEnabled, library.ScanOnStartup, library.ID)
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
