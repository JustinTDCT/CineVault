package db

import (
	"context"
	"encoding/json"

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
		collectionID, collectionName, castJSON(m.Credits), m.Credits.Directors())
	return err
}

// storedCast is a cast member as kept in titles.cast_members.
type storedCast struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
}

// castJSON is the top of the bill as stored; "[]" when TMDb lists no one.
func castJSON(c tmdb.Credits) []byte {
	out := []storedCast{}
	for _, m := range c.TopCast() {
		out = append(out, storedCast{Name: m.Name, Character: m.Character, ProfilePath: m.ProfilePath})
	}
	b, _ := json.Marshal(out)
	return b
}

// TitlesNeedingCredits implements match.Store.
func (d *DB) TitlesNeedingCredits(ctx context.Context, libraryID int64) ([]match.Title, error) {
	rows, err := d.Pool.Query(ctx, sqlTitlesNeedingCredits, libraryID)
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

// SaveCredits implements match.Store.
func (d *DB) SaveCredits(ctx context.Context, titleID int64, c tmdb.Credits) error {
	_, err := d.Pool.Exec(ctx, sqlSaveCredits, titleID, castJSON(c), c.Directors())
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

// The DB is the music video matcher's store too.
var _ match.MusicVideoStore = (*DB)(nil)

// sqlNewRecording is true while saving a match whose recording differs from
// the one the title had (SET expressions see the row as it was).
const sqlNewRecording = `mb_recording_id IS DISTINCT FROM $2`

// MusicVideosToMatch implements match.MusicVideoStore.
func (d *DB) MusicVideosToMatch(ctx context.Context, libraryID int64, all bool) ([]match.MusicVideo, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, COALESCE(artist, ''), name FROM titles
		WHERE library_id = $1 AND media_type = 'musicvideo' AND ($2 OR match_status IN ('unmatched', 'error'))
		ORDER BY sort_name, id`, libraryID, all)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []match.MusicVideo{}
	for rows.Next() {
		var v match.MusicVideo
		if err := rows.Scan(&v.ID, &v.Artist, &v.Song); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SaveMusicVideoMatch implements match.MusicVideoStore. The title keeps the
// artist and song from its file name; MusicBrainz adds the recording, album
// and year. A new recording is a new cache key, so the cache state starts
// over and the next lookup asks about the new one.
func (d *DB) SaveMusicVideoMatch(ctx context.Context, titleID int64, m match.MusicVideoMatch) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE titles SET mb_recording_id = $2, album = NULLIF($3, ''), album_year = NULLIF($4::integer, 0),
		       match_status = 'matched', match_error = NULL, matched_at = now(), updated_at = now(),
		       cache_state      = CASE WHEN `+sqlNewRecording+` THEN 'none' ELSE cache_state END,
		       cache_version    = CASE WHEN `+sqlNewRecording+` THEN 0 ELSE cache_version END,
		       cache_checked_at = CASE WHEN `+sqlNewRecording+` THEN NULL ELSE cache_checked_at END,
		       cache_retry_at   = CASE WHEN `+sqlNewRecording+` THEN NULL ELSE cache_retry_at END,
		       cache_error      = CASE WHEN `+sqlNewRecording+` THEN NULL ELSE cache_error END,
		       cache_withdrawn  = CASE WHEN `+sqlNewRecording+` THEN false ELSE cache_withdrawn END,
		       cache_sources    = CASE WHEN `+sqlNewRecording+` THEN NULL ELSE cache_sources END,
		       cache_credits    = CASE WHEN `+sqlNewRecording+` THEN NULL ELSE cache_credits END,
		       contributed_hash = CASE WHEN `+sqlNewRecording+` THEN NULL ELSE contributed_hash END
		WHERE id = $1`, titleID, m.RecordingID, m.Album, m.Year)
	return err
}
