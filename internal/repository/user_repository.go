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

func (r *UserRepository) Create(user *models.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash, pin_hash, display_name, first_name, last_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`
	
	return r.db.QueryRow(query, user.ID, user.Username, user.Email,
		user.PasswordHash, user.PinHash, user.DisplayName, user.FirstName, user.LastName, user.Role, user.IsActive).
		Scan(&user.CreatedAt, &user.UpdatedAt)
}

func (r *UserRepository) GetByID(id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, pin_hash, display_name, first_name, last_name, role, is_active, created_at, updated_at
		FROM users WHERE id = $1`
	
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.PinHash, &user.DisplayName, &user.FirstName, &user.LastName, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err == nil {
		user.HasPin = user.PinHash != nil && *user.PinHash != ""
	}
	return user, err
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, pin_hash, display_name, first_name, last_name, role, is_active, created_at, updated_at
		FROM users WHERE username = $1`
	
	err := r.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.PinHash, &user.DisplayName, &user.FirstName, &user.LastName, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err == nil {
		user.HasPin = user.PinHash != nil && *user.PinHash != ""
	}
	return user, err
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, pin_hash, display_name, first_name, last_name, role, is_active, created_at, updated_at
		FROM users WHERE email = $1`
	
	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.PinHash, &user.DisplayName, &user.FirstName, &user.LastName, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err == nil {
		user.HasPin = user.PinHash != nil && *user.PinHash != ""
	}
	return user, err
}

func (r *UserRepository) List() ([]*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, pin_hash, display_name, first_name, last_name, role, is_active, created_at, updated_at
		FROM users ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		user := &models.User{}
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash,
			&user.PinHash, &user.DisplayName, &user.FirstName, &user.LastName, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		user.HasPin = user.PinHash != nil && *user.PinHash != ""
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) Update(user *models.User) error {
	query := `
		UPDATE users 
		SET username = $1, email = $2, role = $3, is_active = $4, display_name = $5, first_name = $6, last_name = $7, updated_at = CURRENT_TIMESTAMP
		WHERE id = $8`
	
	result, err := r.db.Exec(query, user.Username, user.Email, user.Role, user.IsActive, user.DisplayName, user.FirstName, user.LastName, user.ID)
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
