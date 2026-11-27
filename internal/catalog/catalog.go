// Package catalog holds CineVault's library data types, shared by the database
// layer and the API without tying the API to a database driver.
package catalog

import (
	"errors"
	"time"
)

// MediaTypes are the kinds of library CineVault supports.
var MediaTypes = []string{"movie", "tv", "music", "musicvideo", "audiobook", "book", "comic"}

// EditionType is one entry in the Edition List: a fixed edition name such as
// "Director's Cut" (see docs/specs/editions-and-versions.md §4).
type EditionType struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	ShortLabel  string   `json:"short_label"`
	MediaTypes  []string `json:"media_types"`
	Aliases     []string `json:"aliases"`
	SortOrder   int      `json:"sort_order"`
	IsDefault   bool     `json:"is_default"`
	Builtin     bool     `json:"builtin"`
	Hidden      bool     `json:"hidden"`
}

// AppliesTo reports whether the edition type can be used for the media type.
func (e EditionType) AppliesTo(mediaType string) bool {
	for _, m := range e.MediaTypes {
		if m == mediaType {
			return true
		}
	}
	return false
}

// ValidMediaType reports whether s is a supported media type.
func ValidMediaType(s string) bool {
	for _, m := range MediaTypes {
		if m == s {
			return true
		}
	}
	return false
}

// Errors shared by stores and the API.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

// Library is a set of folders holding one media type.
type Library struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	MediaType string    `json:"media_type"`
	Paths     []string  `json:"paths"`
	CreatedAt time.Time `json:"created_at"`
}

// Title is one work (a movie, show, album...) with its editions.
type Title struct {
	ID        int64  `json:"id"`
	LibraryID int64  `json:"library_id"`
	MediaType string `json:"media_type"`
	Name      string `json:"name"`
	SortName  string `json:"sort_name"`
	Year      int    `json:"year,omitempty"`
	TMDbID    int    `json:"tmdb_id,omitempty"`

	// Metadata from TMDb, filled in once the title is matched.
	OriginalTitle  string   `json:"original_title,omitempty"`
	Overview       string   `json:"overview,omitempty"`
	Tagline        string   `json:"tagline,omitempty"`
	ReleaseDate    string   `json:"release_date,omitempty"` // YYYY-MM-DD
	RuntimeMinutes int      `json:"runtime_minutes,omitempty"`
	PosterPath     string   `json:"poster_path,omitempty"`
	BackdropPath   string   `json:"backdrop_path,omitempty"`
	PosterURL      string   `json:"poster_url,omitempty"`
	BackdropURL    string   `json:"backdrop_url,omitempty"`
	IMDbID         string   `json:"imdb_id,omitempty"`
	Genres         []string `json:"genres"`
	Rating         float64  `json:"rating,omitempty"`
	CollectionID   int      `json:"collection_tmdb_id,omitempty"`
	CollectionName string   `json:"collection_name,omitempty"`
	// MatchStatus is unmatched, matched, not_found or error.
	MatchStatus string `json:"match_status"`
	MatchError  string `json:"match_error,omitempty"`
	// Artist is who performs a music video.
	Artist string `json:"artist,omitempty"`

	// For TV shows: how many seasons and episodes have files.
	SeasonCount  int `json:"season_count,omitempty"`
	EpisodeCount int `json:"episode_count,omitempty"`

	// Editions with their versions. A show's editions list no versions here;
	// its files are under its episodes (see Season).
	Editions []Edition `json:"editions"`
}

// Season is one season of a show with its episodes, owned or not.
type Season struct {
	Number    int       `json:"number"` // 0 is Specials
	Name      string    `json:"name,omitempty"`
	Overview  string    `json:"overview,omitempty"`
	PosterURL string    `json:"poster_url,omitempty"`
	AirDate   string    `json:"air_date,omitempty"`
	Episodes  []Episode `json:"episodes"`
}

// Episode is one episode. Versions is empty when the library has no file
// for it, unless it's part of a multi-episode file: then PartOf is the
// episode number whose file holds it.
type Episode struct {
	ID             int64     `json:"id"`
	Season         int       `json:"season"`
	Number         int       `json:"number"`
	Name           string    `json:"name,omitempty"`
	Overview       string    `json:"overview,omitempty"`
	AirDate        string    `json:"air_date,omitempty"`
	RuntimeMinutes int       `json:"runtime_minutes,omitempty"`
	StillURL       string    `json:"still_url,omitempty"`
	Rating         float64   `json:"rating,omitempty"`
	PartOf         int       `json:"part_of,omitempty"`
	Versions       []Version `json:"versions"`
}

// Edition is one cut of a title, such as the Director's Cut, with its files.
type Edition struct {
	ID             int64     `json:"id"`
	EditionTypeID  string    `json:"edition_type_id"`
	DisplayName    string    `json:"display_name"`
	ShortLabel     string    `json:"short_label"`
	RuntimeSeconds int       `json:"runtime_seconds,omitempty"`
	Versions       []Version `json:"versions"`
}

// Version is one file of an edition, such as a 4K or 1080p copy.
type Version struct {
	ID              int64   `json:"id"`
	Path            string  `json:"path"`
	SizeBytes       int64   `json:"size_bytes"`
	Label           string  `json:"label,omitempty"`
	Container       string  `json:"container,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	HDR             string  `json:"hdr,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
	AudioChannels   int     `json:"audio_channels,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Bitrate         int64   `json:"bitrate,omitempty"`
	EditionLocked   bool    `json:"edition_locked"`
	Missing         bool    `json:"missing"`
	// LastEpisode is the last episode in a multi-episode file, 0 otherwise.
	LastEpisode int `json:"last_episode,omitempty"`
}

// LinkMultiEpisodeFiles marks episodes held in another episode's file
// (S01E01-E02): an episode with no file of its own gets PartOf set to the
// episode number whose file covers it.
func LinkMultiEpisodeFiles(seasons []Season) {
	for si := range seasons {
		eps := seasons[si].Episodes
		for i := range eps {
			for _, v := range eps[i].Versions {
				for j := range eps {
					n := eps[j].Number
					if n > eps[i].Number && n <= v.LastEpisode && len(eps[j].Versions) == 0 {
						eps[j].PartOf = eps[i].Number
					}
				}
			}
		}
	}
}
