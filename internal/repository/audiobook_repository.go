package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type AudiobookRepository struct {
	db *sql.DB
}

func NewAudiobookRepository(db *sql.DB) *AudiobookRepository {
	return &AudiobookRepository{db: db}
}

// ──────────────────── Authors ────────────────────

func (r *AudiobookRepository) CreateAuthor(a *models.Author) error {
	query := `
		INSERT INTO authors (id, library_id, name, sort_name, description, poster_path, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, a.ID, a.LibraryID, a.Name, a.SortName,
		a.Description, a.PosterPath, a.SortPosition).
		Scan(&a.CreatedAt, &a.UpdatedAt)
}

func (r *AudiobookRepository) GetAuthorByID(id uuid.UUID) (*models.Author, error) {
	a := &models.Author{}
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path,
		       sort_position, created_at, updated_at
		FROM authors WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&a.ID, &a.LibraryID, &a.Name, &a.SortName, &a.Description,
		&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("author not found")
	}
	return a, err
}

func (r *AudiobookRepository) ListAuthorsByLibrary(libraryID uuid.UUID) ([]*models.Author, error) {
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path,
		       sort_position, created_at, updated_at
		FROM authors WHERE library_id = $1 ORDER BY COALESCE(sort_name, name)`
	rows, err := r.db.Query(query, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []*models.Author
	for rows.Next() {
		a := &models.Author{}
		if err := rows.Scan(&a.ID, &a.LibraryID, &a.Name, &a.SortName,
			&a.Description, &a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		authors = append(authors, a)
	}
	return authors, rows.Err()
}

func (r *AudiobookRepository) FindAuthorByName(libraryID uuid.UUID, name string) (*models.Author, error) {
	a := &models.Author{}
	query := `
		SELECT id, library_id, name, sort_name, description, poster_path,
		       sort_position, created_at, updated_at
		FROM authors WHERE library_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, libraryID, name).Scan(
		&a.ID, &a.LibraryID, &a.Name, &a.SortName, &a.Description,
		&a.PosterPath, &a.SortPosition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (r *AudiobookRepository) DeleteAuthor(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM authors WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("author not found")
	}
	return nil
}

// ──────────────────── Book Series ────────────────────

func (r *AudiobookRepository) CreateSeries(s *models.BookSeries) error {
	query := `
		INSERT INTO book_series (id, author_id, title, sort_title, description, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, s.ID, s.AuthorID, s.Title, s.SortTitle,
		s.Description, s.SortPosition).
		Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *AudiobookRepository) ListSeriesByAuthor(authorID uuid.UUID) ([]*models.BookSeries, error) {
	query := `
		SELECT id, author_id, title, sort_title, description, sort_position, created_at, updated_at
		FROM book_series WHERE author_id = $1 ORDER BY COALESCE(sort_title, title)`
	rows, err := r.db.Query(query, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []*models.BookSeries
	for rows.Next() {
		s := &models.BookSeries{}
		if err := rows.Scan(&s.ID, &s.AuthorID, &s.Title, &s.SortTitle,
			&s.Description, &s.SortPosition, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		series = append(series, s)
	}
	return series, rows.Err()
}

// ──────────────────── Books ────────────────────

func (r *AudiobookRepository) CreateBook(b *models.Book) error {
	query := `
		INSERT INTO books (id, author_id, series_id, library_id, title, sort_title, year,
		                   description, narrator, poster_path, sort_position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`
	return r.db.QueryRow(query, b.ID, b.AuthorID, b.SeriesID, b.LibraryID,
		b.Title, b.SortTitle, b.Year, b.Description, b.Narrator,
		b.PosterPath, b.SortPosition).
		Scan(&b.CreatedAt, &b.UpdatedAt)
}

func (r *AudiobookRepository) GetBookByID(id uuid.UUID) (*models.Book, error) {
	b := &models.Book{}
	query := `
		SELECT id, author_id, series_id, library_id, title, sort_title, year,
		       description, narrator, poster_path, sort_position, created_at, updated_at
		FROM books WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(
		&b.ID, &b.AuthorID, &b.SeriesID, &b.LibraryID, &b.Title, &b.SortTitle,
		&b.Year, &b.Description, &b.Narrator, &b.PosterPath,
		&b.SortPosition, &b.CreatedAt, &b.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("book not found")
	}
	return b, err
}

func (r *AudiobookRepository) ListBooksByAuthor(authorID uuid.UUID) ([]*models.Book, error) {
	query := `
		SELECT id, author_id, series_id, library_id, title, sort_title, year,
		       description, narrator, poster_path, sort_position, created_at, updated_at
		FROM books WHERE author_id = $1 ORDER BY COALESCE(sort_title, title)`
	rows, err := r.db.Query(query, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*models.Book
	for rows.Next() {
		b := &models.Book{}
		if err := rows.Scan(&b.ID, &b.AuthorID, &b.SeriesID, &b.LibraryID,
			&b.Title, &b.SortTitle, &b.Year, &b.Description, &b.Narrator,
			&b.PosterPath, &b.SortPosition, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (r *AudiobookRepository) FindBookByTitle(authorID uuid.UUID, title string) (*models.Book, error) {
	b := &models.Book{}
	query := `
		SELECT id, author_id, series_id, library_id, title, sort_title, year,
		       description, narrator, poster_path, sort_position, created_at, updated_at
		FROM books WHERE author_id = $1 AND LOWER(title) = LOWER($2) LIMIT 1`
	err := r.db.QueryRow(query, authorID, title).Scan(
		&b.ID, &b.AuthorID, &b.SeriesID, &b.LibraryID, &b.Title, &b.SortTitle,
		&b.Year, &b.Description, &b.Narrator, &b.PosterPath,
		&b.SortPosition, &b.CreatedAt, &b.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

func (r *AudiobookRepository) DeleteBook(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM books WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("book not found")
	}
	return nil
}
