package scanner

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Parsed Result ────────────────────

// ParsedFilename holds all metadata extracted from a media filename.
type ParsedFilename struct {
	Title       string
	Year        *int
	Edition     string // defaults to "Theatrical" if none found
	Resolution  string // e.g. "1080p", "720p", "2160p"
	Container   string // e.g. "mkv", "mp4"
	Artist      string // for music and music videos
	Album       string // for music
	Song        string // for music videos
	DiscNumber  *int   // for music (DxxTxx)
	TrackNumber *int   // for music (DxxTxx)
	Season      int    // for TV shows
	Episode     int    // for TV shows
	PartNumber  *int   // for multi-part movies (CD-x, DISC-x, PART-x)
	PartType    string // "CD", "DISC", or "PART"
	BaseTitle   string // full base name without part indicator (for multi-part grouping)
}

// multiPartEntry tracks a media item that is part of a multi-part set.
type multiPartEntry struct {
	ItemID     uuid.UUID
	PartNumber int
}

// ──────────────────── Filename Parsing Patterns ────────────────────

// Movie: name (year) {edition} [resolution/container]
// Example: Aliens (1989) {Director's Cut} [1080p/MKV]
var movieFilenamePattern = regexp.MustCompile(
	`(?i)^(.+?)\s*\((\d{4})\)\s*(?:\{([^}]+)\})?\s*(?:\[([^/\]]+)/([^\]]+)\])?\s*$`)

// Adult movie: XXX - name (year) {edition} [resolution/container]
// Example: XXX - Debbie Does Dallas (1979) {Anniversary Edition} [1080p/MP4]
var adultMovieFilenamePattern = regexp.MustCompile(
	`(?i)^XXX\s+-\s+(.+?)\s*\((\d{4})\)\s*(?:\{([^}]+)\})?\s*(?:\[([^/\]]+)/([^\]]+)\])?\s*$`)

// TV show: name - SxxxExxx
// Example: The Big Bang Theory - S01E01
var tvShowFilenamePattern = regexp.MustCompile(
	`(?i)^(.+?)\s+-\s+S(\d{1,3})E(\d{1,3})\s*$`)

// Music: artist - album - DxxxTxxx
// Example: Metallica - Black Album - D01T02
var musicFilenamePattern = regexp.MustCompile(
	`(?i)^(.+?)\s+-\s+(.+?)\s+-\s+D(\d{1,3})T(\d{1,3})\s*$`)

// Music video: artist - song
// Example: Cyndi Lauper - Time After Time
var musicVideoFilenamePattern = regexp.MustCompile(
	`^(.+?)\s+-\s+(.+)$`)

// Multi-part indicator: CD-x, DISC-x, PART-x at end of base name
// Matches: " DISC-1", " CD2", " PART-3", etc.
var multiPartPattern = regexp.MustCompile(
	`(?i)\s+(CD|DISC|PART)-?(\d+)\s*$`)

// Edition prefix: strips "edition-" from Radarr/Sonarr convention
var editionPrefixPattern = regexp.MustCompile(`(?i)^edition-`)

// ──────────────────── Main Parser ────────────────────

// parseFilename extracts metadata from a media filename based on the library's media type.
// It applies type-specific patterns to pre-fill fields for metadata matching.
func (s *Scanner) parseFilename(filename string, mediaType models.MediaType) ParsedFilename {
	result := ParsedFilename{
		Edition: "Theatrical",
	}

	// Strip file extension
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)

	// Step 1: For movies/adult movies, check for multi-part indicator and strip it
	if mediaType == models.MediaTypeMovies || mediaType == models.MediaTypeAdultMovies {
		if matches := multiPartPattern.FindStringSubmatch(baseName); len(matches) >= 3 {
			partNum, _ := strconv.Atoi(matches[2])
			result.PartNumber = &partNum
			result.PartType = strings.ToUpper(matches[1])
			baseName = multiPartPattern.ReplaceAllString(baseName, "")
			baseName = strings.TrimSpace(baseName)
		}
	}

	// Step 2: Apply type-specific patterns
	switch mediaType {
	case models.MediaTypeAdultMovies:
		result = s.parseAdultMovieFilename(baseName, result)
	case models.MediaTypeMovies:
		result = s.parseMovieFilename(baseName, result)
	case models.MediaTypeTVShows:
		result = s.parseTVShowFilename(baseName, result)
	case models.MediaTypeMusic:
		result = s.parseMusicFilename(baseName, result)
	case models.MediaTypeMusicVideos:
		result = s.parseMusicVideoFilename(baseName, result)
	default:
		// For other types (home videos, other videos, images, audiobooks),
		// use the existing generic title extraction
		result.Title = s.titleFromFilename(filename)
		result.Year = s.extractYear(filename)
		result.Edition = s.extractEdition(filename)
	}

	// Step 3: Set BaseTitle for multi-part grouping (the cleaned base name without part indicator)
	if result.PartNumber != nil {
		result.BaseTitle = baseName
	}

	return result
}

// ──────────────────── Type-Specific Parsers ────────────────────

// parseMovieFilename extracts metadata from movie filenames.
// Pattern: name (year) {edition} [resolution/container]
func (s *Scanner) parseMovieFilename(baseName string, result ParsedFilename) ParsedFilename {
	matches := movieFilenamePattern.FindStringSubmatch(baseName)
	if len(matches) >= 3 {
		result.Title = strings.TrimSpace(matches[1])

		year, err := strconv.Atoi(matches[2])
		if err == nil && year >= 1900 && year <= 2100 {
			result.Year = &year
		}

		if len(matches) >= 4 && matches[3] != "" {
			result.Edition = cleanEditionName(matches[3])
		}

		if len(matches) >= 5 && matches[4] != "" {
			result.Resolution = strings.TrimSpace(matches[4])
		}

		if len(matches) >= 6 && matches[5] != "" {
			result.Container = strings.ToLower(strings.TrimSpace(matches[5]))
		}

		log.Printf("Filename parse (movie): %q → title=%q year=%v edition=%q res=%q container=%q",
			baseName, result.Title, result.Year, result.Edition, result.Resolution, result.Container)
	} else {
		// Fallback to generic parsing for non-standard naming
		result.Title = s.titleFromFilename(baseName + ".tmp")
		result.Year = s.extractYear(baseName)
		result.Edition = s.extractEdition(baseName)
	}
	return result
}

// parseAdultMovieFilename extracts metadata from adult movie filenames.
// Pattern: XXX - name (year) {edition} [resolution/container]
func (s *Scanner) parseAdultMovieFilename(baseName string, result ParsedFilename) ParsedFilename {
	matches := adultMovieFilenamePattern.FindStringSubmatch(baseName)
	if len(matches) >= 3 {
		result.Title = strings.TrimSpace(matches[1])

		year, err := strconv.Atoi(matches[2])
		if err == nil && year >= 1900 && year <= 2100 {
			result.Year = &year
		}

		if len(matches) >= 4 && matches[3] != "" {
			result.Edition = cleanEditionName(matches[3])
		}

		if len(matches) >= 5 && matches[4] != "" {
			result.Resolution = strings.TrimSpace(matches[4])
		}

		if len(matches) >= 6 && matches[5] != "" {
			result.Container = strings.ToLower(strings.TrimSpace(matches[5]))
		}

		log.Printf("Filename parse (adult): %q → title=%q year=%v edition=%q res=%q container=%q",
			baseName, result.Title, result.Year, result.Edition, result.Resolution, result.Container)
	} else {
		// Fallback: strip XXX prefix if present, then parse generically
		stripped := baseName
		xxxPrefix := regexp.MustCompile(`(?i)^XXX\s+-\s+`)
		if xxxPrefix.MatchString(baseName) {
			stripped = xxxPrefix.ReplaceAllString(baseName, "")
		}
		result.Title = s.titleFromFilename(stripped + ".tmp")
		result.Year = s.extractYear(stripped)
		result.Edition = s.extractEdition(stripped)
	}
	return result
}

// parseTVShowFilename extracts metadata from TV show filenames.
// Pattern: name - SxxxExxx
func (s *Scanner) parseTVShowFilename(baseName string, result ParsedFilename) ParsedFilename {
	matches := tvShowFilenamePattern.FindStringSubmatch(baseName)
	if len(matches) >= 4 {
		result.Title = strings.TrimSpace(matches[1])
		result.Season, _ = strconv.Atoi(matches[2])
		result.Episode, _ = strconv.Atoi(matches[3])

		log.Printf("Filename parse (tv): %q → title=%q S%02dE%02d",
			baseName, result.Title, result.Season, result.Episode)
	} else {
		// Fallback to generic parsing (handles SxxExx via existing tvPatterns in parseTVInfo)
		result.Title = s.titleFromFilename(baseName + ".tmp")
		result.Year = s.extractYear(baseName)
	}
	return result
}

// parseMusicFilename extracts metadata from music filenames.
// Pattern: artist - album - DxxxTxxx
func (s *Scanner) parseMusicFilename(baseName string, result ParsedFilename) ParsedFilename {
	matches := musicFilenamePattern.FindStringSubmatch(baseName)
	if len(matches) >= 5 {
		result.Artist = strings.TrimSpace(matches[1])
		result.Album = strings.TrimSpace(matches[2])

		disc, _ := strconv.Atoi(matches[3])
		result.DiscNumber = &disc
		track, _ := strconv.Atoi(matches[4])
		result.TrackNumber = &track

		// Title is the album name (used for metadata matching against MusicBrainz)
		result.Title = result.Album

		log.Printf("Filename parse (music): %q → artist=%q album=%q D%02dT%02d",
			baseName, result.Artist, result.Album, disc, track)
	} else {
		// Fallback to generic parsing
		result.Title = s.titleFromFilename(baseName + ".tmp")
	}
	return result
}

// parseMusicVideoFilename extracts metadata from music video filenames.
// Pattern: artist - song
func (s *Scanner) parseMusicVideoFilename(baseName string, result ParsedFilename) ParsedFilename {
	matches := musicVideoFilenamePattern.FindStringSubmatch(baseName)
	if len(matches) >= 3 {
		result.Artist = strings.TrimSpace(matches[1])
		result.Song = strings.TrimSpace(matches[2])

		// Title is the song name
		result.Title = result.Song

		log.Printf("Filename parse (music_video): %q → artist=%q song=%q",
			baseName, result.Artist, result.Song)
	} else {
		// No dash separator found, use full name as title
		result.Title = s.titleFromFilename(baseName + ".tmp")
	}
	return result
}

// ──────────────────── Helpers ────────────────────

// cleanEditionName normalizes edition names, stripping the "edition-" prefix
// from Radarr/Sonarr convention if present.
func cleanEditionName(raw string) string {
	name := strings.TrimSpace(raw)
	if editionPrefixPattern.MatchString(name) {
		name = editionPrefixPattern.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
	}
	if name == "" {
		return "Theatrical"
	}
	return name
}

// ──────────────────── Music Hierarchy ────────────────────

// handleMusicHierarchy finds or creates Artist and Album records from parsed filename data,
// and links them to the media item before it is persisted.
func (s *Scanner) handleMusicHierarchy(library *models.Library, item *models.MediaItem, parsed ParsedFilename) error {
	if parsed.Artist == "" || s.musicRepo == nil {
		return nil
	}

	// Find or create artist
	artist, err := s.musicRepo.FindArtistByName(library.ID, parsed.Artist)
	if err != nil {
		return err
	}
	if artist == nil {
		artist = &models.Artist{
			ID:        uuid.New(),
			LibraryID: library.ID,
			Name:      parsed.Artist,
		}
		if err := s.musicRepo.CreateArtist(artist); err != nil {
			return fmt.Errorf("create artist: %w", err)
		}
		log.Printf("Music hierarchy: created artist %q", parsed.Artist)
	}
	item.ArtistID = &artist.ID

	// Find or create album (music tracks only, not music videos)
	if parsed.Album != "" {
		album, err := s.musicRepo.FindAlbumByTitle(artist.ID, parsed.Album)
		if err != nil {
			return err
		}
		if album == nil {
			album = &models.Album{
				ID:        uuid.New(),
				ArtistID:  artist.ID,
				LibraryID: library.ID,
				Title:     parsed.Album,
			}
			if err := s.musicRepo.CreateAlbum(album); err != nil {
				return fmt.Errorf("create album: %w", err)
			}
			log.Printf("Music hierarchy: created album %q for artist %q", parsed.Album, parsed.Artist)
		}
		item.AlbumID = &album.ID
	}

	return nil
}

// ──────────────────── Multi-Part Grouping ────────────────────

// groupMultiPartFiles creates sister groups for multi-part files detected during the scan.
// Files sharing the same base title in the same directory are grouped together.
func (s *Scanner) groupMultiPartFiles() {
	if s.sisterRepo == nil {
		log.Printf("Multi-part: sister repository not available, skipping grouping")
		return
	}

	for key, parts := range s.pendingMultiParts {
		if len(parts) < 2 {
			continue
		}

		// Extract the base title from the grouping key (format: "dir|baseTitle")
		groupName := key
		if idx := strings.Index(key, "|"); idx >= 0 {
			groupName = key[idx+1:]
		}

		// Create a sister group for this multi-part set
		group := &models.SisterGroup{
			ID:   uuid.New(),
			Name: groupName,
		}
		if err := s.sisterRepo.Create(group); err != nil {
			log.Printf("Multi-part: failed to create sister group for %q: %v", groupName, err)
			continue
		}

		// Link all parts to the sister group
		for _, part := range parts {
			if err := s.sisterRepo.AddMember(group.ID, part.ItemID); err != nil {
				log.Printf("Multi-part: failed to add part %d (item %s) to group: %v",
					part.PartNumber, part.ItemID, err)
			}
		}

		log.Printf("Multi-part: grouped %d parts as %q (sister group %s)",
			len(parts), groupName, group.ID)
	}
}
