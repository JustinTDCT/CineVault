package settings

import "database/sql"

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Get(key string) (string, error) {
	var val string
	err := r.db.QueryRow("SELECT value FROM settings WHERE key=$1", key).Scan(&val)
	return val, err
}

func (r *Repository) Set(key, value string) error {
	_, err := r.db.Exec(`
		INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value=$2, updated_at=NOW()`,
		key, value)
	return err
}

func (r *Repository) GetAll() ([]Setting, error) {
	rows, err := r.db.Query("SELECT key, value, updated_at FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Setting
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *Repository) Delete(key string) error {
	_, err := r.db.Exec("DELETE FROM settings WHERE key=$1", key)
	return err
}
