package repository

import (
	"database/sql"
	"fmt"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// userColumns is the standard SELECT list for users.
const userColumns = `id, username, email, password_hash, pin_hash, display_name, first_name, last_name,
	role, is_active, max_content_rating, is_kids_profile, avatar_id, parent_user_id, created_at, updated_at`

func scanUser(row interface{ Scan(dest ...interface{}) error }) (*models.User, error) {
	user := &models.User{}
	err := row.Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.PinHash, &user.DisplayName, &user.FirstName, &user.LastName,
		&user.Role, &user.IsActive, &user.MaxContentRating, &user.IsKidsProfile,
		&user.AvatarID, &user.ParentUserID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == nil {
		user.HasPin = user.PinHash != nil && *user.PinHash != ""
		user.IsMaster = user.ParentUserID == nil
	}
	return user, err
}

func (r *UserRepository) Create(user *models.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash, pin_hash, display_name, first_name, last_name,
		                    role, is_active, max_content_rating, is_kids_profile, avatar_id, parent_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at, updated_at`

	return r.db.QueryRow(query, user.ID, user.Username, user.Email,
		user.PasswordHash, user.PinHash, user.DisplayName, user.FirstName, user.LastName,
		user.Role, user.IsActive, user.MaxContentRating, user.IsKidsProfile, user.AvatarID,
		user.ParentUserID).
		Scan(&user.CreatedAt, &user.UpdatedAt)
}

func (r *UserRepository) GetByID(id uuid.UUID) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	user, err := scanUser(r.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE username = $1`
	user, err := scanUser(r.db.QueryRow(query, username))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE email = $1`
	user, err := scanUser(r.db.QueryRow(query, email))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	return user, err
}

func (r *UserRepository) List() ([]*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users ORDER BY created_at DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// ListMasterUsers returns only active master users (parent_user_id IS NULL) for fast login.
func (r *UserRepository) ListMasterUsers() ([]*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE parent_user_id IS NULL AND is_active = true ORDER BY created_at DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// ListHousehold returns the master user + all their sub-profiles.
func (r *UserRepository) ListHousehold(masterID uuid.UUID) ([]*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE (id = $1 OR parent_user_id = $1) AND is_active = true ORDER BY parent_user_id NULLS FIRST, created_at ASC`
	rows, err := r.db.Query(query, masterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// CountByParent returns the number of sub-profiles for a master user.
func (r *UserRepository) CountByParent(masterID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE parent_user_id = $1`, masterID).Scan(&count)
	return count, err
}

func (r *UserRepository) Update(user *models.User) error {
	query := `
		UPDATE users 
		SET username = $1, email = $2, role = $3, is_active = $4, display_name = $5,
		    first_name = $6, last_name = $7, max_content_rating = $8, is_kids_profile = $9,
		    avatar_id = $10, updated_at = CURRENT_TIMESTAMP
		WHERE id = $11`

	result, err := r.db.Exec(query, user.Username, user.Email, user.Role, user.IsActive,
		user.DisplayName, user.FirstName, user.LastName, user.MaxContentRating,
		user.IsKidsProfile, user.AvatarID, user.ID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateProfile(id uuid.UUID, firstName, lastName, email *string) error {
	query := `
		UPDATE users 
		SET first_name = COALESCE($1, first_name), last_name = COALESCE($2, last_name), 
		    email = COALESCE($3, email), updated_at = CURRENT_TIMESTAMP
		WHERE id = $4`

	result, err := r.db.Exec(query, firstName, lastName, email, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateProfileSettings(id uuid.UUID, maxContentRating *string, isKidsProfile *bool, avatarID *string) error {
	query := `
		UPDATE users 
		SET max_content_rating = $1, 
		    is_kids_profile = COALESCE($2, is_kids_profile),
		    avatar_id = COALESCE($3, avatar_id),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $4`

	result, err := r.db.Exec(query, maxContentRating, isKidsProfile, avatarID, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// UpdateSubProfile updates display_name, avatar, kids mode, and content rating for a sub-profile.
func (r *UserRepository) UpdateSubProfile(id uuid.UUID, displayName *string, avatarID *string, isKidsProfile *bool, maxContentRating *string) error {
	query := `
		UPDATE users 
		SET display_name = COALESCE($1, display_name),
		    avatar_id = COALESCE($2, avatar_id),
		    is_kids_profile = COALESCE($3, is_kids_profile),
		    max_content_rating = $4,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $5`

	result, err := r.db.Exec(query, displayName, avatarID, isKidsProfile, maxContentRating, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdatePassword(id uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	result, err := r.db.Exec(query, passwordHash, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdatePinHash(id uuid.UUID, pinHash *string) error {
	query := `UPDATE users SET pin_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	result, err := r.db.Exec(query, pinHash, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) Count() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (r *UserRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}
