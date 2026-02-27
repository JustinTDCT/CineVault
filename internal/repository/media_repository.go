package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// MediaFilter holds optional filter and sort parameters for media queries.
type MediaFilter struct {
	Genre         string // genre tag name
	Folder        string // folder path prefix
	ContentRating string // e.g. "PG-13"
	Edition       string // e.g. "Director's Cut"
	Source        string // e.g. "bluray", "web", "hdtv"
	DynamicRange  string // "SDR" or "HDR"
	Codec         string // e.g. "hevc", "h264", "av1"
	HDRFormat     string // e.g. "Dolby Vision", "HDR10", "HLG"
	Resolution    string // e.g. "4K", "1080p", "720p"
	AudioCodec    string // e.g. "truehd", "eac3", "aac"
	BitrateRange  string // "low" (<5Mbps), "medium" (5-15), "high" (15-30), "ultra" (30+)
	Country       string // e.g. "United States", "Canada"
	DurationRange string // "short" (<30min), "medium" (30-90), "long" (90-180), "vlong" (>180)
	WatchStatus   string // "watched", "unwatched"
	AddedDays     string // "1" (today), "7", "30", "90"
	YearFrom      string // e.g. "2000"
	YearTo        string // e.g. "2025"
	MinRating     string // e.g. "7"
	Sort          string // "title" (default), "year", "resolution", "duration", "rt_rating", "rating", "audience_score", "bitrate", "added_at"
	Order         string // "asc" (default), "desc"
}

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

// DB returns the underlying database connection for direct queries.
func (r *MediaRepository) DB() *sql.DB {
	return r.db
}

// mediaColumns is the standard SELECT list for media_items
const mediaColumns = `id, library_id, media_type, file_path, file_name, file_size,
	file_hash, title, sort_title, original_title, description, tagline, year, release_date,
	duration_seconds, rating, resolution, width, height, codec, container,
	bitrate, framerate, audio_codec, audio_channels, audio_format,
	original_language, country, trailer_url,
	poster_path, thumbnail_path, backdrop_path, logo_path,
	tv_show_id, tv_season_id, episode_number,
	artist_id, album_id, track_number, disc_number, album_artist, recording_mbid,
	author_id, book_id, chapter_number,
	image_gallery_id, sister_group_id, phash, audio_fingerprint,
	imdb_rating, rt_rating, audience_score,
	edition_type, content_rating, sort_position, external_ids, generated_poster,
	source_type, hdr_format, dynamic_range, keywords,
	metacritic_score, content_ratings_json, taglines_json, trailers_json, descriptions_json,
	custom_notes, custom_tags,
	metadata_locked, locked_fields, duplicate_status, preview_path, sprite_path,
	loudness_lufs, loudness_gain_db,
	parent_media_id, extra_type,
	play_count, last_played_at,
	added_at, updated_at`

// prefixedMediaColumns returns mediaColumns with each column prefixed by the given alias (e.g. "m.").
func prefixedMediaColumns(prefix string) string {
	cols := strings.Split(mediaColumns, ",")
	for i, c := range cols {
		cols[i] = prefix + strings.TrimSpace(c)
	}
	return strings.Join(cols, ", ")
}

func scanMediaItem(row interface{ Scan(dest ...interface{}) error }) (*models.MediaItem, error) {
	item := &models.MediaItem{}
	err := row.Scan(
		&item.ID, &item.LibraryID, &item.MediaType, &item.FilePath, &item.FileName,
		&item.FileSize, &item.FileHash, &item.Title, &item.SortTitle, &item.OriginalTitle,
		&item.Description, &item.Tagline, &item.Year, &item.ReleaseDate,
		&item.DurationSeconds, &item.Rating, &item.Resolution, &item.Width, &item.Height,
		&item.Codec, &item.Container, &item.Bitrate, &item.Framerate,
		&item.AudioCodec, &item.AudioChannels, &item.AudioFormat,
		&item.OriginalLanguage, &item.Country, &item.TrailerURL,
		&item.PosterPath, &item.ThumbnailPath, &item.BackdropPath, &item.LogoPath,
		&item.TVShowID, &item.TVSeasonID, &item.EpisodeNumber,
		&item.ArtistID, &item.AlbumID, &item.TrackNumber, &item.DiscNumber,
		&item.AlbumArtist, &item.RecordingMBID,
		&item.AuthorID, &item.BookID, &item.ChapterNumber,
		&item.ImageGalleryID, &item.SisterGroupID, &item.Phash, &item.AudioFingerprint,
		&item.IMDBRating, &item.RTRating, &item.AudienceScore,
		&item.EditionType, &item.ContentRating, &item.SortPosition, &item.ExternalIDs, &item.GeneratedPoster,
		&item.SourceType, &item.HDRFormat, &item.DynamicRange, &item.Keywords,
		&item.MetacriticScore, &item.ContentRatingsJSON, &item.TaglinesJSON, &item.TrailersJSON, &item.DescriptionsJSON,
		&item.CustomNotes, &item.CustomTags,
		&item.MetadataLocked, &item.LockedFields, &item.DuplicateStatus,
		&item.PreviewPath, &item.SpritePath,
		&item.LoudnessLUFS, &item.LoudnessGainDB,
		&item.ParentMediaID, &item.ExtraType,
		&item.PlayCount, &item.LastPlayedAt,
		&item.AddedAt, &item.UpdatedAt,
	)
	return item, err
}

func (r *MediaRepository) Create(item *models.MediaItem) error {
	query := `
		INSERT INTO media_items (
			id, library_id, media_type, file_path, file_name, file_size, file_hash,
			title, sort_title, original_title, description, year, release_date,
			duration_seconds, rating, resolution, width, height, codec, container,
			bitrate, framerate, audio_codec, audio_channels,
			poster_path, thumbnail_path, backdrop_path,
			tv_show_id, tv_season_id, episode_number,
			artist_id, album_id, track_number, disc_number, album_artist,
			author_id, book_id, chapter_number,
			image_gallery_id, sister_group_id, edition_type, sort_position,
			source_type, hdr_format, dynamic_range,
			parent_media_id, extra_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24,
			$25, $26, $27,
			$28, $29, $30,
			$31, $32, $33, $34, $35,
			$36, $37, $38,
			$39, $40, $41, $42,
			$43, $44, $45,
			$46, $47
		)
		RETURNING added_at, updated_at`

	return r.db.QueryRow(query,
		item.ID, item.LibraryID, item.MediaType, item.FilePath, item.FileName,
		item.FileSize, item.FileHash,
		item.Title, item.SortTitle, item.OriginalTitle, item.Description, item.Year, item.ReleaseDate,
		item.DurationSeconds, item.Rating, item.Resolution, item.Width, item.Height,
		item.Codec, item.Container, item.Bitrate, item.Framerate,
		item.AudioCodec, item.AudioChannels,
		item.PosterPath, item.ThumbnailPath, item.BackdropPath,
		item.TVShowID, item.TVSeasonID, item.EpisodeNumber,
		item.ArtistID, item.AlbumID, item.TrackNumber, item.DiscNumber, item.AlbumArtist,
		item.AuthorID, item.BookID, item.ChapterNumber,
		item.ImageGalleryID, item.SisterGroupID, item.EditionType, item.SortPosition,
		item.SourceType, item.HDRFormat, item.DynamicRange,
		item.ParentMediaID, item.ExtraType,
	).Scan(&item.AddedAt, &item.UpdatedAt)
}

func (r *MediaRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`DELETE FROM media_items WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("media item not found")
	}
	return nil
}

// BulkDelete removes multiple media items by ID.
func (r *MediaRepository) BulkDelete(ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM media_items WHERE id IN (%s)", strings.Join(placeholders, ", "))
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ExtendedMetadataUpdate holds optional fields for extended metadata updates.
// Only non-nil fields are written to the database.
type ExtendedMetadataUpdate struct {
	Tagline          *string
	OriginalLanguage *string
	Country          *string
	TrailerURL       *string
	LogoPath         *string
	OriginalTitle    *string
	SortTitle        *string
	ReleaseDate      *string
	BannerPath       *string
}

// FilterOptions holds the distinct filter values available for a library.
type FilterOptions struct {
	Genres         []string `json:"genres"`
	Folders        []string `json:"folders"`
	ContentRatings []string `json:"content_ratings"`
	Editions       []string `json:"editions"`
	Sources        []string `json:"sources"`
	DynamicRanges  []string `json:"dynamic_ranges"`
	Codecs         []string `json:"codecs"`
	HDRFormats     []string `json:"hdr_formats"`
	Resolutions    []string `json:"resolutions"`
	AudioCodecs    []string `json:"audio_codecs"`
	Countries      []string `json:"countries"`
}
