package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/scan"
)

// The DB is the scanner's store.
var _ scan.Store = (*DB)(nil)

// CreateLibrary adds a library. A duplicate name returns catalog.ErrConflict.
func (d *DB) CreateLibrary(ctx context.Context, lib catalog.Library) (catalog.Library, error) {
	err := d.Pool.QueryRow(ctx, sqlCreateLibrary, lib.Name, lib.MediaType, lib.Paths).
		Scan(&lib.ID, &lib.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return lib, catalog.ErrConflict
	}
	return lib, err
}

// Libraries lists all libraries by name.
func (d *DB) Libraries(ctx context.Context) ([]catalog.Library, error) {
	rows, err := d.Pool.Query(ctx, sqlLibraries)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []catalog.Library{}
	for rows.Next() {
		var l catalog.Library
		if err := rows.Scan(&l.ID, &l.Name, &l.MediaType, &l.Paths, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Library returns one library, or catalog.ErrNotFound.
func (d *DB) Library(ctx context.Context, id int64) (catalog.Library, error) {
	var l catalog.Library
	err := d.Pool.QueryRow(ctx, sqlLibrary, id).Scan(&l.ID, &l.Name, &l.MediaType, &l.Paths, &l.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, catalog.ErrNotFound
	}
	return l, err
}

// LibraryTitles returns every title in the library with its editions and versions.
func (d *DB) LibraryTitles(ctx context.Context, libraryID int64) ([]catalog.Title, error) {
	rows, err := d.Pool.Query(ctx, sqlLibraryTitles, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []catalog.Title{}
	for rows.Next() {
		var t catalog.Title
		var e catalog.Edition
		var v catalog.Version
		if err := rows.Scan(
			&t.ID, &t.LibraryID, &t.MediaType, &t.Name, &t.SortName, &t.Year, &t.TMDbID,
			&e.ID, &e.EditionTypeID, &e.DisplayName, &e.ShortLabel, &e.RuntimeSeconds,
			&v.ID, &v.Path, &v.SizeBytes, &v.Label, &v.Container, &v.VideoCodec,
			&v.Width, &v.Height, &v.HDR, &v.AudioCodec, &v.AudioChannels,
			&v.DurationSeconds, &v.Bitrate, &v.EditionLocked, &v.Missing,
		); err != nil {
			return nil, err
		}
		// Rows arrive grouped by title, then edition; fold them into a tree.
		if n := len(out); n == 0 || out[n-1].ID != t.ID {
			t.Editions = []catalog.Edition{}
			out = append(out, t)
		}
		title := &out[len(out)-1]
		if n := len(title.Editions); n == 0 || title.Editions[n-1].ID != e.ID {
			e.Versions = []catalog.Version{}
			title.Editions = append(title.Editions, e)
		}
		if v.ID != 0 {
			ed := &title.Editions[len(title.Editions)-1]
			ed.Versions = append(ed.Versions, v)
		}
	}
	return out, rows.Err()
}

// KnownVersions implements scan.Store.
func (d *DB) KnownVersions(ctx context.Context, libraryID int64) (map[string]scan.KnownVersion, error) {
	rows, err := d.Pool.Query(ctx, sqlKnownVersions, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]scan.KnownVersion{}
	for rows.Next() {
		var k scan.KnownVersion
		var path string
		if err := rows.Scan(&k.ID, &path, &k.Size); err != nil {
			return nil, err
		}
		out[path] = k
	}
	return out, rows.Err()
}

// FindOrCreateTitle implements scan.Store.
func (d *DB) FindOrCreateTitle(ctx context.Context, libraryID int64, mediaType string, p scan.ParsedName) (int64, error) {
	var id int64
	err := d.Pool.QueryRow(ctx, sqlFindTitle, libraryID, p.Title, p.Year).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	err = d.Pool.QueryRow(ctx, sqlInsertTitle, libraryID, mediaType, p.Title, p.SortName, p.Year, p.TMDbID).Scan(&id)
	return id, err
}

// FindOrCreateEdition implements scan.Store.
func (d *DB) FindOrCreateEdition(ctx context.Context, titleID int64, editionTypeID string, runtimeSeconds int) (int64, error) {
	var id int64
	err := d.Pool.QueryRow(ctx, sqlUpsertEdition, titleID, editionTypeID, runtimeSeconds).Scan(&id)
	return id, err
}

// UpsertVersion implements scan.Store.
func (d *DB) UpsertVersion(ctx context.Context, editionID int64, v scan.NewVersion) error {
	i := v.Info
	_, err := d.Pool.Exec(ctx, sqlUpsertVersion,
		editionID, v.Path, v.SizeBytes, v.Fingerprint, i.Label(), i.Container,
		i.VideoCodec, i.Width, i.Height, i.HDR, i.AudioCodec, i.AudioChannels,
		i.DurationSeconds, i.Bitrate)
	return err
}

// SetMissing implements scan.Store.
func (d *DB) SetMissing(ctx context.Context, ids []int64, missing bool) error {
	if len(ids) == 0 {
		return nil
	}
	q := sqlSetPresent
	if missing {
		q = sqlSetMissing
	}
	_, err := d.Pool.Exec(ctx, q, ids)
	return err
}
