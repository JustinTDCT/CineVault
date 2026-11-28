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

	// Titles are found by the name read from files, not the (possibly renamed)
	// display name. Year 0 means unknown.
	sqlFindTitle = `
		SELECT id FROM titles
		WHERE library_id = $1
		  AND lower(file_title) = lower($2)
		  AND file_year IS NOT DISTINCT FROM NULLIF($3::integer, 0)
		  AND lower(COALESCE(artist, '')) = lower($4)
		ORDER BY id
		LIMIT 1`

	// $5 year and $6 tmdb_id: 0 means unknown.
	sqlInsertTitle = `
		INSERT INTO titles (library_id, media_type, name, file_title, sort_name, year, file_year, tmdb_id, artist)
		VALUES ($1, $2, $3, $3, $4, NULLIF($5::integer, 0), NULLIF($5::integer, 0), NULLIF($6::integer, 0), NULLIF($7, ''))
		RETURNING id`

	// Keeps an existing runtime; fills it in if it was empty.
	sqlUpsertEdition = `
		INSERT INTO editions (title_id, edition_type_id, runtime_seconds)
		VALUES ($1, $2, NULLIF($3::integer, 0))
		ON CONFLICT (title_id, edition_type_id) DO UPDATE
		   SET runtime_seconds = COALESCE(editions.runtime_seconds, EXCLUDED.runtime_seconds)
		RETURNING id`

	// Creates the season row too. A name from TMDb is kept over one from a file.
	sqlFindOrCreateEpisode = `
		WITH season AS (
		    INSERT INTO seasons (title_id, season_number) VALUES ($1, $2)
		    ON CONFLICT (title_id, season_number) DO NOTHING
		)
		INSERT INTO episodes (title_id, season_number, episode_number, name)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		ON CONFLICT (title_id, season_number, episode_number) DO UPDATE
		   SET name = COALESCE(episodes.name, EXCLUDED.name)
		RETURNING id`

	sqlFindOrCreateTrack = `
		INSERT INTO tracks (title_id, disc_number, track_number, name, artist)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT (title_id, disc_number, track_number, lower(name)) DO UPDATE
		   SET artist = COALESCE(EXCLUDED.artist, tracks.artist)
		RETURNING id`

	sqlSetBookInfo = `
		UPDATE titles SET
		    series       = COALESCE(series, NULLIF($2, '')),
		    series_index = COALESCE(series_index, NULLIF($3::numeric, 0)),
		    narrator     = COALESCE(narrator, NULLIF($4, '')),
		    overview     = COALESCE(overview, NULLIF($5, ''))
		WHERE id = $1`

	sqlDeleteChapters = `DELETE FROM chapters WHERE version_id = (SELECT id FROM versions WHERE path = $1)`

	sqlInsertChapter = `
		INSERT INTO chapters (version_id, idx, start_seconds, end_seconds, name)
		SELECT id, $2, $3, $4, $5 FROM versions WHERE path = $1`

	sqlVersionChapters = `
		SELECT version_id, start_seconds::float8, end_seconds::float8, name
		FROM chapters WHERE version_id = ANY($1) ORDER BY version_id, idx`

	sqlGetProgress = `
		SELECT version_id, position_seconds::float8, COALESCE(locator, ''), updated_at FROM playback_progress WHERE title_id = $1`

	// Saved only when the file belongs to the title.
	sqlSaveProgress = `
		INSERT INTO playback_progress (title_id, version_id, position_seconds, locator, updated_at)
		SELECT $1, $2, $3, NULLIF($4, ''), now()
		WHERE EXISTS (SELECT 1 FROM versions v JOIN editions e ON e.id = v.edition_id
		              WHERE v.id = $2 AND e.title_id = $1)
		ON CONFLICT (title_id) DO UPDATE
		   SET version_id = EXCLUDED.version_id, position_seconds = EXCLUDED.position_seconds,
		       locator = EXCLUDED.locator, updated_at = now()
		RETURNING updated_at`

	sqlSetGenres = `UPDATE titles SET genres = $2 WHERE id = $1 AND cardinality(genres) = 0`

	// An album's tracks with their files, per edition; disc and track order.
	sqlAlbumTracks = `
		SELECT v.edition_id, tr.id, tr.disc_number, tr.track_number, tr.name, COALESCE(tr.artist, ''),
		       v.id, v.path, v.size_bytes, COALESCE(v.label, ''), COALESCE(v.container, ''),
		       COALESCE(v.audio_codec, ''), COALESCE(v.audio_channels, 0),
		       COALESCE(v.duration_seconds, 0)::float8, COALESCE(v.bitrate, 0),
		       v.missing_since IS NOT NULL, COALESCE(v.page_count, 0)
		FROM tracks tr
		JOIN versions v ON v.track_id = tr.id
		WHERE tr.title_id = $1
		ORDER BY v.edition_id, tr.disc_number, tr.track_number = 0, tr.track_number, tr.name, v.id`

	// A version whose edition the user locked keeps its edition.
	sqlUpsertVersion = `
		INSERT INTO versions (edition_id, path, size_bytes, fingerprint, label, container,
		                      video_codec, width, height, hdr, audio_codec, audio_channels,
		                      duration_seconds, bitrate, episode_id, last_episode, track_id, page_count)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''),
		        NULLIF($7, ''), NULLIF($8::integer, 0), NULLIF($9::integer, 0), NULLIF($10, ''),
		        NULLIF($11, ''), NULLIF($12::integer, 0),
		        NULLIF($13::float8, 0)::numeric(10,3), NULLIF($14::bigint, 0),
		        NULLIF($15::bigint, 0), NULLIF($16::integer, 0), NULLIF($17::bigint, 0), NULLIF($18::integer, 0))
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
		    episode_id       = EXCLUDED.episode_id,
		    last_episode     = EXCLUDED.last_episode,
		    track_id         = EXCLUDED.track_id,
		    page_count       = EXCLUDED.page_count,
		    missing_since    = NULL`

	sqlSetMissing = `
		UPDATE versions SET missing_since = now()
		WHERE id = ANY($1::bigint[]) AND missing_since IS NULL`

	sqlSetPresent = `
		UPDATE versions SET missing_since = NULL
		WHERE id = ANY($1::bigint[]) AND missing_since IS NOT NULL`

	sqlVersion = `
		SELECT v.id, v.path, v.size_bytes, COALESCE(v.label, ''), COALESCE(v.container, ''),
		       COALESCE(v.video_codec, ''), COALESCE(v.width, 0), COALESCE(v.height, 0),
		       COALESCE(v.hdr, ''), COALESCE(v.audio_codec, ''), COALESCE(v.audio_channels, 0),
		       COALESCE(v.duration_seconds, 0)::float8, COALESCE(v.bitrate, 0),
		       v.edition_locked, v.missing_since IS NOT NULL, COALESCE(v.last_episode, 0), COALESCE(v.page_count, 0)
		FROM versions v
		WHERE v.id = $1`

	sqlTitlesToMatch = `
		SELECT id, media_type, file_title, COALESCE(file_year, 0), COALESCE(tmdb_id, 0)
		FROM titles
		WHERE library_id = $1 AND match_status IN ('unmatched', 'error')
		ORDER BY sort_name, id`

	// Fields named in locked_fields (edited by hand) are never overwritten.
	// The year stays as read from files, so later scans still find the title;
	// TMDb's full date is kept in release_date.
	sqlSaveMatch = `
		UPDATE titles SET
		    tmdb_id            = $2,
		    imdb_id            = NULLIF($3, ''),
		    name               = CASE WHEN 'name' = ANY(locked_fields) THEN name ELSE $4 END,
		    sort_name          = CASE WHEN 'name' = ANY(locked_fields) THEN sort_name ELSE $5 END,
		    original_title     = NULLIF($6, ''),
		    overview           = CASE WHEN 'overview' = ANY(locked_fields) THEN overview ELSE NULLIF($7, '') END,
		    tagline            = NULLIF($8, ''),
		    release_date       = NULLIF($9::text, '')::date,
		    year               = COALESCE(year, NULLIF($10::integer, 0)),
		    runtime_minutes    = NULLIF($11::integer, 0),
		    poster_path        = CASE WHEN 'poster' = ANY(locked_fields) THEN poster_path ELSE NULLIF($12, '') END,
		    backdrop_path      = NULLIF($13, ''),
		    genres             = $14::text[],
		    rating             = NULLIF($15::float8, 0)::numeric(3, 1),
		    collection_tmdb_id = NULLIF($16::integer, 0),
		    collection_name    = NULLIF($17, ''),
		    cast_members       = $18::jsonb,
		    directors          = $19::text[],
		    match_status       = 'matched',
		    match_error        = NULL,
		    matched_at         = now(),
		    updated_at         = now()
		WHERE id = $1`

	// Matched shows with episodes that have no TMDb details yet.
	sqlShowsNeedingEpisodes = `
		SELECT t.id, t.name, t.tmdb_id, array_agg(DISTINCT e.season_number ORDER BY e.season_number)
		FROM titles t
		JOIN episodes e ON e.title_id = t.id
		WHERE t.library_id = $1 AND t.media_type = 'tv' AND t.match_status = 'matched'
		  AND t.tmdb_id IS NOT NULL AND e.tmdb_id IS NULL
		GROUP BY t.id, t.name, t.tmdb_id
		ORDER BY t.id`

	sqlSaveSeason = `
		INSERT INTO seasons (title_id, season_number, name, overview, poster_path, air_date)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6::text, '')::date)
		ON CONFLICT (title_id, season_number) DO UPDATE SET
		    name = EXCLUDED.name, overview = EXCLUDED.overview,
		    poster_path = EXCLUDED.poster_path, air_date = EXCLUDED.air_date`

	sqlSaveEpisode = `
		INSERT INTO episodes (title_id, season_number, episode_number, name, overview, air_date,
		                      runtime_minutes, still_path, rating, tmdb_id)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6::text, '')::date,
		        NULLIF($7::integer, 0), NULLIF($8, ''), NULLIF($9::float8, 0)::numeric(3, 1), $10)
		ON CONFLICT (title_id, season_number, episode_number) DO UPDATE SET
		    name = COALESCE(EXCLUDED.name, episodes.name), overview = EXCLUDED.overview,
		    air_date = EXCLUDED.air_date, runtime_minutes = EXCLUDED.runtime_minutes,
		    still_path = EXCLUDED.still_path, rating = EXCLUDED.rating, tmdb_id = EXCLUDED.tmdb_id`

	// One row per episode file (or per episode with no file); specials last.
	sqlTitleEpisodes = `
		SELECT s.season_number, COALESCE(s.name, ''), COALESCE(s.overview, ''),
		       COALESCE(s.poster_path, ''), COALESCE(to_char(s.air_date, 'YYYY-MM-DD'), ''),
		       ep.id, ep.episode_number, COALESCE(ep.name, ''), COALESCE(ep.overview, ''),
		       COALESCE(to_char(ep.air_date, 'YYYY-MM-DD'), ''), COALESCE(ep.runtime_minutes, 0),
		       COALESCE(ep.still_path, ''), COALESCE(ep.rating, 0)::float8,
		       COALESCE(v.id, 0), COALESCE(v.path, ''), COALESCE(v.size_bytes, 0),
		       COALESCE(v.label, ''), COALESCE(v.container, ''), COALESCE(v.video_codec, ''),
		       COALESCE(v.width, 0), COALESCE(v.height, 0), COALESCE(v.hdr, ''),
		       COALESCE(v.audio_codec, ''), COALESCE(v.audio_channels, 0),
		       COALESCE(v.duration_seconds, 0)::float8, COALESCE(v.bitrate, 0),
		       COALESCE(v.edition_locked, false), v.missing_since IS NOT NULL,
		       COALESCE(v.last_episode, 0)
		FROM seasons s
		JOIN episodes ep ON ep.title_id = s.title_id AND ep.season_number = s.season_number
		LEFT JOIN versions v ON v.episode_id = ep.id
		WHERE s.title_id = $1
		ORDER BY s.season_number = 0, s.season_number, ep.episode_number,
		         v.height DESC NULLS LAST, v.id`

	sqlTitleExists = `SELECT EXISTS (SELECT 1 FROM titles WHERE id = $1)`

	// Matched movies and shows whose cast hasn't been fetched yet.
	sqlTitlesNeedingCredits = `
		SELECT id, media_type, name, COALESCE(year, 0), tmdb_id
		FROM titles
		WHERE library_id = $1 AND match_status = 'matched' AND tmdb_id IS NOT NULL
		  AND media_type IN ('movie', 'tv') AND cast_members IS NULL
		ORDER BY sort_name, id`

	sqlSaveCredits = `UPDATE titles SET cast_members = $2::jsonb, directors = $3::text[], updated_at = now() WHERE id = $1`

	sqlSaveMatchFailure = `
		UPDATE titles SET match_status = $2, match_error = $3, updated_at = now()
		WHERE id = $1`

	// One row per version (or per edition with no versions), in display order.
	sqlLibraryTitles = `
		SELECT t.id, t.library_id, t.media_type, t.name, t.sort_name,
		       COALESCE(t.year, 0), COALESCE(t.tmdb_id, 0),
		       COALESCE(t.original_title, ''), COALESCE(t.overview, ''), COALESCE(t.tagline, ''),
		       COALESCE(to_char(t.release_date, 'YYYY-MM-DD'), ''), COALESCE(t.runtime_minutes, 0),
		       COALESCE(t.poster_path, ''), COALESCE(t.backdrop_path, ''), COALESCE(t.imdb_id, ''),
		       t.genres, COALESCE(t.rating, 0)::float8,
		       COALESCE(t.collection_tmdb_id, 0), COALESCE(t.collection_name, ''),
		       t.match_status, COALESCE(t.match_error, ''), COALESCE(t.artist, ''),
		       COALESCE(t.series, ''), COALESCE(t.series_index, 0)::float8, COALESCE(t.narrator, ''),
		       COALESCE(CASE WHEN t.media_type IN ('music', 'audiobook', 'book', 'comic') THEN
		         (SELECT min(x.id) FROM versions x JOIN editions xe ON xe.id = x.edition_id
		           WHERE xe.title_id = t.id AND x.missing_since IS NULL
		             AND (x.track_id IS NOT NULL OR t.media_type = 'book'))
		       END, 0),
		       (SELECT count(*) FROM tracks tr WHERE tr.title_id = t.id
		          AND EXISTS (SELECT 1 FROM versions x WHERE x.track_id = tr.id)),
		       (SELECT count(DISTINCT ep.season_number) FROM episodes ep
		         WHERE ep.title_id = t.id AND EXISTS (SELECT 1 FROM versions x WHERE x.episode_id = ep.id)),
		       (SELECT count(*) FROM episodes ep
		         WHERE ep.title_id = t.id AND EXISTS (SELECT 1 FROM versions x WHERE x.episode_id = ep.id)),
		       t.cache_state, t.cache_withdrawn, COALESCE(t.cache_sources, '[]'::jsonb),
		       e.id, e.edition_type_id, et.display_name, et.short_label,
		       COALESCE(e.runtime_seconds, 0),
		       e.from_cache, COALESCE(e.cache_status, ''), COALESCE(e.cache_display_name, ''),
		       COALESCE(e.cache_runtime_min, 0), COALESCE(e.cache_runtime_sources, '{}'),
		       COALESCE(e.cache_difference_short, ''), COALESCE(e.cache_difference_full, '[]'::jsonb),
		       COALESCE(e.cache_confidence, ''),
		       COALESCE(v.id, 0), COALESCE(v.path, ''), COALESCE(v.size_bytes, 0),
		       COALESCE(v.label, ''), COALESCE(v.container, ''), COALESCE(v.video_codec, ''),
		       COALESCE(v.width, 0), COALESCE(v.height, 0), COALESCE(v.hdr, ''),
		       COALESCE(v.audio_codec, ''), COALESCE(v.audio_channels, 0),
		       COALESCE(v.duration_seconds, 0)::float8, COALESCE(v.bitrate, 0),
		       COALESCE(v.edition_locked, false), v.missing_since IS NOT NULL,
		       COALESCE(t.cast_members, '[]'::jsonb), COALESCE(t.directors, '{}'),
		       COALESCE((SELECT max(x.added_at) FROM versions x JOIN editions xe ON xe.id = x.edition_id
		                  WHERE xe.title_id = t.id AND x.missing_since IS NULL), t.created_at),
		       COALESCE(pp.version_id, 0), COALESCE(pp.position_seconds, 0)::float8, COALESCE(pp.locator, ''),
		       COALESCE(pp.updated_at, 'epoch'), COALESCE(pv.duration_seconds, 0)::float8, COALESCE(pv.page_count, 0),
		       COALESCE(pe.season_number, 0), COALESCE(pe.episode_number, 0), COALESCE(pe.name, ''),
		       COALESCE(t.mb_recording_id, ''), COALESCE(t.album, ''), COALESCE(t.album_year, 0), t.cache_credits
		FROM titles t
		JOIN editions e ON e.title_id = t.id
		JOIN edition_types et ON et.id = e.edition_type_id
		LEFT JOIN versions v ON v.edition_id = e.id AND v.episode_id IS NULL AND v.track_id IS NULL
		LEFT JOIN playback_progress pp ON pp.title_id = t.id
		LEFT JOIN versions pv ON pv.id = pp.version_id
		LEFT JOIN episodes pe ON pe.id = pv.episode_id
		WHERE t.library_id = $1
		ORDER BY t.sort_name, t.year NULLS LAST, t.id, et.sort_order, e.id,
		         v.height DESC NULLS LAST, v.id`
)
