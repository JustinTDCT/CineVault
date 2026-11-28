// Package cacheapi is the contract between a CineVault server and the
// CineVault Cache service (the paid subscription in the CineVault Cache spec):
// the JSON types of API version 1, their validation rules, and a client.
//
// Both sides import this package, so the wire format is defined once. The
// golden files in testdata pin it; change them only together with the
// service.
//
// Optional fields are omitted when empty. A client treats an absent field and
// a JSON null the same way.
package cacheapi

import "time"

// APIVersion is the path prefix every endpoint lives under.
const APIVersion = "v1"

// Limits the service enforces on one request.
const (
	MaxSyncItems      = 1000 // items in one sync request
	MaxRecordKeys     = 100  // keys in one records request
	MaxReportMessage  = 1000 // characters in a mistake report's message
	MaxLookupEditions = 20   // edition IDs passed with one lookup
)

// Kinds of title. A key's kind decides its ID space: movies and shows use
// TMDb IDs (movie 679 and TV show 679 are different works), music videos
// MusicBrainz recording IDs, albums MusicBrainz release group IDs, books and
// audiobooks Open Library work IDs, and comic series Comic Vine volume IDs.
const (
	KindMovie      = "movie"
	KindTV         = "tv"
	KindMusicVideo = "musicvideo"
	KindAlbum      = "album"
	KindBook       = "book"
	KindAudiobook  = "audiobook"
	KindComic      = "comic"
)

// Kinds lists every kind, in Edition List media type order.
var Kinds = []string{KindMovie, KindTV, KindAlbum, KindMusicVideo, KindAudiobook, KindBook, KindComic}

// Providers a key's ID comes from.
const (
	ProviderTMDb        = "tmdb"
	ProviderMusicBrainz = "musicbrainz"
	ProviderOpenLibrary = "openlibrary"
	ProviderComicVine   = "comicvine"
)

// Key identifies a title in the cache. Exactly one ID is set, the one its
// kind uses: TMDbID (movie, tv), MBID (musicvideo: a recording; album: a
// release group; both lowercase UUIDs), OLID (book, audiobook: an Open
// Library work such as "OL45883W") or CVID (comic: a Comic Vine volume).
type Key struct {
	Kind   string `json:"kind"`
	TMDbID int    `json:"tmdb_id,omitempty"`
	MBID   string `json:"mbid,omitempty"`
	OLID   string `json:"olid,omitempty"`
	CVID   int    `json:"cvid,omitempty"`
}

// Record statuses.
const (
	StatusReady     = "ready"
	StatusPending   = "pending"
	StatusWithdrawn = "withdrawn"
)

// Edition statuses. Only published and withdrawn editions are ever served.
const (
	EditionPublished = "published"
	EditionWithdrawn = "withdrawn"
)

// Confidence levels.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// Kinds of change in an edition's full difference notes.
const (
	ChangeAdded   = "added"
	ChangeRemoved = "removed"
	ChangeChanged = "changed"
)

// Record is everything the cache knows about one title. It is returned by a
// lookup and by the records call.
type Record struct {
	Key           Key       `json:"key"`
	Status        string    `json:"status"`         // ready or withdrawn
	RecordVersion int       `json:"record_version"` // only ever goes up
	PublishedAt   time.Time `json:"published_at,omitzero"`
	// EditionListVersion is the editionlist.Version the record was written
	// against.
	EditionListVersion int `json:"edition_list_version"`

	// Base is licensed provider metadata. It is absent when the cache may not
	// serve it; the server then keeps the metadata it fetched itself.
	Base *Base `json:"base,omitempty"`

	// Editions are in Edition List order. A withdrawn record has none.
	Editions []Edition `json:"editions,omitempty"`

	// Credits are a music video's on-screen credits (music videos only).
	Credits *Credits `json:"credits,omitempty"`

	Sources []Source `json:"sources,omitempty"`
}

// Credit fields, the keys of Credits.Sources.
const (
	CreditSong     = "song"
	CreditArtist   = "artist"
	CreditAlbum    = "album"
	CreditYear     = "year"
	CreditLabel    = "label"
	CreditDirector = "director"
)

// Credits are what a music channel shows in the corner of a music video:
// artist, "song", album (year), label and director. Every value names its
// sources in Sources, keyed by field; a value without a source is never sent.
type Credits struct {
	Song     string `json:"song"`
	Artist   string `json:"artist"`
	Album    string `json:"album,omitempty"`
	Year     int    `json:"year,omitempty"` // the album's (or the song's first) release year
	Label    string `json:"label,omitempty"`
	Director string `json:"director,omitempty"` // the video's director; several are joined with " & "

	Sources map[string][]string `json:"sources"`
	// Confidence is for the researched fields (director, label); the rest
	// come from MusicBrainz.
	Confidence string `json:"confidence,omitempty"`
}

// Base is provider metadata for the title, served only where licensed.
type Base struct {
	// License names whose data this is ("tmdb"), for attribution and for what
	// to remove when a subscription lapses.
	License          string   `json:"license"`
	Title            string   `json:"title"`
	OriginalTitle    string   `json:"original_title,omitempty"`
	ReleaseDate      string   `json:"release_date,omitempty"` // YYYY-MM-DD
	RuntimeMin       int      `json:"runtime_min,omitempty"`
	Overview         string   `json:"overview,omitempty"`
	Tagline          string   `json:"tagline,omitempty"`
	Genres           []string `json:"genres,omitempty"`
	IMDbID           string   `json:"imdb_id,omitempty"`
	CollectionTMDbID int      `json:"collection_tmdb_id,omitempty"`
	PosterPath       string   `json:"poster_path,omitempty"`
	BackdropPath     string   `json:"backdrop_path,omitempty"`
}

// Edition is one known edition of a title, owned by the user or not.
type Edition struct {
	// Edition is a built-in Edition List ID (package editionlist).
	Edition string `json:"edition"`
	Status  string `json:"status"` // published or withdrawn
	// DisplayName overrides the Edition List's display name for this title,
	// e.g. "Special Edition (Director's Cut)".
	DisplayName string `json:"display_name,omitempty"`

	// RuntimeMin is set only together with RuntimeSources: the cache never
	// sends a number without a source.
	RuntimeMin     int      `json:"runtime_min,omitempty"`
	RuntimeSources []string `json:"runtime_sources,omitempty"`

	// DifferenceShort is the line of about 15 words for cards and lists.
	DifferenceShort string       `json:"difference_short,omitempty"`
	DifferenceFull  []Difference `json:"difference_full,omitempty"` // story order
	ContentRating   *Rating      `json:"content_rating,omitempty"`  // only when it differs from the title's

	Confidence string `json:"confidence,omitempty"`
	Reviewed   bool   `json:"reviewed,omitempty"` // checked by a person
}

// Difference is one entry in an edition's full difference notes.
type Difference struct {
	Change  string   `json:"change"` // added, removed or changed
	Text    string   `json:"text"`
	Sources []string `json:"sources"`
}

// Rating is a content rating in one country.
type Rating struct {
	Country string `json:"country"` // ISO 3166-1, e.g. "US"
	Rating  string `json:"rating"`  // e.g. "R", "Unrated"
}

// Source is a reference a record's facts come from. Facts point to sources
// by ID.
type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Pending is the body of a 202 reply to a lookup: the title is being
// researched.
type Pending struct {
	Status      string `json:"status"` // pending
	RetryAfterS int    `json:"retry_after_s,omitempty"`
}

// SyncRequest asks which titles changed. List every pending title (with
// HaveVersion 0) and every cached one.
type SyncRequest struct {
	Items []SyncItem `json:"items"`
}

// SyncItem is one title in a sync request.
type SyncItem struct {
	Kind        string `json:"kind"`
	TMDbID      int    `json:"tmdb_id,omitempty"`
	MBID        string `json:"mbid,omitempty"`
	OLID        string `json:"olid,omitempty"`
	CVID        int    `json:"cvid,omitempty"`
	HaveVersion int    `json:"have_version"` // 0: no record yet
}

// Key returns the item's key.
func (i SyncItem) Key() Key {
	return Key{Kind: i.Kind, TMDbID: i.TMDbID, MBID: i.MBID, OLID: i.OLID, CVID: i.CVID}
}

// SyncItemFor returns a sync item for a key.
func SyncItemFor(k Key, haveVersion int) SyncItem {
	return SyncItem{Kind: k.Kind, TMDbID: k.TMDbID, MBID: k.MBID, OLID: k.OLID, CVID: k.CVID, HaveVersion: haveVersion}
}

// SyncResponse says which titles to fetch with the records call.
type SyncResponse struct {
	Changed   []Changed `json:"changed"`
	Pending   []Key     `json:"pending"`
	NotFound  []Key     `json:"not_found"`
	Unchanged int       `json:"unchanged"`
}

// Changed is a title whose record is newer than the one the server has
// (including newly ready and withdrawn records).
type Changed struct {
	Kind          string `json:"kind"`
	TMDbID        int    `json:"tmdb_id,omitempty"`
	MBID          string `json:"mbid,omitempty"`
	OLID          string `json:"olid,omitempty"`
	CVID          int    `json:"cvid,omitempty"`
	RecordVersion int    `json:"record_version"`
}

// Key returns the changed title's key.
func (c Changed) Key() Key {
	return Key{Kind: c.Kind, TMDbID: c.TMDbID, MBID: c.MBID, OLID: c.OLID, CVID: c.CVID}
}

// ChangedFor returns a changed entry for a key.
func ChangedFor(k Key, version int) Changed {
	return Changed{Kind: k.Kind, TMDbID: k.TMDbID, MBID: k.MBID, OLID: k.OLID, CVID: k.CVID, RecordVersion: version}
}

// RecordsRequest fetches up to MaxRecordKeys records.
type RecordsRequest struct {
	Keys []Key `json:"keys"`
}

// RecordsResponse returns the records in request order.
type RecordsResponse struct {
	Records []Record `json:"records"`
}

// Fields a mistake report can be about.
const (
	FieldExists          = "exists"
	FieldRuntime         = "runtime"
	FieldDifferenceShort = "difference_short"
	FieldDifferenceFull  = "difference_full"
	FieldContentRating   = "content_rating"
	FieldOther           = "other"
)

// Report is a "Report a mistake" sent on a user's behalf.
type Report struct {
	Key     Key    `json:"key"`
	Edition string `json:"edition,omitempty"` // Edition List ID; empty for the whole title
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ReportResponse acknowledges a report.
type ReportResponse struct {
	ReportID string `json:"report_id"`
}

// Subscription describes the server's token.
type Subscription struct {
	Status      string    `json:"status"` // active, lapsed or cancelled
	ServerLabel string    `json:"server_label,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitzero"`
}

// ServiceStatus is the reply to the unauthenticated status call.
type ServiceStatus struct {
	Status      string   `json:"status"`
	APIVersions []string `json:"api_versions"`
}

// Error codes, in ErrorBody.Error.Code.
const (
	CodeTokenMissing         = "token_missing"
	CodeTokenInvalid         = "token_invalid"
	CodeTokenRevoked         = "token_revoked"
	CodeSubscriptionInactive = "subscription_inactive"
	CodeNotFound             = "not_found"
	CodeInvalidRequest       = "invalid_request"
	CodeRateLimited          = "rate_limited"
	CodeInternal             = "internal"
)

// ErrorBody is the body of every error reply.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the code and message in an error reply.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Limits on a contribution (CineVault Cache spec §5a).
const (
	MaxContributeItems = 1000 // items in one contribute request
	MaxRuntimeMin      = 1000 // largest runtime a contribution may report
)

// ContributeRequest shares which titles a CineVault server has, with the
// edition IDs from its file names and each edition's measured runtime. Any
// server may send it, subscribed or not; it needs no token.
type ContributeRequest struct {
	// Install is a random ID the server made once (32 random bytes,
	// base32). It isn't tied to an account or a token.
	Install string           `json:"install"`
	Items   []ContributeItem `json:"items"`
}

// ContributeItem is one title the server has.
type ContributeItem struct {
	Kind     string              `json:"kind"`
	TMDbID   int                 `json:"tmdb_id,omitempty"`
	MBID     string              `json:"mbid,omitempty"`
	OLID     string              `json:"olid,omitempty"`
	CVID     int                 `json:"cvid,omitempty"`
	Editions []ContributeEdition `json:"editions,omitempty"`
}

// Key returns the item's key.
func (i ContributeItem) Key() Key {
	return Key{Kind: i.Kind, TMDbID: i.TMDbID, MBID: i.MBID, OLID: i.OLID, CVID: i.CVID}
}

// ContributeItemFor returns a contribute item for a key.
func ContributeItemFor(k Key, editions []ContributeEdition) ContributeItem {
	return ContributeItem{Kind: k.Kind, TMDbID: k.TMDbID, MBID: k.MBID, OLID: k.OLID, CVID: k.CVID, Editions: editions}
}

// ContributeEdition is an edition the server has a file for.
type ContributeEdition struct {
	Edition    string `json:"edition"`               // built-in Edition List ID
	RuntimeMin int    `json:"runtime_min,omitempty"` // the file's duration, rounded; 0 when unknown
}

// ContributeResponse acknowledges a contribution. Free servers get nothing
// else back.
type ContributeResponse struct {
	Accepted int `json:"accepted"`
}
