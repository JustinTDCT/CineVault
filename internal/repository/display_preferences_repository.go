package repository

import (
	"database/sql"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

const defaultOverlaySettings = `{"resolution_hdr":true,"audio_codec":true,"ratings":true,"content_rating":false,"edition_type":true,"source_type":false}`

type DisplayPreferencesRepository struct {
	db *sql.DB
}

func NewDisplayPreferencesRepository(db *sql.DB) *DisplayPreferencesRepository {
	return &DisplayPreferencesRepository{db: db}
}

// GetByUserID returns the display preferences for a user, creating defaults if none exist.
func (r *DisplayPreferencesRepository) GetByUserID(userID uuid.UUID) (*models.UserDisplayPreferences, error) {
	var pref models.UserDisplayPreferences
	err := r.db.QueryRow(
		`SELECT id, user_id, overlay_settings, region, created_at, updated_at
		 FROM user_display_preferences WHERE user_id = $1`, userID,
	).Scan(&pref.ID, &pref.UserID, &pref.OverlaySettings, &pref.Region, &pref.CreatedAt, &pref.UpdatedAt)

	if err == sql.ErrNoRows {
		pref.ID = uuid.New()
		pref.UserID = userID
		pref.OverlaySettings = defaultOverlaySettings
		pref.Region = ""
		_, insertErr := r.db.Exec(
			`INSERT INTO user_display_preferences (id, user_id, overlay_settings, region)
			 VALUES ($1, $2, $3, $4)`,
			pref.ID, pref.UserID, pref.OverlaySettings, pref.Region,
		)
		if insertErr != nil {
			return nil, insertErr
		}
		_ = r.db.QueryRow(
			`SELECT id, user_id, overlay_settings, region, created_at, updated_at
			 FROM user_display_preferences WHERE user_id = $1`, userID,
		).Scan(&pref.ID, &pref.UserID, &pref.OverlaySettings, &pref.Region, &pref.CreatedAt, &pref.UpdatedAt)
		return &pref, nil
	}
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

// Upsert creates or updates display preferences for a user.
func (r *DisplayPreferencesRepository) Upsert(userID uuid.UUID, overlaySettings string) error {
	_, err := r.db.Exec(
		`INSERT INTO user_display_preferences (id, user_id, overlay_settings)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		 SET overlay_settings = $3, updated_at = CURRENT_TIMESTAMP`,
		uuid.New(), userID, overlaySettings,
	)
	return err
}

// UpdateRegion sets the preferred region for a user.
func (r *DisplayPreferencesRepository) UpdateRegion(userID uuid.UUID, region string) error {
	_, err := r.db.Exec(
		`INSERT INTO user_display_preferences (id, user_id, overlay_settings, region)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id) DO UPDATE
		 SET region = $4, updated_at = CURRENT_TIMESTAMP`,
		uuid.New(), userID, defaultOverlaySettings, region,
	)
	return err
}
