package db

import (
	"context"

	"github.com/JustinTDCT/CineVault/internal/match"
	"github.com/JustinTDCT/CineVault/internal/scan"
)

// The DB stores what the album, book and comic lookups find.
var _ match.LookupStore = (*DB)(nil)

// LookupItems implements match.LookupStore: titles of the library to look
// up, by the name and artist they show (a person's edit included, so a
// corrected name finds the right match). Locked titles are left alone.
func (d *DB) LookupItems(ctx context.Context, libraryID int64, all bool) ([]match.LookupItem, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, media_type, name, COALESCE(NULLIF(artist, ''), file_artist, ''), COALESCE(year, file_year, 0)
		FROM titles
		WHERE library_id = $1 AND NOT ('*' = ANY(locked_fields))
		  AND ($2 OR match_status IN ('unmatched', 'error'))
		ORDER BY sort_name, id`, libraryID, all)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []match.LookupItem{}
	for rows.Next() {
		var it match.LookupItem
		if err := rows.Scan(&it.ID, &it.MediaType, &it.Name, &it.Artist, &it.Year); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// SaveLookup implements match.LookupStore.
func (d *DB) SaveLookup(ctx context.Context, titleID int64, f match.Found) error {
	_, err := d.Pool.Exec(ctx, sqlSaveLookup, titleID, f.MBReleaseGroupID, f.OLWorkID, f.ComicVineID,
		f.Name, scan.SortName(f.Name), f.Artist, f.Year, f.Overview, f.Genres, f.CoverURL, f.Publisher)
	return err
}

// sqlNewLookupID is true while saving a match whose source ID differs from
// the title's: a new cache key, so the cache state starts over.
const sqlNewLookupID = `(t.mb_release_group_id, t.ol_work_id, t.comicvine_id) IS DISTINCT FROM
	(NULLIF($2, ''), NULLIF($3, ''), NULLIF($4::integer, 0))`

// keepIfEmpty uses the found value, or the title's own when nothing was found.
func keepIfEmpty(param, col string) string {
	return "COALESCE(NULLIF(" + param + ", ''), t." + col + ")"
}

var sqlSaveLookup = automaticUpdate([]set{
	{"name", "name", keepIfEmpty("$5", "name")},
	{"sort_name", "name", keepIfEmpty("$6", "sort_name")},
	{"artist", "artist", keepIfEmpty("$7", "artist")},
	{"year", "year", "COALESCE(NULLIF($8::integer, 0), t.year)"},
	{"overview", "overview", keepIfEmpty("$9", "overview")},
	{"genres", "genres", "CASE WHEN cardinality($10::text[]) > 0 THEN $10::text[] ELSE t.genres END"},
	{"cover_url", "poster", keepIfEmpty("$11", "cover_url")},
	{"publisher", "publisher", keepIfEmpty("$12", "publisher")},
},
	"mb_release_group_id = NULLIF($2, '')",
	"ol_work_id = NULLIF($3, '')",
	"comicvine_id = NULLIF($4::integer, 0)",
	"match_status = 'matched'", "match_error = NULL", "matched_at = now()",
	"cache_state = CASE WHEN "+sqlNewLookupID+" THEN 'none' ELSE cache_state END",
	"cache_version = CASE WHEN "+sqlNewLookupID+" THEN 0 ELSE cache_version END",
	"cache_checked_at = CASE WHEN "+sqlNewLookupID+" THEN NULL ELSE cache_checked_at END",
	"cache_retry_at = CASE WHEN "+sqlNewLookupID+" THEN NULL ELSE cache_retry_at END",
	"cache_error = CASE WHEN "+sqlNewLookupID+" THEN NULL ELSE cache_error END",
	"cache_withdrawn = CASE WHEN "+sqlNewLookupID+" THEN false ELSE cache_withdrawn END",
	"cache_sources = CASE WHEN "+sqlNewLookupID+" THEN NULL ELSE cache_sources END",
	"contributed_hash = CASE WHEN "+sqlNewLookupID+" THEN NULL ELSE contributed_hash END",
)
