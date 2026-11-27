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
		if err := rows.Scan(&t.ID, &t.MediaType, &t.Name, &t.Year, &t.TMDbID); err != nil {
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

// SaveSeason implements match.Store. The season and its episodes are saved
// together, so a failure leaves none of them half-written.
func (d *DB) SaveSeason(ctx context.Context, titleID int64, s tmdb.Season) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, sqlSaveSeason, titleID, s.SeasonNumber, s.Name, s.Overview, s.PosterPath, s.AirDate); err != nil {
		return err
	}
	for _, e := range s.Episodes {
		if _, err := tx.Exec(ctx, sqlSaveEpisode, titleID, s.SeasonNumber, e.EpisodeNumber, e.Name, e.Overview,
			e.AirDate, e.Runtime, e.StillPath, e.VoteAverage, e.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ShowsNeedingEpisodes implements match.Store.
func (d *DB) ShowsNeedingEpisodes(ctx context.Context, libraryID int64) ([]match.ShowSeasons, error) {
	rows, err := d.Pool.Query(ctx, sqlShowsNeedingEpisodes, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []match.ShowSeasons{}
	for rows.Next() {
		var s match.ShowSeasons
		var seasons []int32
		if err := rows.Scan(&s.TitleID, &s.Name, &s.TMDbID, &seasons); err != nil {
			return nil, err
		}
		for _, n := range seasons {
			s.Seasons = append(s.Seasons, int(n))
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
