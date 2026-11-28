package db

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/scan"
)

// itemLock in locked_fields locks the whole title.
const itemLock = "*"

// splitLocks separates the whole-title lock from the field locks.
func splitLocks(stored []string) (fields []string, locked bool) {
	fields = []string{}
	for _, f := range stored {
		if f == itemLock {
			locked = true
		} else {
			fields = append(fields, f)
		}
	}
	return fields, locked
}

// EditTitle saves a person's edit of a title's details and locks every field
// it changes. It returns catalog.ErrNotFound for an unknown title.
func (d *DB) EditTitle(ctx context.Context, id int64, e catalog.TitleEdit) error {
	var sets []string
	var args []any
	var locks []string
	add := func(field, assignment string, v any) {
		args = append(args, v)
		sets = append(sets, strings.ReplaceAll(assignment, "?", "$"+itoa(len(args)+1)))
		if !slices.Contains(locks, field) {
			locks = append(locks, field)
		}
	}
	if e.Name != nil {
		name := strings.TrimSpace(*e.Name)
		add(catalog.FieldName, "name = ?", name)
		add(catalog.FieldName, "sort_name = ?", scan.SortName(name))
	}
	if e.Year != nil {
		add(catalog.FieldYear, "year = NULLIF(?::integer, 0)", *e.Year)
	}
	if e.Artist != nil {
		add(catalog.FieldArtist, "artist = NULLIF(?, '')", strings.TrimSpace(*e.Artist))
	}
	if e.Overview != nil {
		add(catalog.FieldOverview, "overview = NULLIF(?, '')", strings.TrimSpace(*e.Overview))
	}
	if e.Genres != nil {
		g := []string{}
		for _, x := range *e.Genres {
			if x = strings.TrimSpace(x); x != "" && !slices.Contains(g, x) {
				g = append(g, x)
			}
		}
		add(catalog.FieldGenres, "genres = ?::text[]", g)
	}
	if e.Series != nil {
		add(catalog.FieldSeries, "series = NULLIF(?, '')", strings.TrimSpace(*e.Series))
	}
	if e.SeriesIndex != nil {
		add(catalog.FieldSeriesIndex, "series_index = NULLIF(?::numeric, 0)", *e.SeriesIndex)
	}
	if e.Narrator != nil {
		add(catalog.FieldNarrator, "narrator = NULLIF(?, '')", strings.TrimSpace(*e.Narrator))
	}
	if e.Publisher != nil {
		add(catalog.FieldPublisher, "publisher = NULLIF(?, '')", strings.TrimSpace(*e.Publisher))
	}
	if len(sets) == 0 {
		return d.titleExists(ctx, id)
	}
	args = append(args, locks)
	sql := "UPDATE titles SET " + strings.Join(sets, ", ") +
		", locked_fields = ARRAY(SELECT DISTINCT unnest(locked_fields || $" + itoa(len(args)+1) + "::text[]) ORDER BY 1)" +
		", updated_at = now() WHERE id = $1"
	tag, err := d.Pool.Exec(ctx, sql, append([]any{id}, args...)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalog.ErrNotFound
	}
	return nil
}

// SetTitleLocks replaces a title's locks: the whole-title lock and the
// fields kept as edited. Unlocking a field lets the next lookup change it.
func (d *DB) SetTitleLocks(ctx context.Context, id int64, l catalog.TitleLocks) (catalog.TitleLocks, error) {
	stored := []string{}
	for _, f := range l.Fields {
		if f = strings.TrimSpace(f); f != "" && f != itemLock && !slices.Contains(stored, f) {
			stored = append(stored, f)
		}
	}
	slices.Sort(stored)
	if l.Locked {
		stored = append([]string{itemLock}, stored...)
	}
	tag, err := d.Pool.Exec(ctx, `UPDATE titles SET locked_fields = $2::text[], updated_at = now() WHERE id = $1`, id, stored)
	if err != nil {
		return l, err
	}
	if tag.RowsAffected() == 0 {
		return l, catalog.ErrNotFound
	}
	fields, locked := splitLocks(stored)
	return catalog.TitleLocks{Locked: locked, Fields: fields}, nil
}

// SetPreferLocalMetadata sets a library's choice of its files' metadata over
// lookups.
func (d *DB) SetPreferLocalMetadata(ctx context.Context, libraryID int64, prefer bool) (catalog.Library, error) {
	tag, err := d.Pool.Exec(ctx, `UPDATE libraries SET prefer_local_metadata = $2 WHERE id = $1`, libraryID, prefer)
	if err != nil {
		return catalog.Library{}, err
	}
	if tag.RowsAffected() == 0 {
		return catalog.Library{}, catalog.ErrNotFound
	}
	return d.Library(ctx, libraryID)
}

func (d *DB) titleExists(ctx context.Context, id int64) error {
	var ok bool
	if err := d.Pool.QueryRow(ctx, sqlTitleExists, id).Scan(&ok); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if !ok {
		return catalog.ErrNotFound
	}
	return nil
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
