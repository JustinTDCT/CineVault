package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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

type LibraryAccess string

const (
	LibraryAccessEveryone    LibraryAccess = "everyone"
	LibraryAccessSelectUsers LibraryAccess = "select_users"
	LibraryAccessAdminOnly   LibraryAccess = "admin_only"
)

type DuplicateAction string

const (
	DuplicateEdit    DuplicateAction = "edit"
	DuplicateEdition DuplicateAction = "edition"
	DuplicateDeleted DuplicateAction = "deleted"
	DuplicateIgnored DuplicateAction = "ignored"
)

// ──────────────────── User ────────────────────

type User struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	Username         string     `json:"username" db:"username"`
	Email            string     `json:"email" db:"email"`
	PasswordHash     string     `json:"-" db:"password_hash"`
	PinHash          *string    `json:"-" db:"pin_hash"`
	DisplayName      *string    `json:"display_name,omitempty" db:"display_name"`
	FirstName        *string    `json:"first_name,omitempty" db:"first_name"`
	LastName         *string    `json:"last_name,omitempty" db:"last_name"`
	Role             UserRole   `json:"role" db:"role"`
	IsActive         bool       `json:"is_active" db:"is_active"`
	MaxContentRating *string    `json:"max_content_rating,omitempty" db:"max_content_rating"`
	IsKidsProfile    bool       `json:"is_kids_profile" db:"is_kids_profile"`
	AvatarID         *string    `json:"avatar_id,omitempty" db:"avatar_id"`
	ParentUserID     *uuid.UUID `json:"parent_user_id,omitempty" db:"parent_user_id"`
	HasPin           bool       `json:"has_pin" db:"-"`
	IsMaster         bool       `json:"is_master" db:"-"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// ContentRatingLevel returns the numeric level for a content rating string.
// Higher values are more restrictive. Returns 999 for unknown/nil (unrestricted).
func ContentRatingLevel(rating string) int {
	levels := map[string]int{
		"G": 1, "PG": 2, "PG-13": 3, "R": 4, "NC-17": 5,
		"TV-Y": 1, "TV-Y7": 2, "TV-G": 2, "TV-PG": 3, "TV-14": 4, "TV-MA": 5,
	}
	if v, ok := levels[rating]; ok {
		return v
	}
	return 999
}

// AllowedContentRatings returns all ratings at or below the given max rating.
func AllowedContentRatings(maxRating string) []string {
	if maxRating == "" {
		return nil // nil means unrestricted
	}
	maxLevel := ContentRatingLevel(maxRating)
	all := []string{"G", "PG", "PG-13", "R", "NC-17", "TV-Y", "TV-Y7", "TV-G", "TV-PG", "TV-14", "TV-MA"}
	var allowed []string
	for _, r := range all {
		if ContentRatingLevel(r) <= maxLevel {
			allowed = append(allowed, r)
		}
	}
	return allowed
}

// ──────────────────── Library ────────────────────

type Library struct {
	ID                uuid.UUID     `json:"id" db:"id"`
	Name              string        `json:"name" db:"name"`
	MediaType         MediaType     `json:"media_type" db:"media_type"`
	Path              string        `json:"path" db:"path"`
	IsEnabled         bool          `json:"is_enabled" db:"is_enabled"`
	ScanOnStartup     bool          `json:"scan_on_startup" db:"scan_on_startup"`
	SeasonGrouping    bool          `json:"season_grouping" db:"season_grouping"`
	AccessLevel       LibraryAccess `json:"access_level" db:"access_level"`
	IncludeInHomepage bool          `json:"include_in_homepage" db:"include_in_homepage"`
	IncludeInSearch   bool          `json:"include_in_search" db:"include_in_search"`
	RetrieveMetadata    bool          `json:"retrieve_metadata" db:"retrieve_metadata"`
	NFOImport           bool          `json:"nfo_import" db:"nfo_import"`
	NFOExport           bool          `json:"nfo_export" db:"nfo_export"`
	PreferLocalArtwork  bool          `json:"prefer_local_artwork" db:"prefer_local_artwork"`
	AdultContentType    *string       `json:"adult_content_type,omitempty" db:"adult_content_type"`
	ScanInterval      string        `json:"scan_interval" db:"scan_interval"`
	NextScanAt        *time.Time    `json:"next_scan_at,omitempty" db:"next_scan_at"`
	WatchEnabled      bool          `json:"watch_enabled" db:"watch_enabled"`
	LastScanAt        *time.Time    `json:"last_scan_at" db:"last_scan_at"`
	CreatedAt         time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at" db:"updated_at"`
	// Aggregated (not in DB)
	Folders []LibraryFolder `json:"folders,omitempty" db:"-"`
}

type LibraryPermission struct {
	ID        uuid.UUID `json:"id" db:"id"`
	LibraryID uuid.UUID `json:"library_id" db:"library_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type LibraryFolder struct {
	ID           uuid.UUID `json:"id" db:"id"`
	LibraryID    uuid.UUID `json:"library_id" db:"library_id"`
	FolderPath   string    `json:"folder_path" db:"folder_path"`
	SortPosition int       `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
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
	Tagline          *string    `json:"tagline,omitempty" db:"tagline"`
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
	AudioFormat      *string    `json:"audio_format,omitempty" db:"audio_format"`
	OriginalLanguage *string    `json:"original_language,omitempty" db:"original_language"`
	Country          *string    `json:"country,omitempty" db:"country"`
	TrailerURL       *string    `json:"trailer_url,omitempty" db:"trailer_url"`
	PosterPath       *string    `json:"poster_path,omitempty" db:"poster_path"`
	ThumbnailPath    *string    `json:"thumbnail_path,omitempty" db:"thumbnail_path"`
	BackdropPath     *string    `json:"backdrop_path,omitempty" db:"backdrop_path"`
	LogoPath         *string    `json:"logo_path,omitempty" db:"logo_path"`
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
	// Edition fields (transient, not stored in DB)
	EditionGroupID *uuid.UUID `json:"edition_group_id,omitempty" db:"-"`
	EditionCount   int        `json:"edition_count,omitempty" db:"-"`
	// Series context (transient, populated by series detail queries)
	SeriesItemID *uuid.UUID `json:"series_item_id,omitempty" db:"-"`
	SeriesOrder  *int       `json:"series_order,omitempty" db:"-"`
	IMDBRating       *float64   `json:"imdb_rating,omitempty" db:"imdb_rating"`
	RTRating         *int       `json:"rt_rating,omitempty" db:"rt_rating"`
	AudienceScore    *int       `json:"audience_score,omitempty" db:"audience_score"`
	EditionType      string     `json:"edition_type" db:"edition_type"`
	ContentRating    *string    `json:"content_rating,omitempty" db:"content_rating"`
	ExternalIDs      *string    `json:"external_ids,omitempty" db:"external_ids"`
	GeneratedPoster  bool       `json:"generated_poster" db:"generated_poster"`
	// Technical metadata (persisted from filename parser + ffprobe)
	SourceType       *string         `json:"source_type,omitempty" db:"source_type"`
	HDRFormat        *string         `json:"hdr_format,omitempty" db:"hdr_format"`
	DynamicRange     string          `json:"dynamic_range" db:"dynamic_range"`
	// Keywords from TMDB (JSON array of strings)
	Keywords         *string         `json:"keywords,omitempty" db:"keywords"`
	// Unified cache server metadata
	MetacriticScore    *int    `json:"metacritic_score,omitempty" db:"metacritic_score"`
	ContentRatingsJSON *string `json:"content_ratings_json,omitempty" db:"content_ratings_json"`
	TaglinesJSON       *string `json:"taglines_json,omitempty" db:"taglines_json"`
	TrailersJSON       *string `json:"trailers_json,omitempty" db:"trailers_json"`
	DescriptionsJSON   *string `json:"descriptions_json,omitempty" db:"descriptions_json"`
	// Power-user annotation fields
	CustomNotes      *string         `json:"custom_notes,omitempty" db:"custom_notes"`
	CustomTags       *string         `json:"custom_tags,omitempty" db:"custom_tags"`
	MetadataLocked   bool            `json:"metadata_locked" db:"metadata_locked"`
	LockedFields     pq.StringArray  `json:"locked_fields" db:"locked_fields"`
	DuplicateStatus  string          `json:"duplicate_status" db:"duplicate_status"`
	PreviewPath      *string         `json:"preview_path,omitempty" db:"preview_path"`
	SpritePath       *string         `json:"sprite_path,omitempty" db:"sprite_path"`
	// Extras support (trailers, featurettes, etc.)
	ParentMediaID    *uuid.UUID      `json:"parent_media_id,omitempty" db:"parent_media_id"`
	ExtraType        *string         `json:"extra_type,omitempty" db:"extra_type"`
	AddedAt          time.Time  `json:"added_at" db:"added_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
	LastScannedAt    *time.Time `json:"last_scanned_at,omitempty" db:"last_scanned_at"`
}

// IsFieldLocked returns true if the given field name is in the locked_fields array.
// A special value "*" means all fields are locked.
func (m *MediaItem) IsFieldLocked(field string) bool {
	for _, f := range m.LockedFields {
		if f == "*" || f == field {
			return true
		}
	}
	return false
}

// ──────────────────── TV ────────────────────

type TVShow struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	LibraryID     uuid.UUID  `json:"library_id" db:"library_id"`
	Title         string     `json:"title" db:"title"`
	SortTitle     *string    `json:"sort_title,omitempty" db:"sort_title"`
	OriginalTitle *string    `json:"original_title,omitempty" db:"original_title"`
	Description   *string    `json:"description,omitempty" db:"description"`
	Tagline       *string    `json:"tagline,omitempty" db:"tagline"`
	Year          *int       `json:"year,omitempty" db:"year"`
	FirstAirDate  *time.Time `json:"first_air_date,omitempty" db:"first_air_date"`
	LastAirDate   *time.Time `json:"last_air_date,omitempty" db:"last_air_date"`
	Status        *string    `json:"status,omitempty" db:"status"`
	Network       *string    `json:"network,omitempty" db:"network"`
	Rating        *float64   `json:"rating,omitempty" db:"rating"`
	ContentRating *string    `json:"content_rating,omitempty" db:"content_rating"`
	PosterPath    *string    `json:"poster_path,omitempty" db:"poster_path"`
	BackdropPath  *string    `json:"backdrop_path,omitempty" db:"backdrop_path"`
	BannerPath    *string    `json:"banner_path,omitempty" db:"banner_path"`
	ExternalIDs   *string    `json:"external_ids,omitempty" db:"external_ids"`
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
	ExternalIDs  *string    `json:"external_ids,omitempty" db:"external_ids"`
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

// ──────────────────── Movie Series ────────────────────

type MovieSeries struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	LibraryID    uuid.UUID  `json:"library_id" db:"library_id"`
	Name         string     `json:"name" db:"name"`
	Description  *string    `json:"description,omitempty" db:"description"`
	PosterPath   *string    `json:"poster_path,omitempty" db:"poster_path"`
	BackdropPath *string    `json:"backdrop_path,omitempty" db:"backdrop_path"`
	ExternalIDs  *string    `json:"external_ids,omitempty" db:"external_ids"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated
	ItemCount int               `json:"item_count,omitempty" db:"-"`
	Items     []MovieSeriesItem `json:"items,omitempty" db:"-"`
}

type MovieSeriesItem struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	SeriesID    uuid.UUID  `json:"series_id" db:"series_id"`
	MediaItemID uuid.UUID  `json:"media_item_id" db:"media_item_id"`
	SortOrder   int        `json:"sort_order" db:"sort_order"`
	AddedAt     time.Time  `json:"added_at" db:"added_at"`
	// Joined
	Title      string  `json:"title,omitempty" db:"-"`
	Year       *int    `json:"year,omitempty" db:"-"`
	PosterPath *string `json:"poster_path,omitempty" db:"-"`
}

// ──────────────────── Collections ────────────────────

type Collection struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	UserID             uuid.UUID  `json:"user_id" db:"user_id"`
	LibraryID          *uuid.UUID `json:"library_id,omitempty" db:"library_id"`
	Name               string     `json:"name" db:"name"`
	Description        *string    `json:"description,omitempty" db:"description"`
	PosterPath         *string    `json:"poster_path,omitempty" db:"poster_path"`
	CollectionType     string     `json:"collection_type" db:"collection_type"`
	Visibility         string     `json:"visibility" db:"visibility"`
	ItemSortMode       string     `json:"item_sort_mode" db:"item_sort_mode"`
	SortPosition       int        `json:"sort_position" db:"sort_position"`
	Rules              *string    `json:"rules,omitempty" db:"rules"`
	ParentCollectionID *uuid.UUID `json:"parent_collection_id,omitempty" db:"parent_collection_id"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated
	ItemCount  int              `json:"item_count,omitempty" db:"-"`
	Items      []CollectionItem `json:"items,omitempty" db:"-"`
	ChildCount int              `json:"child_count,omitempty" db:"-"`
}

// SmartCollectionRules defines the criteria for a smart collection.
type SmartCollectionRules struct {
	Genres         []string `json:"genres,omitempty"`
	ExcludeGenres  []string `json:"exclude_genres,omitempty"`
	Moods          []string `json:"moods,omitempty"`
	YearFrom       *int     `json:"year_from,omitempty"`
	YearTo         *int     `json:"year_to,omitempty"`
	MinRating      *float64 `json:"min_rating,omitempty"`
	ContentRating  []string `json:"content_rating,omitempty"`
	MediaTypes     []string `json:"media_types,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	Performers     []string `json:"performers,omitempty"`
	Studios        []string `json:"studios,omitempty"`
	MinDuration    *int     `json:"min_duration,omitempty"`
	MaxDuration    *int     `json:"max_duration,omitempty"`
	AddedWithin    *int     `json:"added_within,omitempty"`
	ReleasedWithin *int     `json:"released_within,omitempty"`
	SortBy         string   `json:"sort_by,omitempty"`
	SortOrder      string   `json:"sort_order,omitempty"`
	MaxResults     int      `json:"max_results,omitempty"`
}

// CollectionStats holds aggregate statistics for a collection.
type CollectionStats struct {
	TotalItems   int                `json:"total_items"`
	TotalRuntime int                `json:"total_runtime_seconds"`
	AvgRating    float64            `json:"avg_rating"`
	Genres       []CollectionGenre  `json:"genres,omitempty"`
}

// CollectionGenre is a genre name with its count within a collection.
type CollectionGenre struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
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
	// Joined metadata (populated by ListItems)
	Title           string  `json:"title,omitempty" db:"-"`
	Year            *int    `json:"year,omitempty" db:"-"`
	PosterPath      *string `json:"poster_path,omitempty" db:"-"`
	Rating          *float64 `json:"rating,omitempty" db:"-"`
	DurationSeconds *int    `json:"duration_seconds,omitempty" db:"-"`
	Resolution      *string `json:"resolution,omitempty" db:"-"`
	MediaType       string  `json:"media_type,omitempty" db:"-"`
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

// ──────────────────── Because You Watched ────────────────────

// BecauseYouWatchedRow groups similar items under a source item the user watched.
type BecauseYouWatchedRow struct {
	SourceItem   *MediaItem   `json:"source_item"`
	SimilarItems []*MediaItem `json:"similar_items"`
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

// ──────────────────── Performers ────────────────────

type PerformerType string

const (
	PerformerActor        PerformerType = "actor"
	PerformerDirector     PerformerType = "director"
	PerformerProducer     PerformerType = "producer"
	PerformerMusician     PerformerType = "musician"
	PerformerNarrator     PerformerType = "narrator"
	PerformerAdult        PerformerType = "adult_performer"
	PerformerOther        PerformerType = "other"
)

type Performer struct {
	ID            uuid.UUID     `json:"id" db:"id"`
	Name          string        `json:"name" db:"name"`
	SortName      *string       `json:"sort_name,omitempty" db:"sort_name"`
	PerformerType PerformerType `json:"performer_type" db:"performer_type"`
	PhotoPath     *string       `json:"photo_path,omitempty" db:"photo_path"`
	Bio           *string       `json:"bio,omitempty" db:"bio"`
	BirthDate     *time.Time    `json:"birth_date,omitempty" db:"birth_date"`
	DeathDate     *time.Time    `json:"death_date,omitempty" db:"death_date"`
	SortPosition  int           `json:"sort_position" db:"sort_position"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at" db:"updated_at"`
	// Aggregated
	MediaCount int `json:"media_count,omitempty" db:"-"`
}

type MediaPerformer struct {
	ID            uuid.UUID `json:"id" db:"id"`
	MediaItemID   uuid.UUID `json:"media_item_id" db:"media_item_id"`
	PerformerID   uuid.UUID `json:"performer_id" db:"performer_id"`
	Role          string    `json:"role" db:"role"`
	CharacterName *string   `json:"character_name,omitempty" db:"character_name"`
	SortOrder     int       `json:"sort_order" db:"sort_order"`
}

// CastMember is the API response for a performer linked to a media item,
// including the performer details and their role/character in the media.
type CastMember struct {
	PerformerID   uuid.UUID     `json:"performer_id"`
	Name          string        `json:"name"`
	PerformerType PerformerType `json:"performer_type"`
	PhotoPath     *string       `json:"photo_path,omitempty"`
	Role          string        `json:"role"`
	CharacterName *string       `json:"character_name,omitempty"`
	SortOrder     int           `json:"sort_order"`
}

// ──────────────────── Tags ────────────────────

type TagCategory string

const (
	TagCategoryGenre  TagCategory = "genre"
	TagCategoryTag    TagCategory = "tag"
	TagCategoryCustom TagCategory = "custom"
	TagCategoryMood   TagCategory = "mood"
)

type Tag struct {
	ID           uuid.UUID   `json:"id" db:"id"`
	Name         string      `json:"name" db:"name"`
	Slug         string      `json:"slug" db:"slug"`
	ParentID     *uuid.UUID  `json:"parent_id,omitempty" db:"parent_id"`
	Category     TagCategory `json:"category" db:"category"`
	Description  *string     `json:"description,omitempty" db:"description"`
	SortPosition int         `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
	// Aggregated
	MediaCount int    `json:"media_count,omitempty" db:"-"`
	Children   []*Tag `json:"children,omitempty" db:"-"`
}

// ──────────────────── Studios ────────────────────

type StudioType string

const (
	StudioTypeStudio      StudioType = "studio"
	StudioTypeLabel       StudioType = "label"
	StudioTypePublisher   StudioType = "publisher"
	StudioTypeNetwork     StudioType = "network"
	StudioTypeDistributor StudioType = "distributor"
)

type Studio struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Name         string     `json:"name" db:"name"`
	StudioType   StudioType `json:"studio_type" db:"studio_type"`
	LogoPath     *string    `json:"logo_path,omitempty" db:"logo_path"`
	Description  *string    `json:"description,omitempty" db:"description"`
	Website      *string    `json:"website,omitempty" db:"website"`
	SortPosition int        `json:"sort_position" db:"sort_position"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	// Aggregated
	MediaCount int `json:"media_count,omitempty" db:"-"`
}

// ──────────────────── Media Subtitles ────────────────────

type SubtitleSource string

const (
	SubtitleSourceExternal SubtitleSource = "external"
	SubtitleSourceEmbedded SubtitleSource = "embedded"
)

type MediaSubtitle struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	MediaItemID uuid.UUID      `json:"media_item_id" db:"media_item_id"`
	Language    *string        `json:"language,omitempty" db:"language"`
	Title       *string        `json:"title,omitempty" db:"title"`
	Format      string         `json:"format" db:"format"`
	FilePath    *string        `json:"file_path,omitempty" db:"file_path"`
	StreamIndex *int           `json:"stream_index,omitempty" db:"stream_index"`
	Source      SubtitleSource `json:"source" db:"source"`
	IsDefault   bool           `json:"is_default" db:"is_default"`
	IsForced    bool           `json:"is_forced" db:"is_forced"`
	IsSDH       bool           `json:"is_sdh" db:"is_sdh"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Media Audio Tracks ────────────────────

type MediaAudioTrack struct {
	ID            uuid.UUID `json:"id" db:"id"`
	MediaItemID   uuid.UUID `json:"media_item_id" db:"media_item_id"`
	StreamIndex   int       `json:"stream_index" db:"stream_index"`
	Language      *string   `json:"language,omitempty" db:"language"`
	Title         *string   `json:"title,omitempty" db:"title"`
	Codec         string    `json:"codec" db:"codec"`
	Channels      int       `json:"channels" db:"channels"`
	ChannelLayout *string   `json:"channel_layout,omitempty" db:"channel_layout"`
	Bitrate       *int      `json:"bitrate,omitempty" db:"bitrate"`
	SampleRate    *int      `json:"sample_rate,omitempty" db:"sample_rate"`
	IsDefault     bool      `json:"is_default" db:"is_default"`
	IsCommentary  bool      `json:"is_commentary" db:"is_commentary"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Media Chapters ────────────────────

type MediaChapter struct {
	ID           uuid.UUID `json:"id" db:"id"`
	MediaItemID  uuid.UUID `json:"media_item_id" db:"media_item_id"`
	Title        *string   `json:"title,omitempty" db:"title"`
	StartSeconds float64   `json:"start_seconds" db:"start_seconds"`
	EndSeconds   *float64  `json:"end_seconds,omitempty" db:"end_seconds"`
	SortOrder    int       `json:"sort_order" db:"sort_order"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// ──────────────────── Streaming ────────────────────

type TranscodeSession struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	MediaItemID   uuid.UUID  `json:"media_item_id" db:"media_item_id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	Quality       string     `json:"quality" db:"quality"`
	Status        string     `json:"status" db:"status"`
	OutputDir     string     `json:"output_dir" db:"output_dir"`
	Pid           *int       `json:"pid,omitempty" db:"pid"`
	SegmentsReady int        `json:"segments_ready" db:"segments_ready"`
	StartedAt     time.Time  `json:"started_at" db:"started_at"`
	LastAccessAt  time.Time  `json:"last_access_at" db:"last_access_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// ──────────────────── Playback Preferences ────────────────────

type PlaybackMode string

const (
	PlaybackAlwaysAsk      PlaybackMode = "always_ask"
	PlaybackPlayDefault    PlaybackMode = "play_default"
	PlaybackHighestQuality PlaybackMode = "highest_quality"
	PlaybackLowestQuality  PlaybackMode = "lowest_quality"
	PlaybackLastPlayed     PlaybackMode = "last_played"
)

type UserPlaybackPreference struct {
	ID               uuid.UUID    `json:"id" db:"id"`
	UserID           uuid.UUID    `json:"user_id" db:"user_id"`
	PlaybackMode     PlaybackMode `json:"playback_mode" db:"playback_mode"`
	PreferredQuality string       `json:"preferred_quality" db:"preferred_quality"`
	AutoPlayNext     bool         `json:"auto_play_next" db:"auto_play_next"`
	SubtitleLanguage *string      `json:"subtitle_language,omitempty" db:"subtitle_language"`
	AudioLanguage    *string      `json:"audio_language,omitempty" db:"audio_language"`
	CreatedAt        time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Display Preferences ────────────────────

type UserDisplayPreferences struct {
	ID              uuid.UUID `json:"id" db:"id"`
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	OverlaySettings string    `json:"overlay_settings" db:"overlay_settings"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Job History ────────────────────

type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

type JobRecord struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	JobType      string     `json:"job_type" db:"job_type"`
	Status       JobStatus  `json:"status" db:"status"`
	Progress     int        `json:"progress" db:"progress"`
	ErrorMessage *string    `json:"error_message,omitempty" db:"error_message"`
	StartedBy    *uuid.UUID `json:"started_by,omitempty" db:"started_by"`
	StartedAt    time.Time  `json:"started_at" db:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Metadata Match ────────────────────

type MetadataMatch struct {
	Source           string   `json:"source"`
	ExternalID       string   `json:"external_id"`
	Title            string   `json:"title"`
	OriginalTitle    *string  `json:"original_title,omitempty"`
	Year             *int     `json:"year,omitempty"`
	ReleaseDate      *string  `json:"release_date,omitempty"`
	Description      *string  `json:"description,omitempty"`
	Tagline          *string  `json:"tagline,omitempty"`
	PosterURL        *string  `json:"poster_url,omitempty"`
	BackdropURL      *string  `json:"backdrop_url,omitempty"`
	Rating           *float64 `json:"rating,omitempty"`
	Genres           []string `json:"genres,omitempty"`
	IMDBId           string   `json:"imdb_id,omitempty"`
	ContentRating    *string  `json:"content_rating,omitempty"`
	OriginalLanguage *string  `json:"original_language,omitempty"`
	Country          *string  `json:"country,omitempty"`
	TrailerURL       *string  `json:"trailer_url,omitempty"`
	CollectionID     *int     `json:"collection_id,omitempty"`
	CollectionName   *string  `json:"collection_name,omitempty"`
	Keywords         []string `json:"keywords,omitempty"`
	Confidence       float64  `json:"confidence"`
}

// ──────────────────── Media Segments (Skip Detection) ────────────────────

type SegmentType string

const (
	SegmentIntro   SegmentType = "intro"
	SegmentCredits SegmentType = "credits"
	SegmentRecap   SegmentType = "recap"
	SegmentPreview SegmentType = "preview"
)

type SegmentSource string

const (
	SegmentSourceAuto      SegmentSource = "auto"
	SegmentSourceManual    SegmentSource = "manual"
	SegmentSourceCommunity SegmentSource = "community"
)

type MediaSegment struct {
	ID           uuid.UUID     `json:"id" db:"id"`
	MediaItemID  uuid.UUID     `json:"media_item_id" db:"media_item_id"`
	SegmentType  SegmentType   `json:"segment_type" db:"segment_type"`
	StartSeconds float64       `json:"start_seconds" db:"start_seconds"`
	EndSeconds   float64       `json:"end_seconds" db:"end_seconds"`
	Confidence   float64       `json:"confidence" db:"confidence"`
	Source       SegmentSource `json:"source" db:"source"`
	Verified     bool          `json:"verified" db:"verified"`
	CreatedAt    time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at" db:"updated_at"`
}

type UserSkipPreference struct {
	ID             uuid.UUID `json:"id" db:"id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	SkipIntros     bool      `json:"skip_intros" db:"skip_intros"`
	SkipCredits    bool      `json:"skip_credits" db:"skip_credits"`
	SkipRecaps     bool      `json:"skip_recaps" db:"skip_recaps"`
	ShowSkipButton bool      `json:"show_skip_button" db:"show_skip_button"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// ──────────────────── Duplicate Pair ────────────────────

type DuplicatePair struct {
	ID              uuid.UUID  `json:"id"`
	MediaA          *MediaItem `json:"media_a"`
	MediaB          *MediaItem `json:"media_b"`
	SimilarityScore float64    `json:"similarity_score"`
}

// ──────────────────── Stream Sessions ────────────────────

type PlaybackType string

const (
	PlaybackDirectPlay   PlaybackType = "direct_play"
	PlaybackDirectStream PlaybackType = "direct_stream"
	PlaybackTranscode    PlaybackType = "transcode"
)

type StreamSession struct {
	ID              uuid.UUID    `json:"id" db:"id"`
	UserID          uuid.UUID    `json:"user_id" db:"user_id"`
	MediaItemID     uuid.UUID    `json:"media_item_id" db:"media_item_id"`
	PlaybackType    PlaybackType `json:"playback_type" db:"playback_type"`
	Quality         *string      `json:"quality,omitempty" db:"quality"`
	Codec           *string      `json:"codec,omitempty" db:"codec"`
	Resolution      *string      `json:"resolution,omitempty" db:"resolution"`
	Container       *string      `json:"container,omitempty" db:"container"`
	BytesServed     int64        `json:"bytes_served" db:"bytes_served"`
	DurationSeconds int          `json:"duration_seconds" db:"duration_seconds"`
	ClientInfo      *string      `json:"client_info,omitempty" db:"client_info"`
	StartedAt       time.Time    `json:"started_at" db:"started_at"`
	EndedAt         *time.Time   `json:"ended_at,omitempty" db:"ended_at"`
	IsActive        bool         `json:"is_active" db:"is_active"`
	// Joined
	Username  string `json:"username,omitempty" db:"-"`
	MediaTitle string `json:"media_title,omitempty" db:"-"`
}

// ──────────────────── Transcode History ────────────────────

type TranscodeHistoryRecord struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	MediaItemID      uuid.UUID  `json:"media_item_id" db:"media_item_id"`
	UserID           uuid.UUID  `json:"user_id" db:"user_id"`
	InputCodec       *string    `json:"input_codec,omitempty" db:"input_codec"`
	OutputCodec      *string    `json:"output_codec,omitempty" db:"output_codec"`
	InputResolution  *string    `json:"input_resolution,omitempty" db:"input_resolution"`
	OutputResolution *string    `json:"output_resolution,omitempty" db:"output_resolution"`
	HWAccel          *string    `json:"hw_accel,omitempty" db:"hw_accel"`
	Quality          *string    `json:"quality,omitempty" db:"quality"`
	DurationSeconds  int        `json:"duration_seconds" db:"duration_seconds"`
	FileSizeBytes    int64      `json:"file_size_bytes" db:"file_size_bytes"`
	Success          bool       `json:"success" db:"success"`
	ErrorMessage     *string    `json:"error_message,omitempty" db:"error_message"`
	StartedAt        time.Time  `json:"started_at" db:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	// Joined
	MediaTitle string `json:"media_title,omitempty" db:"-"`
}

// ──────────────────── System Metrics ────────────────────

type SystemMetric struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	CPUPercent        float32    `json:"cpu_percent" db:"cpu_percent"`
	MemoryPercent     float32    `json:"memory_percent" db:"memory_percent"`
	MemoryUsedMB      int        `json:"memory_used_mb" db:"memory_used_mb"`
	GPUEncoderPercent *float32   `json:"gpu_encoder_percent,omitempty" db:"gpu_encoder_percent"`
	GPUMemoryPercent  *float32   `json:"gpu_memory_percent,omitempty" db:"gpu_memory_percent"`
	GPUTempCelsius    *float32   `json:"gpu_temp_celsius,omitempty" db:"gpu_temp_celsius"`
	DiskTotalGB       float32    `json:"disk_total_gb" db:"disk_total_gb"`
	DiskUsedGB        float32    `json:"disk_used_gb" db:"disk_used_gb"`
	DiskFreeGB        float32    `json:"disk_free_gb" db:"disk_free_gb"`
	ActiveStreams     int        `json:"active_streams" db:"active_streams"`
	ActiveTranscodes  int        `json:"active_transcodes" db:"active_transcodes"`
	RecordedAt        time.Time  `json:"recorded_at" db:"recorded_at"`
}

// ──────────────────── Daily Stats ────────────────────

type DailyStat struct {
	ID               uuid.UUID `json:"id" db:"id"`
	StatDate         time.Time `json:"stat_date" db:"stat_date"`
	TotalPlays       int       `json:"total_plays" db:"total_plays"`
	UniqueUsers      int       `json:"unique_users" db:"unique_users"`
	TotalWatchMinutes int      `json:"total_watch_minutes" db:"total_watch_minutes"`
	TotalBytesServed int64     `json:"total_bytes_served" db:"total_bytes_served"`
	Transcodes       int       `json:"transcodes" db:"transcodes"`
	DirectPlays      int       `json:"direct_plays" db:"direct_plays"`
	DirectStreams    int        `json:"direct_streams" db:"direct_streams"`
	TranscodeFailures int      `json:"transcode_failures" db:"transcode_failures"`
	NewMediaAdded    int       `json:"new_media_added" db:"new_media_added"`
	LibrarySizeTotal int       `json:"library_size_total" db:"library_size_total"`
	StorageUsedBytes int64     `json:"storage_used_bytes" db:"storage_used_bytes"`
}

// ──────────────────── Notification Channels ────────────────────

type NotificationChannel struct {
	ID          uuid.UUID         `json:"id" db:"id"`
	Name        string            `json:"name" db:"name"`
	ChannelType string            `json:"channel_type" db:"channel_type"`
	WebhookURL  string            `json:"webhook_url" db:"webhook_url"`
	IsEnabled   bool              `json:"is_enabled" db:"is_enabled"`
	Events      string            `json:"events" db:"events"`
	Config      *json.RawMessage  `json:"config,omitempty" db:"config"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
}

// GetConfig returns the channel config as a string map.
func (c *NotificationChannel) GetConfig() map[string]string {
	result := make(map[string]string)
	if c.Config == nil {
		return result
	}
	json.Unmarshal(*c.Config, &result)
	return result
}

// ──────────────────── Alert Rules ────────────────────

type AlertRule struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	Name            string     `json:"name" db:"name"`
	ConditionType   string     `json:"condition_type" db:"condition_type"`
	Threshold       float32    `json:"threshold" db:"threshold"`
	CooldownMinutes int        `json:"cooldown_minutes" db:"cooldown_minutes"`
	ChannelID       uuid.UUID  `json:"channel_id" db:"channel_id"`
	IsEnabled       bool       `json:"is_enabled" db:"is_enabled"`
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty" db:"last_triggered_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	// Joined
	ChannelName string `json:"channel_name,omitempty" db:"-"`
}

// ──────────────────── Alert Log ────────────────────

type AlertLogEntry struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	RuleID      *uuid.UUID `json:"rule_id,omitempty" db:"rule_id"`
	ChannelID   *uuid.UUID `json:"channel_id,omitempty" db:"channel_id"`
	Message     string     `json:"message" db:"message"`
	Success     bool       `json:"success" db:"success"`
	ErrorDetail *string    `json:"error_detail,omitempty" db:"error_detail"`
	SentAt      time.Time  `json:"sent_at" db:"sent_at"`
	// Joined
	RuleName    string `json:"rule_name,omitempty" db:"-"`
	ChannelName string `json:"channel_name,omitempty" db:"-"`
}

// ──────────────────── Analytics Response Types ────────────────────

type AnalyticsOverview struct {
	ActiveStreams     int     `json:"active_streams"`
	TotalPlaysToday  int     `json:"total_plays_today"`
	TotalPlaysWeek   int     `json:"total_plays_week"`
	UniqueUsersToday int     `json:"unique_users_today"`
	BandwidthToday   int64   `json:"bandwidth_today_bytes"`
	LibrarySize      int     `json:"library_size"`
	StorageUsedBytes int64   `json:"storage_used_bytes"`
	TranscodesToday  int     `json:"transcodes_today"`
	DirectPlaysToday int     `json:"direct_plays_today"`
	FailuresToday    int     `json:"failures_today"`
	CPUPercent       float32 `json:"cpu_percent"`
	MemoryPercent    float32 `json:"memory_percent"`
	DiskFreeGB       float32 `json:"disk_free_gb"`
}

type StreamTypeBreakdown struct {
	DirectPlays   int `json:"direct_plays"`
	DirectStreams int `json:"direct_streams"`
	Transcodes    int `json:"transcodes"`
	Total         int `json:"total"`
}

type UserActivitySummary struct {
	UserID          uuid.UUID `json:"user_id"`
	Username        string    `json:"username"`
	TotalPlays      int       `json:"total_plays"`
	TotalWatchMins  int       `json:"total_watch_minutes"`
	LastActive      time.Time `json:"last_active"`
	FavoriteGenre   string    `json:"favorite_genre,omitempty"`
}

type LibraryHealthReport struct {
	LibraryID         uuid.UUID          `json:"library_id"`
	LibraryName       string             `json:"library_name"`
	TotalItems        int                `json:"total_items"`
	MissingMetadata   int                `json:"missing_metadata"`
	MissingPoster     int                `json:"missing_poster"`
	MetadataPercent   float64            `json:"metadata_percent"`
	CodecDistribution []NameCount        `json:"codec_distribution"`
	ResolutionDist    []NameCount        `json:"resolution_distribution"`
	HDRCount          int                `json:"hdr_count"`
	AtmosCount        int                `json:"atmos_count"`
}

type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type StorageInfo struct {
	LibraryID   uuid.UUID `json:"library_id"`
	LibraryName string    `json:"library_name"`
	FileCount   int       `json:"file_count"`
	TotalBytes  int64     `json:"total_bytes"`
}

type TranscodeStats struct {
	TotalTranscodes   int         `json:"total_transcodes"`
	SuccessCount      int         `json:"success_count"`
	FailureCount      int         `json:"failure_count"`
	SuccessRate       float64     `json:"success_rate"`
	AvgDurationSecs   float64     `json:"avg_duration_seconds"`
	HWAccelPercent    float64     `json:"hw_accel_percent"`
	CodecDistribution []NameCount `json:"codec_distribution"`
}

type WatchActivityEntry struct {
	UserID        uuid.UUID `json:"user_id"`
	Username      string    `json:"username"`
	MediaItemID   uuid.UUID `json:"media_item_id"`
	MediaTitle    string    `json:"media_title"`
	PlaybackType  string    `json:"playback_type"`
	WatchedAt     time.Time `json:"watched_at"`
	DurationMins  int       `json:"duration_minutes"`
	Completed     bool      `json:"completed"`
	PosterPath    *string   `json:"poster_path,omitempty"`
}
