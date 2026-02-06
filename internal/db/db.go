package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/JustinTDCT/CineVault/internal/config"
)

type DB struct {
	*sql.DB
}

func Connect(cfg *config.DatabaseConfig) (*DB, error) {
	db, err := sql.Open("postgres", cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return &DB{db}, nil
}
