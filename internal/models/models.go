package models

import (
	"time"

	"github.com/google/uuid"
)

// ──────────────────── Enums ────────────────────

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
	RoleGuest UserRole = "guest"
)

type MediaType string

const (
	MediaTypeMovies      MediaType = "movies"
	MediaTypeAdultMovies MediaType = "adult_movies"
	MediaTypeTVShows     MediaType = "tv_shows"
	MediaTypeMusic       MediaType = "music"
	MediaTypeMusicVideos MediaType = "music_videos"
	MediaTypeHomeVideos  MediaType = "home_videos"
	MediaTypeOtherVideos MediaType = "other_videos"
	MediaTypeImages      MediaType = "images"
	MediaTypeAudiobooks  MediaType = "audiobooks"
)

type DuplicateAction string

const (
	DuplicateMerged        DuplicateAction = "merged"
	DuplicateDeleted       DuplicateAction = "deleted"
	DuplicateIgnored       DuplicateAction = "ignored"
	DuplicateSplitAsSister DuplicateAction = "split_as_sister"
	DuplicateEditionGrouped DuplicateAction = "edition_grouped"
)

// ──────────────────── User ────────────────────

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         UserRole  `json:"role" db:"role"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Library ────────────────────

type Library struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Name          string     `json:"name" db:"name"`
	MediaType     MediaType  `json:"media_type" db:"media_type"`
	Path          string     `json:"path" db:"path"`
	IsEnabled     bool       `json:"is_enabled" db:"is_enabled"`
	ScanOnStartup bool       `json:"scan_on_startup" db:"scan_on_startup"`
	LastScanAt    *time.Time `json:"last_scan_at" db:"last_scan_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// ──────────────────── MediaItem ────────────────────

type MediaItem struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	LibraryID        uuid.UUID  `json:"library_id" db:"library_id"`
	MediaType        MediaType  `json:"media_type" db:"media_type"`
	FilePath         string     `json:"file_path" db:"file_path"`
	FileName         string     `json:"file_name" db:"file_name"`
	FileSize         int64      `json:"file_size" db:"file_size"`
	FileHash         *string    `json:"file_hash,omitempty" db:"file_hash"`
	Title            string     `json:"title" db:"title"`
	SortTitle        *string    `json:"sort_title,omitempty" db:"sort_title"`
	OriginalTitle    *string    `json:"original_title,omitempty" db:"original_title"`
	Description      *string    `json:"description,omitempty" db:"description"`
	Year             *int       `json:"year,omitempty" db:"year"`
	ReleaseDate      *time.Time `json:"release_date,omitempty" db:"release_date"`
	DurationSeconds  *int       `json:"duration_seconds,omitempty" db:"duration_seconds"`
	Rating           *float64   `json:"rating,omitempty" db:"rating"`
	Resolution       *string    `json:"resolution,omitempty" db:"resolution"`
	Width            *int       `json:"width,omitempty" db:"width"`
	Height           *int       `json:"height,omitempty" db:"height"`
	Codec            *string    `json:"codec,omitempty" db:"codec"`
	Container        *string    `json:"container,omitempty" db:"container"`
	Bitrate          *int64     `json:"bitrate,omitempty" db:"bitrate"`
	Framerate        *float64   `json:"framerate,omitempty" db:"framerate"`
	AudioCodec       *string    `json:"audio_codec,omitempty" db:"audio_codec"`
	AudioChannels    *int       `json:"audio_channels,omitempty" db:"audio_channels"`
	PosterPath       *string    `json:"poster_path,omitempty" db:"poster_path"`
	ThumbnailPath    *string    `json:"thumbnail_path,omitempty" db:"thumbnail_path"`
	BackdropPath     *string    `json:"backdrop_path,omitempty" db:"backdrop_path"`
	// TV fields
	TVShowID      *uuid.UUID `json:"tv_show_id,omitempty" db:"tv_show_id"`
	TVSeasonID    *uuid.UUID `json:"tv_season_id,omitempty" db:"tv_season_id"`
	EpisodeNumber *int       `json:"episode_number,omitempty" db:"episode_number"`
	// Music fields
	ArtistID    *uuid.UUID `json:"artist_id,omitempty" db:"artist_id"`
	AlbumID     *uuid.UUID `json:"album_id,omitempty" db:"album_id"`
	TrackNumber *int       `json:"track_number,omitempty" db:"track_number"`
	DiscNumber  *int       `json:"disc_number,omitempty" db:"disc_number"`
	// Audiobook fields
	AuthorID      *uuid.UUID `json:"author_id,omitempty" db:"author_id"`
	BookID        *uuid.UUID `json:"book_id,omitempty" db:"book_id"`
	ChapterNumber *int       `json:"chapter_number,omitempty" db:"chapter_number"`
	// Image fields
	ImageGalleryID *uuid.UUID `json:"image_gallery_id,omitempty" db:"image_gallery_id"`
	// Grouping fields
	SisterGroupID    *uuid.UUID `json:"sister_group_id,omitempty" db:"sister_group_id"`
	Phash            *string    `json:"phash,omitempty" db:"phash"`
	AudioFingerprint *string    `json:"audio_fingerprint,omitempty" db:"audio_fingerprint"`
	SortPosition     int        `json:"sort_position" db:"sort_position"`
	AddedAt          time.Time  `json:"added_at" db:"added_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
	LastScannedAt    *time.Time `json:"last_scanned_at,omitempty" db:"last_scanned_at"`
}

// ──────────────────── TV ────────────────────

type TVShow struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	LibraryID     uuid.UUID  `json:"library_id" db:"library_id"`
	Title         string     `json:"title" db:"title"`
	SortTitle     *string    `json:"sort_title,omitempty" db:"sort_title"`
	OriginalTitle *string    `json:"original_title,omitempty" db:"original_title"`
	Description   *string    `json:"description,omitempty" db:"description"`
	Year          *int       `json:"year,omitempty" db:"year"`
	FirstAirDate  *time.Time `json:"first_air_date,omitempty" db:"first_air_date"`
	LastAirDate   *time.Time `json:"last_air_date,omitempty" db:"last_air_date"`
	Status        *string    `json:"status,omitempty" db:"status"`
	PosterPath    *string    `json:"poster_path,omitempty" db:"poster_path"`
	BackdropPath  *string    `json:"backdrop_path,omitempty" db:"backdrop_path"`
	SortPosition  int        `json:"sort_position" db:"sort_position"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated fields (not stored in DB)
	SeasonCount  int `json:"season_count,omitempty" db:"-"`
	EpisodeCount int `json:"episode_count,omitempty" db:"-"`
}

type TVSeason struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	TVShowID     uuid.UUID  `json:"tv_show_id" db:"tv_show_id"`
	SeasonNumber int        `json:"season_number" db:"season_number"`
	Title        *string    `json:"title,omitempty" db:"title"`
	Description  *string    `json:"description,omitempty" db:"description"`
	AirDate      *time.Time `json:"air_date,omitempty" db:"air_date"`
	EpisodeCount int        `json:"episode_count" db:"episode_count"`
	PosterPath   *string    `json:"poster_path,omitempty" db:"poster_path"`
	SortPosition int        `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Music ────────────────────

type Artist struct {
	ID           uuid.UUID `json:"id" db:"id"`
	LibraryID    uuid.UUID `json:"library_id" db:"library_id"`
	Name         string    `json:"name" db:"name"`
	SortName     *string   `json:"sort_name,omitempty" db:"sort_name"`
	Description  *string   `json:"description,omitempty" db:"description"`
	PosterPath   *string   `json:"poster_path,omitempty" db:"poster_path"`
	SortPosition int       `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	// Aggregated
	AlbumCount int `json:"album_count,omitempty" db:"-"`
	TrackCount int `json:"track_count,omitempty" db:"-"`
}

type Album struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	ArtistID     uuid.UUID  `json:"artist_id" db:"artist_id"`
	LibraryID    uuid.UUID  `json:"library_id" db:"library_id"`
	Title        string     `json:"title" db:"title"`
	SortTitle    *string    `json:"sort_title,omitempty" db:"sort_title"`
	Year         *int       `json:"year,omitempty" db:"year"`
	ReleaseDate  *time.Time `json:"release_date,omitempty" db:"release_date"`
	Description  *string    `json:"description,omitempty" db:"description"`
	Genre        *string    `json:"genre,omitempty" db:"genre"`
	PosterPath   *string    `json:"poster_path,omitempty" db:"poster_path"`
	SortPosition int        `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated
	TrackCount int `json:"track_count,omitempty" db:"-"`
}

// ──────────────────── Audiobooks ────────────────────

type Author struct {
	ID           uuid.UUID `json:"id" db:"id"`
	LibraryID    uuid.UUID `json:"library_id" db:"library_id"`
	Name         string    `json:"name" db:"name"`
	SortName     *string   `json:"sort_name,omitempty" db:"sort_name"`
	Description  *string   `json:"description,omitempty" db:"description"`
	PosterPath   *string   `json:"poster_path,omitempty" db:"poster_path"`
	SortPosition int       `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	// Aggregated
	BookCount int `json:"book_count,omitempty" db:"-"`
}

type BookSeries struct {
	ID           uuid.UUID `json:"id" db:"id"`
	AuthorID     uuid.UUID `json:"author_id" db:"author_id"`
	Title        string    `json:"title" db:"title"`
	SortTitle    *string   `json:"sort_title,omitempty" db:"sort_title"`
	Description  *string   `json:"description,omitempty" db:"description"`
	SortPosition int       `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Book struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	AuthorID     uuid.UUID  `json:"author_id" db:"author_id"`
	SeriesID     *uuid.UUID `json:"series_id,omitempty" db:"series_id"`
	LibraryID    uuid.UUID  `json:"library_id" db:"library_id"`
	Title        string     `json:"title" db:"title"`
	SortTitle    *string    `json:"sort_title,omitempty" db:"sort_title"`
	Year         *int       `json:"year,omitempty" db:"year"`
	Description  *string    `json:"description,omitempty" db:"description"`
	Narrator     *string    `json:"narrator,omitempty" db:"narrator"`
	PosterPath   *string    `json:"poster_path,omitempty" db:"poster_path"`
	SortPosition int        `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated
	ChapterCount int `json:"chapter_count,omitempty" db:"-"`
}

// ──────────────────── Images ────────────────────

type ImageGallery struct {
	ID           uuid.UUID `json:"id" db:"id"`
	LibraryID    uuid.UUID `json:"library_id" db:"library_id"`
	Title        string    `json:"title" db:"title"`
	Description  *string   `json:"description,omitempty" db:"description"`
	PosterPath   *string   `json:"poster_path,omitempty" db:"poster_path"`
	SortPosition int       `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	// Aggregated
	ImageCount int `json:"image_count,omitempty" db:"-"`
}

// ──────────────────── Edition Groups ────────────────────

type EditionGroup struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	LibraryID        uuid.UUID  `json:"library_id" db:"library_id"`
	MediaType        MediaType  `json:"media_type" db:"media_type"`
	Title            string     `json:"title" db:"title"`
	SortTitle        *string    `json:"sort_title,omitempty" db:"sort_title"`
	Year             *int       `json:"year,omitempty" db:"year"`
	Description      *string    `json:"description,omitempty" db:"description"`
	PosterPath       *string    `json:"poster_path,omitempty" db:"poster_path"`
	BackdropPath     *string    `json:"backdrop_path,omitempty" db:"backdrop_path"`
	DefaultEditionID *uuid.UUID `json:"default_edition_id,omitempty" db:"default_edition_id"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated
	Items []EditionItem `json:"items,omitempty" db:"-"`
}

type EditionItem struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	EditionGroupID    uuid.UUID  `json:"edition_group_id" db:"edition_group_id"`
	MediaItemID       uuid.UUID  `json:"media_item_id" db:"media_item_id"`
	EditionType       string     `json:"edition_type" db:"edition_type"`
	CustomEditionName *string    `json:"custom_edition_name,omitempty" db:"custom_edition_name"`
	QualityTier       *string    `json:"quality_tier,omitempty" db:"quality_tier"`
	DisplayName       *string    `json:"display_name,omitempty" db:"display_name"`
	IsDefault         bool       `json:"is_default" db:"is_default"`
	SortOrder         int        `json:"sort_order" db:"sort_order"`
	Notes             *string    `json:"notes,omitempty" db:"notes"`
	AddedAt           time.Time  `json:"added_at" db:"added_at"`
	AddedBy           *uuid.UUID `json:"added_by,omitempty" db:"added_by"`
	// Joined
	MediaItem *MediaItem `json:"media_item,omitempty" db:"-"`
}

// ──────────────────── Sister Groups ────────────────────

type SisterGroup struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	Name      string     `json:"name" db:"name"`
	Notes     *string    `json:"notes,omitempty" db:"notes"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
	// Aggregated
	Members []*MediaItem `json:"members,omitempty" db:"-"`
}

// ──────────────────── Collections ────────────────────

type Collection struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	UserID         uuid.UUID  `json:"user_id" db:"user_id"`
	LibraryID      *uuid.UUID `json:"library_id,omitempty" db:"library_id"`
	Name           string     `json:"name" db:"name"`
	Description    *string    `json:"description,omitempty" db:"description"`
	PosterPath     *string    `json:"poster_path,omitempty" db:"poster_path"`
	CollectionType string     `json:"collection_type" db:"collection_type"`
	Visibility     string     `json:"visibility" db:"visibility"`
	ItemSortMode   string     `json:"item_sort_mode" db:"item_sort_mode"`
	SortPosition   int        `json:"sort_position" db:"sort_position"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated
	ItemCount int              `json:"item_count,omitempty" db:"-"`
	Items     []CollectionItem `json:"items,omitempty" db:"-"`
}

type CollectionItem struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	CollectionID   uuid.UUID  `json:"collection_id" db:"collection_id"`
	MediaItemID    *uuid.UUID `json:"media_item_id,omitempty" db:"media_item_id"`
	EditionGroupID *uuid.UUID `json:"edition_group_id,omitempty" db:"edition_group_id"`
	TVShowID       *uuid.UUID `json:"tv_show_id,omitempty" db:"tv_show_id"`
	AlbumID        *uuid.UUID `json:"album_id,omitempty" db:"album_id"`
	BookID         *uuid.UUID `json:"book_id,omitempty" db:"book_id"`
	SortPosition   int        `json:"sort_position" db:"sort_position"`
	Notes          *string    `json:"notes,omitempty" db:"notes"`
	AddedAt        time.Time  `json:"added_at" db:"added_at"`
	AddedBy        *uuid.UUID `json:"added_by,omitempty" db:"added_by"`
}

// ──────────────────── Watch History ────────────────────

type WatchHistory struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	UserID          uuid.UUID  `json:"user_id" db:"user_id"`
	MediaItemID     uuid.UUID  `json:"media_item_id" db:"media_item_id"`
	EditionGroupID  *uuid.UUID `json:"edition_group_id,omitempty" db:"edition_group_id"`
	ProgressSeconds int        `json:"progress_seconds" db:"progress_seconds"`
	DurationSeconds *int       `json:"duration_seconds,omitempty" db:"duration_seconds"`
	Completed       bool       `json:"completed" db:"completed"`
	LastWatchedAt   time.Time  `json:"last_watched_at" db:"last_watched_at"`
	// Joined
	MediaItem *MediaItem `json:"media_item,omitempty" db:"-"`
}

// ──────────────────── Duplicate Decisions ────────────────────

type DuplicateDecision struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	MediaIDA        *uuid.UUID      `json:"media_id_a,omitempty" db:"media_id_a"`
	MediaIDB        *uuid.UUID      `json:"media_id_b,omitempty" db:"media_id_b"`
	Action          DuplicateAction `json:"action" db:"action"`
	PrimaryMediaID  *uuid.UUID      `json:"primary_media_id,omitempty" db:"primary_media_id"`
	DecidedBy       *uuid.UUID      `json:"decided_by,omitempty" db:"decided_by"`
	DecidedAt       time.Time       `json:"decided_at" db:"decided_at"`
	Notes           *string         `json:"notes,omitempty" db:"notes"`
	SimilarityScore *float64        `json:"similarity_score,omitempty" db:"similarity_score"`
}

// ──────────────────── Scan Result ────────────────────

type ScanResult struct {
	FilesFound   int      `json:"files_found"`
	FilesAdded   int      `json:"files_added"`
	FilesSkipped int      `json:"files_skipped"`
	FilesUpdated int      `json:"files_updated"`
	Errors       []string `json:"errors,omitempty"`
}
