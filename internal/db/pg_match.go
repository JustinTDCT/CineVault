package db

import (
	"context"

	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/internal/tmdb"
)

// The DB is the matcher's store.
var _ match.Store = (*DB)(nil)

// TitlesToMatch implements match.Store.
func (d *DB) TitlesToMatch(ctx context.Context, libraryID int64) ([]match.Title, error) {
	rows, err := d.Pool.Query(ctx, sqlTitlesToMatch, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []match.Title{}
	for rows.Next() {
		var t match.Title
		if err := rows.Scan(&t.ID, &t.Name, &t.Year, &t.TMDbID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveMatch implements match.Store.
func (d *DB) SaveMatch(ctx context.Context, titleID int64, m tmdb.Movie) error {
	collectionID, collectionName := 0, ""
	if m.Collection != nil {
		collectionID, collectionName = m.Collection.ID, m.Collection.Name
	}
	_, err := d.Pool.Exec(ctx, sqlSaveMatch,
		titleID, m.ID, m.IMDbID, m.Title, scan.SortName(m.Title), m.OriginalTitle,
		m.Overview, m.Tagline, m.ReleaseDate, m.Year(), m.Runtime,
		m.PosterPath, m.BackdropPath, m.GenreNames(), m.VoteAverage,
		collectionID, collectionName)
	return err
}

// SaveMatchFailure implements match.Store.
func (d *DB) SaveMatchFailure(ctx context.Context, titleID int64, status, message string) error {
	_, err := d.Pool.Exec(ctx, sqlSaveMatchFailure, titleID, status, message)
	return err
}
