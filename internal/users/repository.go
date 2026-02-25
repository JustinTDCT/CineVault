package users

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(u *User) error {
	return r.db.QueryRow(`
		INSERT INTO users (account_type, parent_id, full_name, email, password_hash, pin, is_child, child_restrictions, avatar_path, is_admin)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`,
		u.AccountType, u.ParentID, u.FullName, u.Email, u.PasswordHash,
		u.PIN, u.IsChild, u.ChildRestrictions, u.AvatarPath, u.IsAdmin,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *Repository) GetByID(id string) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(`
		SELECT id, account_type, parent_id, full_name, email, password_hash, pin,
		       is_child, child_restrictions, avatar_path, is_admin, created_at, updated_at
		FROM users WHERE id=$1`, id,
	).Scan(&u.ID, &u.AccountType, &u.ParentID, &u.FullName, &u.Email, &u.PasswordHash,
		&u.PIN, &u.IsChild, &u.ChildRestrictions, &u.AvatarPath, &u.IsAdmin,
		&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

func (r *Repository) GetByEmail(email string) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(`
		SELECT id, account_type, parent_id, full_name, email, password_hash, pin,
		       is_child, child_restrictions, avatar_path, is_admin, created_at, updated_at
		FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.AccountType, &u.ParentID, &u.FullName, &u.Email, &u.PasswordHash,
		&u.PIN, &u.IsChild, &u.ChildRestrictions, &u.AvatarPath, &u.IsAdmin,
		&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

func (r *Repository) List() ([]User, error) {
	rows, err := r.db.Query(`
		SELECT id, account_type, parent_id, full_name, email, is_child, avatar_path, is_admin, created_at, updated_at
		FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.AccountType, &u.ParentID, &u.FullName, &u.Email,
			&u.IsChild, &u.AvatarPath, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

func (r *Repository) Update(u *User) error {
	_, err := r.db.Exec(`
		UPDATE users SET full_name=$2, email=$3, pin=$4, is_child=$5,
		       child_restrictions=$6, avatar_path=$7, updated_at=NOW()
		WHERE id=$1`,
		u.ID, u.FullName, u.Email, u.PIN, u.IsChild,
		u.ChildRestrictions, u.AvatarPath)
	return err
}

func (r *Repository) UpdatePassword(id, hash string) error {
	_, err := r.db.Exec("UPDATE users SET password_hash=$2, updated_at=NOW() WHERE id=$1", id, hash)
	return err
}

func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM users WHERE id=$1", id)
	return err
}

func (r *Repository) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (r *Repository) GetProfile(userID string) (*UserProfile, error) {
	p := &UserProfile{}
	err := r.db.QueryRow(`
		SELECT id, user_id, default_video_quality, auto_play_music, auto_play_videos,
		       auto_play_music_videos, auto_play_audiobooks, overlay_settings, library_order,
		       created_at, updated_at
		FROM user_profiles WHERE user_id=$1`, userID,
	).Scan(&p.ID, &p.UserID, &p.DefaultVideoQuality, &p.AutoPlayMusic, &p.AutoPlayVideos,
		&p.AutoPlayMusicVideos, &p.AutoPlayAudiobooks, &p.OverlaySettings,
		pq.Array(&p.LibraryOrder), &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Repository) CreateDefaultProfile(userID string) error {
	defaults, _ := json.Marshal(map[string]interface{}{
		"resolution_audio": map[string]interface{}{"enabled": true, "position": "top_left"},
		"edition":          map[string]interface{}{"enabled": false, "position": "top_right"},
		"ratings":          map[string]interface{}{"enabled": true, "position": "bottom_left"},
		"content_rating":   map[string]interface{}{"enabled": false, "position": "bottom_right"},
		"source_type":      map[string]interface{}{"enabled": false, "position": "top"},
		"hide_theatrical":  false,
	})
	_, err := r.db.Exec(`
		INSERT INTO user_profiles (user_id, overlay_settings) VALUES ($1, $2)`,
		userID, defaults)
	return err
}

func (r *Repository) UpdateProfile(p *UserProfile) error {
	_, err := r.db.Exec(`
		UPDATE user_profiles SET default_video_quality=$2, auto_play_music=$3, auto_play_videos=$4,
		       auto_play_music_videos=$5, auto_play_audiobooks=$6, overlay_settings=$7,
		       library_order=$8, updated_at=NOW()
		WHERE user_id=$1`,
		p.UserID, p.DefaultVideoQuality, p.AutoPlayMusic, p.AutoPlayVideos,
		p.AutoPlayMusicVideos, p.AutoPlayAudiobooks, p.OverlaySettings,
		pq.Array(p.LibraryOrder))
	return err
}

func (r *Repository) ListForPINSwitch() ([]User, error) {
	rows, err := r.db.Query(`
		SELECT id, full_name, avatar_path FROM users WHERE pin IS NOT NULL ORDER BY full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.FullName, &u.AvatarPath); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}
