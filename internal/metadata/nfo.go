package metadata

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
)

// ──────────────────── Kodi-Compatible NFO Data Structures ────────────────────

// NFOData holds all metadata that can be read from or written to an NFO file.
// Compatible with the Kodi/Jellyfin/Emby NFO XML format.
type NFOData struct {
	Title         string
	OriginalTitle string
	SortTitle     string
	Tagline       string
	Plot          string // maps to Description
	Year          int
	Runtime       int // minutes
	MPAA          string
	Country       string
	TrailerURL    string

	// Ratings (multi-source)
	Ratings []NFORating

	// Provider IDs
	UniqueIDs []NFOUniqueID

	// People
	Directors []string
	Writers   []string
	Actors    []NFOActor

	// Categories
	Genres  []string
	Studios []string
	Tags    []string

	// Artwork references
	Thumbs  []NFOThumb
	Fanarts []NFOThumb

	// Lock control
	LockData bool

	// TV-specific
	ShowTitle     string
	SeasonNumber  int
	EpisodeNumber int
	Aired         string
}

// NFORating represents a named rating source in an NFO file.
type NFORating struct {
	Name  string  // e.g. "imdb", "tmdb", "default"
	Value float64
	Votes int
	Max   int // typically 10
}

// NFOUniqueID represents an external provider ID.
type NFOUniqueID struct {
	Type    string // "imdb", "tmdb", "tvdb"
	Value   string
	Default bool
}

// NFOActor represents a cast/crew member in an NFO file.
type NFOActor struct {
	Name      string
	Role      string
	Thumb     string
	Order     int
	SortOrder int
}

// NFOThumb represents an artwork reference in an NFO file.
type NFOThumb struct {
	Aspect string // "poster", "banner", "landscape"
	URL    string
}

// ──────────────────── XML Deserialization Structures ────────────────────

// xmlMovie represents the <movie> root element of a Kodi movie NFO.
type xmlMovie struct {
	XMLName       xml.Name       `xml:"movie"`
	Title         string         `xml:"title"`
	OriginalTitle string         `xml:"originaltitle"`
	SortTitle     string         `xml:"sorttitle"`
	Tagline       string         `xml:"tagline"`
	Plot          string         `xml:"plot"`
	Year          string         `xml:"year"`
	Runtime       string         `xml:"runtime"`
	MPAA          string         `xml:"mpaa"`
	Country       string         `xml:"country"`
	Trailer       string         `xml:"trailer"`
	Genres        []string       `xml:"genre"`
	Studios       []string       `xml:"studio"`
	Tags          []string       `xml:"tag"`
	Directors     []string       `xml:"director"`
	Credits       []string       `xml:"credits"`
	Actors        []xmlActor     `xml:"actor"`
	UniqueIDs     []xmlUniqueID  `xml:"uniqueid"`
	Ratings       *xmlRatings    `xml:"ratings"`
	Thumbs        []xmlThumb     `xml:"thumb"`
	Fanart        *xmlFanart     `xml:"fanart"`
	LockData      string         `xml:"lockdata"`
	// Legacy single-ID fields
	ID            string         `xml:"id"`
	IMDBId        string         `xml:"imdbid"`
	TMDBId        string         `xml:"tmdbid"`
}

// xmlTVShow represents the <tvshow> root element.
type xmlTVShow struct {
	XMLName       xml.Name       `xml:"tvshow"`
	Title         string         `xml:"title"`
	OriginalTitle string         `xml:"originaltitle"`
	SortTitle     string         `xml:"sorttitle"`
	Tagline       string         `xml:"tagline"`
	Plot          string         `xml:"plot"`
	Year          string         `xml:"year"`
	Runtime       string         `xml:"runtime"`
	MPAA          string         `xml:"mpaa"`
	Country       string         `xml:"country"`
	Genres        []string       `xml:"genre"`
	Studios       []string       `xml:"studio"`
	Tags          []string       `xml:"tag"`
	Actors        []xmlActor     `xml:"actor"`
	UniqueIDs     []xmlUniqueID  `xml:"uniqueid"`
	Ratings       *xmlRatings    `xml:"ratings"`
	Thumbs        []xmlThumb     `xml:"thumb"`
	Fanart        *xmlFanart     `xml:"fanart"`
	LockData      string         `xml:"lockdata"`
	ID            string         `xml:"id"`
}

// xmlEpisode represents the <episodedetails> root element.
type xmlEpisode struct {
	XMLName       xml.Name       `xml:"episodedetails"`
	Title         string         `xml:"title"`
	OriginalTitle string         `xml:"originaltitle"`
	Plot          string         `xml:"plot"`
	Year          string         `xml:"year"`
	Runtime       string         `xml:"runtime"`
	MPAA          string         `xml:"mpaa"`
	Season        string         `xml:"season"`
	Episode       string         `xml:"episode"`
	Aired         string         `xml:"aired"`
	ShowTitle     string         `xml:"showtitle"`
	Genres        []string       `xml:"genre"`
	Directors     []string       `xml:"director"`
	Credits       []string       `xml:"credits"`
	Actors        []xmlActor     `xml:"actor"`
	UniqueIDs     []xmlUniqueID  `xml:"uniqueid"`
	Ratings       *xmlRatings    `xml:"ratings"`
	Thumbs        []xmlThumb     `xml:"thumb"`
	LockData      string         `xml:"lockdata"`
	ID            string         `xml:"id"`
}

type xmlActor struct {
	Name      string `xml:"name"`
	Role      string `xml:"role"`
	Thumb     string `xml:"thumb"`
	Order     string `xml:"order"`
	SortOrder string `xml:"sortorder"`
}

type xmlUniqueID struct {
	Type    string `xml:"type,attr"`
	Default string `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

type xmlRatings struct {
	Ratings []xmlRating `xml:"rating"`
}

type xmlRating struct {
	Name  string  `xml:"name,attr"`
	Max   string  `xml:"max,attr"`
	Value float64 `xml:"value"`
	Votes int     `xml:"votes"`
}

type xmlThumb struct {
	Aspect string `xml:"aspect,attr"`
	URL    string `xml:",chardata"`
}

type xmlFanart struct {
	Thumbs []xmlThumb `xml:"thumb"`
}

// ──────────────────── NFO Reader ────────────────────

// ReadMovieNFO parses a Kodi-compatible movie NFO file.
func ReadMovieNFO(path string) (*NFOData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var movie xmlMovie
	if err := xml.Unmarshal(data, &movie); err != nil {
		// Not valid XML — might be a plain-text NFO with just an IMDB URL
		return nil, fmt.Errorf("not a valid XML NFO: %w", err)
	}

	// If title is empty, this isn't a real metadata NFO
	if movie.Title == "" {
		return nil, fmt.Errorf("NFO has no title element")
	}

	nfo := &NFOData{
		Title:         movie.Title,
		OriginalTitle: movie.OriginalTitle,
		SortTitle:     movie.SortTitle,
		Tagline:       movie.Tagline,
		Plot:          movie.Plot,
		MPAA:          movie.MPAA,
		Country:       movie.Country,
		TrailerURL:    movie.Trailer,
		Genres:        movie.Genres,
		Studios:       movie.Studios,
		Tags:          movie.Tags,
		Directors:     movie.Directors,
		Writers:       movie.Credits,
		LockData:      strings.EqualFold(strings.TrimSpace(movie.LockData), "true"),
	}

	if y, err := strconv.Atoi(movie.Year); err == nil {
		nfo.Year = y
	}
	if r, err := strconv.Atoi(movie.Runtime); err == nil {
		nfo.Runtime = r
	}

	// Parse actors
	for _, a := range movie.Actors {
		actor := NFOActor{Name: a.Name, Role: a.Role, Thumb: a.Thumb}
		if o, err := strconv.Atoi(a.Order); err == nil {
			actor.Order = o
		}
		nfo.Actors = append(nfo.Actors, actor)
	}

	// Parse unique IDs
	nfo.UniqueIDs = parseUniqueIDs(movie.UniqueIDs)
	// Fallback: legacy single-ID fields
	if len(nfo.UniqueIDs) == 0 {
		if movie.IMDBId != "" {
			nfo.UniqueIDs = append(nfo.UniqueIDs, NFOUniqueID{Type: "imdb", Value: movie.IMDBId, Default: true})
		} else if movie.ID != "" && strings.HasPrefix(movie.ID, "tt") {
			nfo.UniqueIDs = append(nfo.UniqueIDs, NFOUniqueID{Type: "imdb", Value: movie.ID, Default: true})
		}
		if movie.TMDBId != "" {
			nfo.UniqueIDs = append(nfo.UniqueIDs, NFOUniqueID{Type: "tmdb", Value: movie.TMDBId})
		}
	}

	// Parse ratings
	if movie.Ratings != nil {
		nfo.Ratings = parseRatings(movie.Ratings)
	}

	// Parse artwork
	nfo.Thumbs = parseThumbs(movie.Thumbs)
	if movie.Fanart != nil {
		nfo.Fanarts = parseThumbs(movie.Fanart.Thumbs)
	}

	return nfo, nil
}

// ReadTVShowNFO parses a Kodi-compatible tvshow.nfo file.
func ReadTVShowNFO(path string) (*NFOData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var show xmlTVShow
	if err := xml.Unmarshal(data, &show); err != nil {
		return nil, fmt.Errorf("not a valid XML NFO: %w", err)
	}

	if show.Title == "" {
		return nil, fmt.Errorf("NFO has no title element")
	}

	nfo := &NFOData{
		Title:         show.Title,
		OriginalTitle: show.OriginalTitle,
		SortTitle:     show.SortTitle,
		Tagline:       show.Tagline,
		Plot:          show.Plot,
		MPAA:          show.MPAA,
		Country:       show.Country,
		Genres:        show.Genres,
		Studios:       show.Studios,
		Tags:          show.Tags,
		LockData:      strings.EqualFold(strings.TrimSpace(show.LockData), "true"),
	}

	if y, err := strconv.Atoi(show.Year); err == nil {
		nfo.Year = y
	}

	for _, a := range show.Actors {
		actor := NFOActor{Name: a.Name, Role: a.Role, Thumb: a.Thumb}
		if o, err := strconv.Atoi(a.Order); err == nil {
			actor.Order = o
		}
		nfo.Actors = append(nfo.Actors, actor)
	}

	nfo.UniqueIDs = parseUniqueIDs(show.UniqueIDs)
	if len(nfo.UniqueIDs) == 0 && show.ID != "" && strings.HasPrefix(show.ID, "tt") {
		nfo.UniqueIDs = append(nfo.UniqueIDs, NFOUniqueID{Type: "imdb", Value: show.ID, Default: true})
	}

	if show.Ratings != nil {
		nfo.Ratings = parseRatings(show.Ratings)
	}

	nfo.Thumbs = parseThumbs(show.Thumbs)
	if show.Fanart != nil {
		nfo.Fanarts = parseThumbs(show.Fanart.Thumbs)
	}

	return nfo, nil
}

// ReadEpisodeNFO parses a Kodi-compatible episodedetails NFO file.
func ReadEpisodeNFO(path string) (*NFOData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var ep xmlEpisode
	if err := xml.Unmarshal(data, &ep); err != nil {
		return nil, fmt.Errorf("not a valid XML NFO: %w", err)
	}

	if ep.Title == "" {
		return nil, fmt.Errorf("NFO has no title element")
	}

	nfo := &NFOData{
		Title:     ep.Title,
		Plot:      ep.Plot,
		MPAA:      ep.MPAA,
		Aired:     ep.Aired,
		ShowTitle: ep.ShowTitle,
		Directors: ep.Directors,
		Writers:   ep.Credits,
		Genres:    ep.Genres,
		LockData:  strings.EqualFold(strings.TrimSpace(ep.LockData), "true"),
	}

	if y, err := strconv.Atoi(ep.Year); err == nil {
		nfo.Year = y
	}
	if r, err := strconv.Atoi(ep.Runtime); err == nil {
		nfo.Runtime = r
	}
	if s, err := strconv.Atoi(ep.Season); err == nil {
		nfo.SeasonNumber = s
	}
	if e, err := strconv.Atoi(ep.Episode); err == nil {
		nfo.EpisodeNumber = e
	}

	for _, a := range ep.Actors {
		actor := NFOActor{Name: a.Name, Role: a.Role, Thumb: a.Thumb}
		if o, err := strconv.Atoi(a.Order); err == nil {
			actor.Order = o
		}
		nfo.Actors = append(nfo.Actors, actor)
	}

	nfo.UniqueIDs = parseUniqueIDs(ep.UniqueIDs)
	if ep.Ratings != nil {
		nfo.Ratings = parseRatings(ep.Ratings)
	}
	nfo.Thumbs = parseThumbs(ep.Thumbs)

	return nfo, nil
}

// ──────────────────── NFO Writer ────────────────────

// WriteMovieNFO generates a Kodi-compatible movie NFO file.
func WriteMovieNFO(item *models.MediaItem, performers []models.CastMember, genres []string, studios []string, path string) error {
	movie := xmlMovie{
		Title: item.Title,
	}
	if item.OriginalTitle != nil {
		movie.OriginalTitle = *item.OriginalTitle
	}
	if item.SortTitle != nil {
		movie.SortTitle = *item.SortTitle
	}
	if item.Tagline != nil {
		movie.Tagline = *item.Tagline
	}
	if item.Description != nil {
		movie.Plot = *item.Description
	}
	if item.Year != nil {
		movie.Year = strconv.Itoa(*item.Year)
	}
	if item.DurationSeconds != nil {
		movie.Runtime = strconv.Itoa(*item.DurationSeconds / 60)
	}
	if item.ContentRating != nil {
		movie.MPAA = *item.ContentRating
	}
	if item.Country != nil {
		movie.Country = *item.Country
	}
	if item.TrailerURL != nil {
		movie.Trailer = *item.TrailerURL
	}

	movie.Genres = genres
	movie.Studios = studios

	// Build unique IDs from external_ids JSON
	movie.UniqueIDs = buildUniqueIDsFromItem(item)

	// Build ratings
	var ratings []xmlRating
	if item.Rating != nil {
		ratings = append(ratings, xmlRating{Name: "tmdb", Max: "10", Value: *item.Rating})
	}
	if item.IMDBRating != nil {
		ratings = append(ratings, xmlRating{Name: "imdb", Max: "10", Value: *item.IMDBRating})
	}
	if len(ratings) > 0 {
		movie.Ratings = &xmlRatings{Ratings: ratings}
	}

	// Build actors
	for _, p := range performers {
		a := xmlActor{Name: p.Name, Order: strconv.Itoa(p.SortOrder)}
		if p.CharacterName != nil {
			a.Role = *p.CharacterName
		}
		if p.PhotoPath != nil {
			a.Thumb = *p.PhotoPath
		}
		if p.Role == "director" {
			movie.Directors = append(movie.Directors, p.Name)
		} else if p.Role == "screenplay" || p.Role == "writer" || p.Role == "story" {
			movie.Credits = append(movie.Credits, p.Name)
		} else {
			movie.Actors = append(movie.Actors, a)
		}
	}

	if item.MetadataLocked {
		movie.LockData = "true"
	}

	return writeNFOFile(path, movie)
}

// WriteTVShowNFO generates a Kodi-compatible tvshow.nfo file.
func WriteTVShowNFO(show *models.TVShow, genres []string, path string) error {
	tv := xmlTVShow{
		Title: show.Title,
	}
	if show.OriginalTitle != nil {
		tv.OriginalTitle = *show.OriginalTitle
	}
	if show.SortTitle != nil {
		tv.SortTitle = *show.SortTitle
	}
	if show.Tagline != nil {
		tv.Tagline = *show.Tagline
	}
	if show.Description != nil {
		tv.Plot = *show.Description
	}
	if show.Year != nil {
		tv.Year = strconv.Itoa(*show.Year)
	}
	if show.ContentRating != nil {
		tv.MPAA = *show.ContentRating
	}

	tv.Genres = genres

	// Build unique IDs from show external_ids
	if show.ExternalIDs != nil {
		tv.UniqueIDs = buildUniqueIDsFromJSON(*show.ExternalIDs)
	}

	return writeNFOFile(path, tv)
}

// WriteEpisodeNFO generates a Kodi-compatible episodedetails NFO file.
func WriteEpisodeNFO(item *models.MediaItem, showTitle string, seasonNum, episodeNum int, path string) error {
	ep := xmlEpisode{
		Title:     item.Title,
		ShowTitle: showTitle,
		Season:    strconv.Itoa(seasonNum),
		Episode:   strconv.Itoa(episodeNum),
	}
	if item.Description != nil {
		ep.Plot = *item.Description
	}
	if item.Year != nil {
		ep.Year = strconv.Itoa(*item.Year)
	}
	if item.DurationSeconds != nil {
		ep.Runtime = strconv.Itoa(*item.DurationSeconds / 60)
	}
	if item.ContentRating != nil {
		ep.MPAA = *item.ContentRating
	}

	ep.UniqueIDs = buildUniqueIDsFromItem(item)

	var ratings []xmlRating
	if item.Rating != nil {
		ratings = append(ratings, xmlRating{Name: "tmdb", Max: "10", Value: *item.Rating})
	}
	if len(ratings) > 0 {
		ep.Ratings = &xmlRatings{Ratings: ratings}
	}

	return writeNFOFile(path, ep)
}

// ──────────────────── NFO File Discovery ────────────────────

// FindNFOFile locates the NFO sidecar file for a media file.
// Checks: <filename>.nfo, then movie.nfo or tvshow.nfo in the directory.
func FindNFOFile(mediaFilePath string, mediaType models.MediaType) string {
	dir := filepath.Dir(mediaFilePath)
	base := strings.TrimSuffix(filepath.Base(mediaFilePath), filepath.Ext(mediaFilePath))

	// Check for exact filename match: MovieName.nfo
	nfoPath := filepath.Join(dir, base+".nfo")
	if _, err := os.Stat(nfoPath); err == nil {
		return nfoPath
	}

	// Check for type-specific NFO names
	switch mediaType {
	case models.MediaTypeMovies, models.MediaTypeAdultMovies:
		movieNFO := filepath.Join(dir, "movie.nfo")
		if _, err := os.Stat(movieNFO); err == nil {
			return movieNFO
		}
	case models.MediaTypeTVShows:
		// For episodes, check in parent directories for tvshow.nfo
		// (episode-level NFO is the filename.nfo already checked above)
	}

	// Fallback: any .nfo in the directory if only one video file exists
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	videoCount := 0
	var nfoFiles []string
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".m4v": true, ".wmv": true, ".flv": true, ".webm": true,
		".ts": true, ".m2ts": true, ".mpg": true, ".mpeg": true,
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if videoExts[ext] {
			videoCount++
		}
		if ext == ".nfo" {
			nfoFiles = append(nfoFiles, filepath.Join(dir, e.Name()))
		}
	}
	if videoCount == 1 && len(nfoFiles) > 0 {
		return nfoFiles[0]
	}

	return ""
}

// FindTVShowNFO searches for a tvshow.nfo file in the show's root directory.
// Walks up from the media file path looking for tvshow.nfo.
func FindTVShowNFO(mediaFilePath string, libraryPath string) string {
	dir := filepath.Dir(mediaFilePath)

	// Walk up from the file's directory to the library root
	for dir != libraryPath && len(dir) > len(libraryPath) {
		nfoPath := filepath.Join(dir, "tvshow.nfo")
		if _, err := os.Stat(nfoPath); err == nil {
			return nfoPath
		}
		dir = filepath.Dir(dir)
	}

	// Check the library root itself
	nfoPath := filepath.Join(libraryPath, "tvshow.nfo")
	if _, err := os.Stat(nfoPath); err == nil {
		return nfoPath
	}

	return ""
}

// ──────────────────── NFO Helper: Extract Provider IDs ────────────────────

// GetIMDBID returns the IMDB ID from the NFO data, if present.
func (n *NFOData) GetIMDBID() string {
	for _, uid := range n.UniqueIDs {
		if uid.Type == "imdb" {
			return uid.Value
		}
	}
	return ""
}

// GetTMDBID returns the TMDB ID from the NFO data, if present.
func (n *NFOData) GetTMDBID() string {
	for _, uid := range n.UniqueIDs {
		if uid.Type == "tmdb" {
			return uid.Value
		}
	}
	return ""
}

// GetTVDBID returns the TVDB ID from the NFO data, if present.
func (n *NFOData) GetTVDBID() string {
	for _, uid := range n.UniqueIDs {
		if uid.Type == "tvdb" {
			return uid.Value
		}
	}
	return ""
}

// GetDefaultRating returns the primary rating value from the NFO, preferring "default" then "tmdb".
func (n *NFOData) GetDefaultRating() *float64 {
	for _, r := range n.Ratings {
		if r.Name == "default" || r.Name == "tmdb" {
			return &r.Value
		}
	}
	if len(n.Ratings) > 0 {
		return &n.Ratings[0].Value
	}
	return nil
}

// HasFullMetadata returns true if the NFO has enough data to skip an external lookup.
func (n *NFOData) HasFullMetadata() bool {
	return n.Title != "" && n.Plot != "" && n.Year > 0
}

// ──────────────────── Internal Helpers ────────────────────

func parseUniqueIDs(ids []xmlUniqueID) []NFOUniqueID {
	var result []NFOUniqueID
	for _, uid := range ids {
		result = append(result, NFOUniqueID{
			Type:    uid.Type,
			Value:   strings.TrimSpace(uid.Value),
			Default: uid.Default == "true",
		})
	}
	return result
}

func parseRatings(r *xmlRatings) []NFORating {
	var result []NFORating
	for _, rating := range r.Ratings {
		nr := NFORating{
			Name:  rating.Name,
			Value: rating.Value,
			Votes: rating.Votes,
		}
		if m, err := strconv.Atoi(rating.Max); err == nil {
			nr.Max = m
		} else {
			nr.Max = 10
		}
		result = append(result, nr)
	}
	return result
}

func parseThumbs(thumbs []xmlThumb) []NFOThumb {
	var result []NFOThumb
	for _, t := range thumbs {
		url := strings.TrimSpace(t.URL)
		if url != "" {
			result = append(result, NFOThumb{
				Aspect: t.Aspect,
				URL:    url,
			})
		}
	}
	return result
}

func buildUniqueIDsFromItem(item *models.MediaItem) []xmlUniqueID {
	if item.ExternalIDs == nil {
		return nil
	}
	return buildUniqueIDsFromJSON(*item.ExternalIDs)
}

func buildUniqueIDsFromJSON(jsonStr string) []xmlUniqueID {
	// Parse the external_ids JSON: {"source":"tmdb","tmdb_id":"12345","imdb_id":"tt1234567"}
	var ids map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &ids); err != nil {
		return nil
	}

	var result []xmlUniqueID

	if tmdbID, ok := ids["tmdb_id"]; ok && tmdbID != nil {
		result = append(result, xmlUniqueID{Type: "tmdb", Value: fmt.Sprintf("%v", tmdbID), Default: "true"})
	}
	if imdbID, ok := ids["imdb_id"]; ok && imdbID != nil {
		val := fmt.Sprintf("%v", imdbID)
		if val != "" {
			result = append(result, xmlUniqueID{Type: "imdb", Value: val})
		}
	}
	if tvdbID, ok := ids["tvdb_id"]; ok && tvdbID != nil {
		result = append(result, xmlUniqueID{Type: "tvdb", Value: fmt.Sprintf("%v", tvdbID)})
	}

	return result
}

func writeNFOFile(path string, v interface{}) error {
	data, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal NFO: %w", err)
	}

	// Prepend XML declaration
	xmlHeader := []byte(xml.Header)
	output := append(xmlHeader, data...)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create NFO directory: %w", err)
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		return fmt.Errorf("write NFO file: %w", err)
	}

	log.Printf("NFO: wrote %s", path)
	return nil
}
