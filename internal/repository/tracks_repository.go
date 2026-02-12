package repository

import (
	"database/sql"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// TracksRepository handles CRUD for media_subtitles, media_audio_tracks, and media_chapters.
type TracksRepository struct {
	db *sql.DB
}

func NewTracksRepository(db *sql.DB) *TracksRepository {
	return &TracksRepository{db: db}
}

// ── Subtitles ──

func (r *TracksRepository) CreateSubtitle(s *models.MediaSubtitle) error {
	s.ID = uuid.New()
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	query := `INSERT INTO media_subtitles (id, media_item_id, language, title, format, file_path,
		stream_index, source, is_default, is_forced, is_sdh, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := r.db.Exec(query, s.ID, s.MediaItemID, s.Language, s.Title, s.Format, s.FilePath,
		s.StreamIndex, s.Source, s.IsDefault, s.IsForced, s.IsSDH, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *TracksRepository) GetSubtitlesByMediaID(mediaItemID uuid.UUID) ([]models.MediaSubtitle, error) {
	query := `SELECT id, media_item_id, language, title, format, file_path, stream_index,
		source, is_default, is_forced, is_sdh, created_at, updated_at
		FROM media_subtitles WHERE media_item_id = $1 ORDER BY is_default DESC, language`

	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []models.MediaSubtitle
	for rows.Next() {
		var s models.MediaSubtitle
		if err := rows.Scan(&s.ID, &s.MediaItemID, &s.Language, &s.Title, &s.Format, &s.FilePath,
			&s.StreamIndex, &s.Source, &s.IsDefault, &s.IsForced, &s.IsSDH,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (r *TracksRepository) GetSubtitleByID(id uuid.UUID) (*models.MediaSubtitle, error) {
	query := `SELECT id, media_item_id, language, title, format, file_path, stream_index,
		source, is_default, is_forced, is_sdh, created_at, updated_at
		FROM media_subtitles WHERE id = $1`

	var s models.MediaSubtitle
	err := r.db.QueryRow(query, id).Scan(&s.ID, &s.MediaItemID, &s.Language, &s.Title, &s.Format,
		&s.FilePath, &s.StreamIndex, &s.Source, &s.IsDefault, &s.IsForced, &s.IsSDH,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TracksRepository) DeleteSubtitlesByMediaID(mediaItemID uuid.UUID) error {
	_, err := r.db.Exec("DELETE FROM media_subtitles WHERE media_item_id = $1", mediaItemID)
	return err
}

// ── Audio Tracks ──

func (r *TracksRepository) CreateAudioTrack(t *models.MediaAudioTrack) error {
	t.ID = uuid.New()
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	query := `INSERT INTO media_audio_tracks (id, media_item_id, stream_index, language, title,
		codec, channels, channel_layout, bitrate, sample_rate, is_default, is_commentary,
		created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := r.db.Exec(query, t.ID, t.MediaItemID, t.StreamIndex, t.Language, t.Title,
		t.Codec, t.Channels, t.ChannelLayout, t.Bitrate, t.SampleRate,
		t.IsDefault, t.IsCommentary, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *TracksRepository) GetAudioTracksByMediaID(mediaItemID uuid.UUID) ([]models.MediaAudioTrack, error) {
	query := `SELECT id, media_item_id, stream_index, language, title, codec, channels,
		channel_layout, bitrate, sample_rate, is_default, is_commentary, created_at, updated_at
		FROM media_audio_tracks WHERE media_item_id = $1 ORDER BY is_default DESC, stream_index`

	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.MediaAudioTrack
	for rows.Next() {
		var t models.MediaAudioTrack
		if err := rows.Scan(&t.ID, &t.MediaItemID, &t.StreamIndex, &t.Language, &t.Title,
			&t.Codec, &t.Channels, &t.ChannelLayout, &t.Bitrate, &t.SampleRate,
			&t.IsDefault, &t.IsCommentary, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// GetAudioTrackByIndex returns a specific audio track by media ID and stream index.
func (r *TracksRepository) GetAudioTrackByIndex(mediaItemID uuid.UUID, streamIndex int) (*models.MediaAudioTrack, error) {
	query := `SELECT id, media_item_id, stream_index, language, title, codec, channels,
		channel_layout, bitrate, sample_rate, is_default, is_commentary, created_at, updated_at
		FROM media_audio_tracks WHERE media_item_id = $1 AND stream_index = $2`

	var t models.MediaAudioTrack
	err := r.db.QueryRow(query, mediaItemID, streamIndex).Scan(
		&t.ID, &t.MediaItemID, &t.StreamIndex, &t.Language, &t.Title,
		&t.Codec, &t.Channels, &t.ChannelLayout, &t.Bitrate, &t.SampleRate,
		&t.IsDefault, &t.IsCommentary, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetSubtitleByStreamIndex returns a subtitle track by media ID and stream index.
func (r *TracksRepository) GetSubtitleByStreamIndex(mediaItemID uuid.UUID, streamIndex int) (*models.MediaSubtitle, error) {
	query := `SELECT id, media_item_id, stream_index, language, title, format, source,
		file_path, is_default, is_forced, is_sdh, created_at, updated_at
		FROM media_subtitles WHERE media_item_id = $1 AND stream_index = $2`

	var s models.MediaSubtitle
	err := r.db.QueryRow(query, mediaItemID, streamIndex).Scan(
		&s.ID, &s.MediaItemID, &s.StreamIndex, &s.Language, &s.Title,
		&s.Format, &s.Source, &s.FilePath, &s.IsDefault, &s.IsForced,
		&s.IsSDH, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TracksRepository) DeleteAudioTracksByMediaID(mediaItemID uuid.UUID) error {
	_, err := r.db.Exec("DELETE FROM media_audio_tracks WHERE media_item_id = $1", mediaItemID)
	return err
}

// ── Chapters ──

func (r *TracksRepository) CreateChapter(c *models.MediaChapter) error {
	c.ID = uuid.New()
	c.CreatedAt = time.Now()

	query := `INSERT INTO media_chapters (id, media_item_id, title, start_seconds, end_seconds,
		sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.Exec(query, c.ID, c.MediaItemID, c.Title, c.StartSeconds, c.EndSeconds,
		c.SortOrder, c.CreatedAt)
	return err
}

func (r *TracksRepository) GetChaptersByMediaID(mediaItemID uuid.UUID) ([]models.MediaChapter, error) {
	query := `SELECT id, media_item_id, title, start_seconds, end_seconds, sort_order, created_at
		FROM media_chapters WHERE media_item_id = $1 ORDER BY sort_order, start_seconds`

	rows, err := r.db.Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []models.MediaChapter
	for rows.Next() {
		var c models.MediaChapter
		if err := rows.Scan(&c.ID, &c.MediaItemID, &c.Title, &c.StartSeconds, &c.EndSeconds,
			&c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

func (r *TracksRepository) DeleteChaptersByMediaID(mediaItemID uuid.UUID) error {
	_, err := r.db.Exec("DELETE FROM media_chapters WHERE media_item_id = $1", mediaItemID)
	return err
}
