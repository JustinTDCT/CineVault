package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/scan"
	"github.com/JustinTDCT/CineVault/internal/tmdb"
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
			&t.OriginalTitle, &t.Overview, &t.Tagline, &t.ReleaseDate, &t.RuntimeMinutes,
			&t.PosterPath, &t.BackdropPath, &t.IMDbID, &t.Genres, &t.Rating,
			&t.CollectionID, &t.CollectionName, &t.MatchStatus, &t.MatchError, &t.Artist,
			&t.CoverVersionID, &t.TrackCount,
			&t.SeasonCount, &t.EpisodeCount,
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
			if t.Genres == nil {
				t.Genres = []string{}
			}
			if t.PosterPath != "" {
				t.PosterURL = tmdb.ImageBaseURL + "w500" + t.PosterPath
			}
			if t.BackdropPath != "" {
				t.BackdropURL = tmdb.ImageBaseURL + "w1280" + t.BackdropPath
			}
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
	err := d.Pool.QueryRow(ctx, sqlFindTitle, libraryID, p.Title, p.Year, p.Artist).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	err = d.Pool.QueryRow(ctx, sqlInsertTitle, libraryID, mediaType, p.Title, p.SortName, p.Year, p.TMDbID, p.Artist).Scan(&id)
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
		i.DurationSeconds, i.Bitrate, v.EpisodeID, v.LastEpisode, v.TrackID)
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

// Version returns one file by ID, or catalog.ErrNotFound.
func (d *DB) Version(ctx context.Context, id int64) (catalog.Version, error) {
	var v catalog.Version
	err := d.Pool.QueryRow(ctx, sqlVersion, id).Scan(
		&v.ID, &v.Path, &v.SizeBytes, &v.Label, &v.Container, &v.VideoCodec, &v.Width, &v.Height,
		&v.HDR, &v.AudioCodec, &v.AudioChannels, &v.DurationSeconds, &v.Bitrate, &v.EditionLocked, &v.Missing, &v.LastEpisode)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, catalog.ErrNotFound
	}
	return v, err
}

// FindOrCreateEpisode implements scan.Store.
func (d *DB) FindOrCreateEpisode(ctx context.Context, titleID int64, season, episode int, name string) (int64, error) {
	var id int64
	err := d.Pool.QueryRow(ctx, sqlFindOrCreateEpisode, titleID, season, episode, name).Scan(&id)
	return id, err
}

// TitleEpisodes returns a show's seasons and episodes with their files, or
// catalog.ErrNotFound when there's no such title.
func (d *DB) TitleEpisodes(ctx context.Context, titleID int64) ([]catalog.Season, error) {
	var exists bool
	if err := d.Pool.QueryRow(ctx, sqlTitleExists, titleID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, catalog.ErrNotFound
	}
	rows, err := d.Pool.Query(ctx, sqlTitleEpisodes, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []catalog.Season{}
	for rows.Next() {
		var s catalog.Season
		var e catalog.Episode
		var v catalog.Version
		var poster, still string
		if err := rows.Scan(
			&s.Number, &s.Name, &s.Overview, &poster, &s.AirDate,
			&e.ID, &e.Number, &e.Name, &e.Overview, &e.AirDate, &e.RuntimeMinutes, &still, &e.Rating,
			&v.ID, &v.Path, &v.SizeBytes, &v.Label, &v.Container, &v.VideoCodec,
			&v.Width, &v.Height, &v.HDR, &v.AudioCodec, &v.AudioChannels,
			&v.DurationSeconds, &v.Bitrate, &v.EditionLocked, &v.Missing, &v.LastEpisode,
		); err != nil {
			return nil, err
		}
		// Rows arrive grouped by season, then episode; fold them into a tree.
		if n := len(out); n == 0 || out[n-1].Number != s.Number {
			if poster != "" {
				s.PosterURL = tmdb.ImageBaseURL + "w342" + poster
			}
			s.Episodes = []catalog.Episode{}
			out = append(out, s)
		}
		season := &out[len(out)-1]
		if n := len(season.Episodes); n == 0 || season.Episodes[n-1].ID != e.ID {
			e.Season = s.Number
			if still != "" {
				e.StillURL = tmdb.ImageBaseURL + "w300" + still
			}
			e.Versions = []catalog.Version{}
			season.Episodes = append(season.Episodes, e)
		}
		if v.ID != 0 {
			ep := &season.Episodes[len(season.Episodes)-1]
			ep.Versions = append(ep.Versions, v)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	catalog.LinkMultiEpisodeFiles(out)
	return out, nil
}

// FindOrCreateTrack implements scan.Store.
func (d *DB) FindOrCreateTrack(ctx context.Context, titleID int64, disc, number int, name, artist string) (int64, error) {
	var id int64
	err := d.Pool.QueryRow(ctx, sqlFindOrCreateTrack, titleID, disc, number, name, artist).Scan(&id)
	return id, err
}

// SetGenres implements scan.Store.
func (d *DB) SetGenres(ctx context.Context, titleID int64, genres []string) error {
	_, err := d.Pool.Exec(ctx, sqlSetGenres, titleID, genres)
	return err
}

// AlbumTracks returns an album's tracks per edition, or catalog.ErrNotFound
// when there's no such title.
func (d *DB) AlbumTracks(ctx context.Context, titleID int64) ([]catalog.AlbumEdition, error) {
	var exists bool
	if err := d.Pool.QueryRow(ctx, sqlTitleExists, titleID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, catalog.ErrNotFound
	}
	rows, err := d.Pool.Query(ctx, sqlAlbumTracks, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []catalog.AlbumEdition{}
	for rows.Next() {
		var editionID int64
		var tr catalog.Track
		var v catalog.Version
		if err := rows.Scan(&editionID, &tr.ID, &tr.Disc, &tr.Number, &tr.Name, &tr.Artist,
			&v.ID, &v.Path, &v.SizeBytes, &v.Label, &v.Container, &v.AudioCodec, &v.AudioChannels,
			&v.DurationSeconds, &v.Bitrate, &v.Missing); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].EditionID != editionID {
			out = append(out, catalog.AlbumEdition{EditionID: editionID, Tracks: []catalog.Track{}})
		}
		ed := &out[len(out)-1]
		if n := len(ed.Tracks); n == 0 || ed.Tracks[n-1].ID != tr.ID {
			tr.Versions = []catalog.Version{}
			ed.Tracks = append(ed.Tracks, tr)
		}
		t := &ed.Tracks[len(ed.Tracks)-1]
		t.Versions = append(t.Versions, v)
	}
	return out, rows.Err()
}
