package libraries

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(lib *Library) error {
	return r.db.QueryRow(`
		INSERT INTO libraries (name, library_type, folders, include_in_homepage, include_in_search,
		       retrieve_metadata, import_nfo, export_nfo, normalize_audio, timeline_scrubbing,
		       preview_videos, intro_detection, credits_detection, recap_detection)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at, updated_at`,
		lib.Name, lib.LibraryType, pq.Array(lib.Folders),
		lib.IncludeInHomepage, lib.IncludeInSearch, lib.RetrieveMetadata,
		lib.ImportNFO, lib.ExportNFO, lib.NormalizeAudio, lib.TimelineScrubbing,
		lib.PreviewVideos, lib.IntroDetection, lib.CreditsDetection, lib.RecapDetection,
	).Scan(&lib.ID, &lib.CreatedAt, &lib.UpdatedAt)
}

func (r *Repository) GetByID(id string) (*Library, error) {
	lib := &Library{}
	err := r.db.QueryRow(`
		SELECT id, name, library_type, folders, include_in_homepage, include_in_search,
		       retrieve_metadata, import_nfo, export_nfo, normalize_audio, timeline_scrubbing,
		       preview_videos, intro_detection, credits_detection, recap_detection,
		       created_at, updated_at
		FROM libraries WHERE id=$1`, id,
	).Scan(&lib.ID, &lib.Name, &lib.LibraryType, pq.Array(&lib.Folders),
		&lib.IncludeInHomepage, &lib.IncludeInSearch, &lib.RetrieveMetadata,
		&lib.ImportNFO, &lib.ExportNFO, &lib.NormalizeAudio, &lib.TimelineScrubbing,
		&lib.PreviewVideos, &lib.IntroDetection, &lib.CreditsDetection, &lib.RecapDetection,
		&lib.CreatedAt, &lib.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("library not found: %w", err)
	}
	return lib, nil
}

func (r *Repository) List() ([]Library, error) {
	rows, err := r.db.Query(`
		SELECT id, name, library_type, folders, include_in_homepage, include_in_search,
		       retrieve_metadata, import_nfo, export_nfo, normalize_audio, timeline_scrubbing,
		       preview_videos, intro_detection, credits_detection, recap_detection,
		       created_at, updated_at
		FROM libraries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Library
	for rows.Next() {
		var lib Library
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.LibraryType, pq.Array(&lib.Folders),
			&lib.IncludeInHomepage, &lib.IncludeInSearch, &lib.RetrieveMetadata,
			&lib.ImportNFO, &lib.ExportNFO, &lib.NormalizeAudio, &lib.TimelineScrubbing,
			&lib.PreviewVideos, &lib.IntroDetection, &lib.CreditsDetection, &lib.RecapDetection,
			&lib.CreatedAt, &lib.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, lib)
	}
	return out, nil
}

func (r *Repository) ListForUser(userID string) ([]Library, error) {
	rows, err := r.db.Query(`
		SELECT l.id, l.name, l.library_type, l.folders, l.include_in_homepage, l.include_in_search,
		       l.retrieve_metadata, l.import_nfo, l.export_nfo, l.normalize_audio, l.timeline_scrubbing,
		       l.preview_videos, l.intro_detection, l.credits_detection, l.recap_detection,
		       l.created_at, l.updated_at
		FROM libraries l
		LEFT JOIN library_permissions lp ON l.id = lp.library_id AND lp.user_id = $1
		WHERE lp.permission_level IN ('view', 'edit') OR lp.id IS NULL
		ORDER BY l.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Library
	for rows.Next() {
		var lib Library
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.LibraryType, pq.Array(&lib.Folders),
			&lib.IncludeInHomepage, &lib.IncludeInSearch, &lib.RetrieveMetadata,
			&lib.ImportNFO, &lib.ExportNFO, &lib.NormalizeAudio, &lib.TimelineScrubbing,
			&lib.PreviewVideos, &lib.IntroDetection, &lib.CreditsDetection, &lib.RecapDetection,
			&lib.CreatedAt, &lib.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, lib)
	}
	return out, nil
}

func (r *Repository) Update(lib *Library) error {
	_, err := r.db.Exec(`
		UPDATE libraries SET name=$2, folders=$3, include_in_homepage=$4, include_in_search=$5,
		       retrieve_metadata=$6, import_nfo=$7, export_nfo=$8, normalize_audio=$9,
		       timeline_scrubbing=$10, preview_videos=$11, intro_detection=$12,
		       credits_detection=$13, recap_detection=$14, updated_at=NOW()
		WHERE id=$1`,
		lib.ID, lib.Name, pq.Array(lib.Folders),
		lib.IncludeInHomepage, lib.IncludeInSearch, lib.RetrieveMetadata,
		lib.ImportNFO, lib.ExportNFO, lib.NormalizeAudio, lib.TimelineScrubbing,
		lib.PreviewVideos, lib.IntroDetection, lib.CreditsDetection, lib.RecapDetection)
	return err
}

func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM libraries WHERE id=$1", id)
	return err
}

func (r *Repository) SetPermission(libraryID, userID string, level PermissionLevel) error {
	_, err := r.db.Exec(`
		INSERT INTO library_permissions (library_id, user_id, permission_level)
		VALUES ($1, $2, $3)
		ON CONFLICT (library_id, user_id) DO UPDATE SET permission_level=$3`,
		libraryID, userID, level)
	return err
}

func (r *Repository) GetPermissions(libraryID string) ([]LibraryPermission, error) {
	rows, err := r.db.Query(`
		SELECT id, library_id, user_id, permission_level
		FROM library_permissions WHERE library_id=$1`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LibraryPermission
	for rows.Next() {
		var p LibraryPermission
		if err := rows.Scan(&p.ID, &p.LibraryID, &p.UserID, &p.PermissionLevel); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *Repository) GetUserPermission(libraryID, userID string) (PermissionLevel, error) {
	var level PermissionLevel
	err := r.db.QueryRow(`
		SELECT permission_level FROM library_permissions
		WHERE library_id=$1 AND user_id=$2`, libraryID, userID).Scan(&level)
	if err == sql.ErrNoRows {
		return PermissionView, nil
	}
	return level, err
}
