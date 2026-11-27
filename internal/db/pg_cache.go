package db

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JustinTDCT/CineVault/internal/cachesync"
	"github.com/JustinTDCT/CineVault/pkg/cacheapi"
)

const (
	// Matched movies and shows: everything the cache can know about.
	sqlCacheItems = `
		SELECT t.id, t.media_type, t.tmdb_id, t.name, t.cache_state, t.cache_version,
		       COALESCE(t.cache_retry_at, 'epoch'::timestamptz), COALESCE(t.contributed_hash, '')
		FROM titles t
		WHERE t.media_type IN ('movie', 'tv') AND t.tmdb_id IS NOT NULL AND t.match_status = 'matched'
		ORDER BY t.id`

	// Editions with a file present, longest file first, in Edition List order.
	sqlCacheOwned = `
		SELECT e.title_id, e.edition_type_id, COALESCE(max(v.duration_seconds), 0)::float8
		FROM editions e
		JOIN titles t ON t.id = e.title_id
		JOIN edition_types et ON et.id = e.edition_type_id
		JOIN versions v ON v.edition_id = e.id AND v.missing_since IS NULL
		WHERE t.media_type IN ('movie', 'tv') AND t.tmdb_id IS NOT NULL
		GROUP BY e.title_id, e.edition_type_id, et.sort_order
		ORDER BY e.title_id, et.sort_order`

	sqlSetCacheState = `
		UPDATE titles SET cache_state = $2, cache_retry_at = $3, cache_error = NULLIF($4, ''),
		       cache_checked_at = now()
		WHERE id = $1`

	sqlApplyTitle = `
		UPDATE titles SET cache_state = 'cached', cache_version = $2, cache_withdrawn = $3,
		       cache_sources = $4, cache_checked_at = now(), cache_retry_at = NULL, cache_error = NULL
		WHERE id = $1`

	// A published edition: update it, or add it as a placeholder ($3) when the
	// library doesn't have it.
	sqlApplyEdition = `
		INSERT INTO editions (title_id, edition_type_id, from_cache, cache_display_name, cache_runtime_min,
		                      cache_runtime_sources, cache_difference_short, cache_difference_full,
		                      cache_confidence, cache_status)
		SELECT $1, $2, true, NULLIF($4, ''), NULLIF($5::integer, 0), $6, NULLIF($7, ''), $8, NULLIF($9, ''), 'published'
		WHERE EXISTS (SELECT 1 FROM edition_types WHERE id = $2) -- skip editions newer than our Edition List
		  AND ($3 OR EXISTS (SELECT 1 FROM editions WHERE title_id = $1 AND edition_type_id = $2))
		ON CONFLICT (title_id, edition_type_id) DO UPDATE SET
		    cache_display_name = EXCLUDED.cache_display_name, cache_runtime_min = EXCLUDED.cache_runtime_min,
		    cache_runtime_sources = EXCLUDED.cache_runtime_sources,
		    cache_difference_short = EXCLUDED.cache_difference_short,
		    cache_difference_full = EXCLUDED.cache_difference_full,
		    cache_confidence = EXCLUDED.cache_confidence, cache_status = 'published'`

	// A withdrawn edition keeps what it had and is marked "No longer verified".
	sqlWithdrawEdition = `UPDATE editions SET cache_status = 'withdrawn' WHERE title_id = $1 AND edition_type_id = $2`

	sqlSetContributed = `UPDATE titles SET contributed_hash = $2 WHERE id = $1`

	sqlGetSetting  = `SELECT value FROM settings WHERE key = $1`
	sqlSaveSetting = `
		INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`

	sqlTitleCacheKey = `SELECT media_type, COALESCE(tmdb_id, 0) FROM titles WHERE id = $1`
)

// CacheItems implements cachesync.Store.
func (d *DB) CacheItems(ctx context.Context) ([]cachesync.Item, error) {
	rows, err := d.Pool.Query(ctx, sqlCacheItems)
	if err != nil {
		return nil, err
	}
	var items []cachesync.Item
	index := map[int64]int{}
	for rows.Next() {
		var it cachesync.Item
		if err := rows.Scan(&it.TitleID, &it.Key.Kind, &it.Key.TMDbID, &it.Name, &it.State, &it.Version, &it.RetryAt, &it.ContributedHash); err != nil {
			rows.Close()
			return nil, err
		}
		index[it.TitleID] = len(items)
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = d.Pool.Query(ctx, sqlCacheOwned)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var e cacheapi.ContributeEdition
		var secs float64
		if err := rows.Scan(&id, &e.Edition, &secs); err != nil {
			return nil, err
		}
		i, ok := index[id]
		if !ok {
			continue
		}
		// A show's edition holds many episodes; only a movie has one runtime.
		if items[i].Key.Kind == cacheapi.KindMovie {
			e.RuntimeMin = min(cacheapi.MaxRuntimeMin, int(math.Round(secs/60)))
		}
		items[i].Owned = append(items[i].Owned, e)
	}
	return items, rows.Err()
}

// SetCacheState implements cachesync.Store.
func (d *DB) SetCacheState(ctx context.Context, titleID int64, state string, retryAt time.Time, msg string) error {
	var retry any
	if !retryAt.IsZero() {
		retry = retryAt
	}
	_, err := d.Pool.Exec(ctx, sqlSetCacheState, titleID, state, retry, msg)
	return err
}

// ApplyRecord implements cachesync.Store: it records what the cache knows
// about each edition, adds editions a movie's library doesn't have as
// placeholders, and never removes anything (cache spec §7).
func (d *DB) ApplyRecord(ctx context.Context, titleID int64, rec cacheapi.Record) error {
	sources, err := json.Marshal(rec.Sources)
	if err != nil {
		return err
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	placeholders := rec.Key.Kind == cacheapi.KindMovie
	for _, e := range rec.Editions {
		if e.Status == cacheapi.EditionWithdrawn {
			if _, err := tx.Exec(ctx, sqlWithdrawEdition, titleID, e.Edition); err != nil {
				return err
			}
			continue
		}
		var full []byte
		if len(e.DifferenceFull) > 0 {
			if full, err = json.Marshal(e.DifferenceFull); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, sqlApplyEdition, titleID, e.Edition, placeholders, e.DisplayName, e.RuntimeMin,
			e.RuntimeSources, e.DifferenceShort, full, e.Confidence); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, sqlApplyTitle, titleID, rec.RecordVersion, rec.Status == cacheapi.StatusWithdrawn, sources); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetContributed implements cachesync.Store.
func (d *DB) SetContributed(ctx context.Context, hashes map[int64]string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for id, h := range hashes {
		if _, err := tx.Exec(ctx, sqlSetContributed, id, h); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Setting reads a setting into v; ok is false when it isn't set.
func (d *DB) Setting(ctx context.Context, key string, v any) (bool, error) {
	var raw []byte
	err := d.Pool.QueryRow(ctx, sqlGetSetting, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, v)
}

// SaveSetting stores a setting as JSON.
func (d *DB) SaveSetting(ctx context.Context, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = d.Pool.Exec(ctx, sqlSaveSetting, key, b)
	return err
}

// TitleCacheKey returns the cache key of a matched movie or show, or
// catalog-style not found when the title has none.
func (d *DB) TitleCacheKey(ctx context.Context, titleID int64) (cacheapi.Key, bool, error) {
	var k cacheapi.Key
	err := d.Pool.QueryRow(ctx, sqlTitleCacheKey, titleID).Scan(&k.Kind, &k.TMDbID)
	if errors.Is(err, pgx.ErrNoRows) {
		return k, false, nil
	}
	if err != nil {
		return k, false, err
	}
	return k, (k.Kind == cacheapi.KindMovie || k.Kind == cacheapi.KindTV) && k.TMDbID > 0, nil
}
