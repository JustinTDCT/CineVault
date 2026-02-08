package repository

import (
	"database/sql"
)

type SettingsRepository struct {
	db *sql.DB
}

func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// Get retrieves a system setting value by key. Returns empty string if not found.
func (r *SettingsRepository) Get(key string) (string, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM system_settings WHERE key = $1`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// Set upserts a system setting key-value pair.
func (r *SettingsRepository) Set(key, value string) error {
	query := `INSERT INTO system_settings (key, value, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP`
	_, err := r.db.Exec(query, key, value)
	return err
}

// GetAll returns all system settings as a map.
func (r *SettingsRepository) GetAll() (map[string]string, error) {
	rows, err := r.db.Query(`SELECT key, value FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}

// Delete removes a system setting by key.
func (r *SettingsRepository) Delete(key string) error {
	_, err := r.db.Exec(`DELETE FROM system_settings WHERE key = $1`, key)
	return err
}
