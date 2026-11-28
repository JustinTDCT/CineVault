package db

import (
	"fmt"
	"strings"
)

// Automatic writes: lookups (TMDb, MusicBrainz, Open Library, Comic Vine)
// and CineVault Cache records change a title's fields only where the user
// lets them. The SQL below runs as
//
//	UPDATE titles t SET … FROM libraries l WHERE t.id = $1 AND l.id = t.library_id
//
// so every SET expression can see the title (t) and its library (l).

// auto returns the SQL to set column col from newExpr in an automatic write.
// The current value stays when:
//   - the whole title is locked ('*' in locked_fields)
//   - field (the name the edit API and locks use) was edited by hand
//   - the library prefers its files' metadata and col already has a value,
//     so the lookup only fills a gap
func auto(col, field, newExpr string) string {
	return fmt.Sprintf("CASE WHEN '*' = ANY(t.locked_fields) OR '%s' = ANY(t.locked_fields)"+
		" OR (l.prefer_local_metadata AND COALESCE(t.%s::text, '') NOT IN ('', '{}', '[]', 'null'))"+
		" THEN t.%s ELSE %s END", field, col, col, newExpr)
}

// set is one column of an automatic write: its field name for locks, and
// the new value's SQL.
type set struct{ col, field, expr string }

// automaticUpdate builds the UPDATE for an automatic write of the given
// columns; extra are plain assignments (IDs, match status) that always apply.
func automaticUpdate(sets []set, extra ...string) string {
	parts := make([]string, 0, len(sets)+len(extra)+1)
	for _, s := range sets {
		parts = append(parts, s.col+" = "+auto(s.col, s.field, s.expr))
	}
	parts = append(parts, extra...)
	parts = append(parts, "updated_at = now()")
	return "UPDATE titles t SET\n\t" + strings.Join(parts, ",\n\t") +
		"\nFROM libraries l WHERE t.id = $1 AND l.id = t.library_id"
}

// sqlSaveCredits stores a title's TMDb cast and directors.
var sqlSaveCredits = automaticUpdate([]set{
	{"cast_members", "cast", "$2::jsonb"},
	{"directors", "directors", "$3::text[]"},
})

// sqlSaveMatch stores a TMDb match. The year stays as read from files, so
// later scans still find the title; TMDb's full date is in release_date.
var sqlSaveMatch = automaticUpdate([]set{
	{"name", "name", "$4"},
	{"sort_name", "name", "$5"},
	{"original_title", "original_title", "NULLIF($6, '')"},
	{"overview", "overview", "NULLIF($7, '')"},
	{"tagline", "tagline", "NULLIF($8, '')"},
	{"release_date", "release_date", "NULLIF($9::text, '')::date"},
	{"year", "year", "COALESCE(t.year, NULLIF($10::integer, 0))"},
	{"runtime_minutes", "runtime", "NULLIF($11::integer, 0)"},
	{"poster_path", "poster", "NULLIF($12, '')"},
	{"backdrop_path", "backdrop", "NULLIF($13, '')"},
	{"genres", "genres", "$14::text[]"},
	{"rating", "rating", "NULLIF($15::float8, 0)::numeric(3, 1)"},
	{"collection_tmdb_id", "collection", "NULLIF($16::integer, 0)"},
	{"collection_name", "collection", "NULLIF($17, '')"},
	{"cast_members", "cast", "$18::jsonb"},
	{"directors", "directors", "$19::text[]"},
},
	"tmdb_id = $2", "imdb_id = NULLIF($3, '')",
	"match_status = 'matched'", "match_error = NULL", "matched_at = now()")
