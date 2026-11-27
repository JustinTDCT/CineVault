package db

// SQL for libraries and scanning. Kept in a driver-free file so the queries can
// be checked against PostgreSQL on their own.

const (
	sqlCreateLibrary = `
		INSERT INTO libraries (name, media_type, paths)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	sqlLibraries = `
		SELECT id, name, media_type, paths, created_at
		FROM libraries
		ORDER BY name`

	sqlLibrary = `
		SELECT id, name, media_type, paths, created_at
		FROM libraries
		WHERE id = $1`

	sqlKnownVersions = `
		SELECT v.id, v.path, v.size_bytes
		FROM versions v
		JOIN editions e ON e.id = v.edition_id
		JOIN titles t ON t.id = e.title_id
		WHERE t.library_id = $1`

	// $4 year and $6 tmdb_id: 0 means unknown.
	sqlFindTitle = `
		SELECT id FROM titles
		WHERE library_id = $1
		  AND lower(name) = lower($2)
		  AND year IS NOT DISTINCT FROM NULLIF($3::integer, 0)
		ORDER BY id
		LIMIT 1`

	sqlInsertTitle = `
		INSERT INTO titles (library_id, media_type, name, sort_name, year, tmdb_id)
		VALUES ($1, $2, $3, $4, NULLIF($5::integer, 0), NULLIF($6::integer, 0))
		RETURNING id`

	// Keeps an existing runtime; fills it in if it was empty.
	sqlUpsertEdition = `
		INSERT INTO editions (title_id, edition_type_id, runtime_seconds)
		VALUES ($1, $2, NULLIF($3::integer, 0))
		ON CONFLICT (title_id, edition_type_id) DO UPDATE
		   SET runtime_seconds = COALESCE(editions.runtime_seconds, EXCLUDED.runtime_seconds)
		RETURNING id`

	// A version whose edition the user locked keeps its edition.
	sqlUpsertVersion = `
		INSERT INTO versions (edition_id, path, size_bytes, fingerprint, label, container,
		                      video_codec, width, height, hdr, audio_codec, audio_channels,
		                      duration_seconds, bitrate)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''),
		        NULLIF($7, ''), NULLIF($8::integer, 0), NULLIF($9::integer, 0), NULLIF($10, ''),
		        NULLIF($11, ''), NULLIF($12::integer, 0),
		        NULLIF($13::float8, 0)::numeric(10,3), NULLIF($14::bigint, 0))
		ON CONFLICT (path) DO UPDATE SET
		    edition_id       = CASE WHEN versions.edition_locked THEN versions.edition_id ELSE EXCLUDED.edition_id END,
		    size_bytes       = EXCLUDED.size_bytes,
		    fingerprint      = EXCLUDED.fingerprint,
		    label            = EXCLUDED.label,
		    container        = EXCLUDED.container,
		    video_codec      = EXCLUDED.video_codec,
		    width            = EXCLUDED.width,
		    height           = EXCLUDED.height,
		    hdr              = EXCLUDED.hdr,
		    audio_codec      = EXCLUDED.audio_codec,
		    audio_channels   = EXCLUDED.audio_channels,
		    duration_seconds = EXCLUDED.duration_seconds,
		    bitrate          = EXCLUDED.bitrate,
		    missing_since    = NULL`

	sqlSetMissing = `
		UPDATE versions SET missing_since = now()
		WHERE id = ANY($1::bigint[]) AND missing_since IS NULL`

	sqlSetPresent = `
		UPDATE versions SET missing_since = NULL
		WHERE id = ANY($1::bigint[]) AND missing_since IS NOT NULL`

	// One row per version (or per edition with no versions), in display order.
	sqlLibraryTitles = `
		SELECT t.id, t.library_id, t.media_type, t.name, t.sort_name,
		       COALESCE(t.year, 0), COALESCE(t.tmdb_id, 0),
		       e.id, e.edition_type_id, et.display_name, et.short_label,
		       COALESCE(e.runtime_seconds, 0),
		       COALESCE(v.id, 0), COALESCE(v.path, ''), COALESCE(v.size_bytes, 0),
		       COALESCE(v.label, ''), COALESCE(v.container, ''), COALESCE(v.video_codec, ''),
		       COALESCE(v.width, 0), COALESCE(v.height, 0), COALESCE(v.hdr, ''),
		       COALESCE(v.audio_codec, ''), COALESCE(v.audio_channels, 0),
		       COALESCE(v.duration_seconds, 0)::float8, COALESCE(v.bitrate, 0),
		       COALESCE(v.edition_locked, false), v.missing_since IS NOT NULL
		FROM titles t
		JOIN editions e ON e.title_id = t.id
		JOIN edition_types et ON et.id = e.edition_type_id
		LEFT JOIN versions v ON v.edition_id = e.id
		WHERE t.library_id = $1
		ORDER BY t.sort_name, t.year NULLS LAST, t.id, et.sort_order, e.id,
		         v.height DESC NULLS LAST, v.id`
)
