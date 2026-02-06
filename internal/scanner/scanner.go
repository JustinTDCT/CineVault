package scanner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/ffmpeg"
	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
)

type Scanner struct {
	ffprobe       *ffmpeg.FFprobe
	mediaRepo     *repository.MediaRepository
	tvRepo        *repository.TVRepository
	musicRepo     *repository.MusicRepository
	audiobookRepo *repository.AudiobookRepository
	galleryRepo   *repository.GalleryRepository
	scrapers      []metadata.Scraper
	posterDir     string
	// matchedShows tracks TV show IDs already matched this scan to avoid duplicate lookups
	matchedShows  map[uuid.UUID]bool
}

// Extension sets per media type
var videoExtensions = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
	".m4v": true, ".wmv": true, ".flv": true, ".webm": true,
	".ts": true, ".m2ts": true, ".mpg": true, ".mpeg": true,
}

var musicExtensions = map[string]bool{
	".mp3": true, ".flac": true, ".aac": true, ".ogg": true,
	".wav": true, ".m4a": true, ".alac": true, ".wma": true,
	".opus": true,
}

var audiobookExtensions = map[string]bool{
	".mp3": true, ".m4b": true, ".aac": true, ".flac": true,
}

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
}

// TV episode regex patterns
var tvPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(.+?)[.\s_-]+S(\d{1,2})E(\d{1,3})`),           // Show.S01E02
	regexp.MustCompile(`(?i)(.+?)[.\s_-]+(\d{1,2})x(\d{1,3})`),            // Show.1x02
	regexp.MustCompile(`(?i)(.+?)[/\\]Season\s*(\d{1,2})[/\\].*E(\d{1,3})`), // Show/Season 1/E02
}

// Year extraction patterns for filenames
var yearPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\((\d{4})\)`),          // Movie Title (2020)
	regexp.MustCompile(`\[(\d{4})\]`),          // Movie Title [2020]
	regexp.MustCompile(`[.\s_-](\d{4})[.\s_-]`), // Movie.Title.2020.1080p
}

func NewScanner(ffprobePath string, mediaRepo *repository.MediaRepository,
	tvRepo *repository.TVRepository, musicRepo *repository.MusicRepository,
	audiobookRepo *repository.AudiobookRepository, galleryRepo *repository.GalleryRepository,
	scrapers []metadata.Scraper, posterDir string,
) *Scanner {
	return &Scanner{
		ffprobe:       ffmpeg.NewFFprobe(ffprobePath),
		mediaRepo:     mediaRepo,
		tvRepo:        tvRepo,
		musicRepo:     musicRepo,
		audiobookRepo: audiobookRepo,
		galleryRepo:   galleryRepo,
		scrapers:      scrapers,
		posterDir:     posterDir,
		matchedShows:  make(map[uuid.UUID]bool),
	}
}

func (s *Scanner) ScanLibrary(library *models.Library) (*models.ScanResult, error) {
	result := &models.ScanResult{}

	err := filepath.Walk(library.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !s.isValidExtension(library.MediaType, ext) {
			return nil
		}
		result.FilesFound++

		// Check if file already scanned
		existing, err := s.mediaRepo.GetByFilePath(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("db check failed for %s: %v", path, err))
			return nil
		}
		if existing != nil {
			result.FilesSkipped++
			return nil
		}

		// Create media item
		item := &models.MediaItem{
			ID:        uuid.New(),
			LibraryID: library.ID,
			MediaType: library.MediaType,
			FilePath:  path,
			FileName:  info.Name(),
			FileSize:  info.Size(),
			Title:     s.titleFromFilename(info.Name()),
			Year:      s.extractYear(info.Name()),
		}

		// Probe with ffprobe for video/audio types
		if s.isProbeableType(library.MediaType) {
			probe, probeErr := s.ffprobe.Probe(path)
			if probeErr != nil {
				log.Printf("ffprobe failed for %s: %v", path, probeErr)
				result.Errors = append(result.Errors, fmt.Sprintf("probe failed: %s", path))
			} else {
				s.applyProbeData(item, probe)
			}
		}

		// Handle TV show hierarchy
		if library.MediaType == models.MediaTypeTVShows {
			if err := s.handleTVHierarchy(library, item, path); err != nil {
				log.Printf("TV hierarchy error for %s: %v", path, err)
			}
		}

		// Persist
		if err := s.mediaRepo.Create(item); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("insert failed %s: %v", path, err))
			return nil
		}

		// If TV, increment season episode count
		if item.TVSeasonID != nil {
			_ = s.tvRepo.IncrementEpisodeCount(*item.TVSeasonID)
		}

		// Auto-populate metadata from external sources
		s.autoPopulateMetadata(library, item)

		result.FilesAdded++
		return nil
	})

	// Reset matched shows tracker for next scan
	s.matchedShows = make(map[uuid.UUID]bool)

	return result, err
}

func (s *Scanner) isValidExtension(mediaType models.MediaType, ext string) bool {
	switch mediaType {
	case models.MediaTypeMovies, models.MediaTypeAdultMovies, models.MediaTypeTVShows,
		models.MediaTypeMusicVideos, models.MediaTypeHomeVideos, models.MediaTypeOtherVideos:
		return videoExtensions[ext]
	case models.MediaTypeMusic:
		return musicExtensions[ext]
	case models.MediaTypeAudiobooks:
		return audiobookExtensions[ext]
	case models.MediaTypeImages:
		return imageExtensions[ext]
	default:
		return videoExtensions[ext]
	}
}

func (s *Scanner) isProbeableType(mediaType models.MediaType) bool {
	return mediaType != models.MediaTypeImages
}

func (s *Scanner) applyProbeData(item *models.MediaItem, probe *ffmpeg.ProbeResult) {
	dur := probe.GetDurationSeconds()
	if dur > 0 {
		item.DurationSeconds = &dur
	}
	res := probe.GetResolution()
	if res != "" {
		item.Resolution = &res
	}
	w := probe.GetWidth()
	if w > 0 {
		item.Width = &w
	}
	h := probe.GetHeight()
	if h > 0 {
		item.Height = &h
	}
	vc := probe.GetVideoCodec()
	if vc != "" {
		item.Codec = &vc
	}
	ac := probe.GetAudioCodec()
	if ac != "" {
		item.AudioCodec = &ac
	}
	br := probe.GetBitrate()
	if br > 0 {
		item.Bitrate = &br
	}
	// container from file extension
	ext := strings.TrimPrefix(filepath.Ext(item.FilePath), ".")
	if ext != "" {
		item.Container = &ext
	}
}

func (s *Scanner) handleTVHierarchy(library *models.Library, item *models.MediaItem, path string) error {
	// Try to parse show name, season, episode from path
	relPath, _ := filepath.Rel(library.Path, path)
	showName, seasonNum, episodeNum := s.parseTVInfo(relPath)

	if showName == "" {
		// Fallback: use top-level directory name as show name
		parts := strings.Split(relPath, string(filepath.Separator))
		if len(parts) > 1 {
			showName = parts[0]
		}
	}

	if showName == "" {
		return nil
	}

	// Find or create show
	show, err := s.tvRepo.FindShowByTitle(library.ID, showName)
	if err != nil {
		return err
	}
	if show == nil {
		show = &models.TVShow{
			ID:        uuid.New(),
			LibraryID: library.ID,
			Title:     showName,
		}
		if err := s.tvRepo.CreateShow(show); err != nil {
			return fmt.Errorf("create show: %w", err)
		}
	}
	item.TVShowID = &show.ID

	if seasonNum > 0 {
		season, err := s.tvRepo.FindSeason(show.ID, seasonNum)
		if err != nil {
			return err
		}
		if season == nil {
			season = &models.TVSeason{
				ID:           uuid.New(),
				TVShowID:     show.ID,
				SeasonNumber: seasonNum,
			}
			if err := s.tvRepo.CreateSeason(season); err != nil {
				return fmt.Errorf("create season: %w", err)
			}
		}
		item.TVSeasonID = &season.ID
	}

	if episodeNum > 0 {
		item.EpisodeNumber = &episodeNum
	}

	return nil
}

func (s *Scanner) parseTVInfo(relPath string) (showName string, season, episode int) {
	for _, pattern := range tvPatterns {
		matches := pattern.FindStringSubmatch(relPath)
		if len(matches) >= 4 {
			showName = s.cleanShowName(matches[1])
			season, _ = strconv.Atoi(matches[2])
			episode, _ = strconv.Atoi(matches[3])
			return
		}
	}

	// Try directory-based: Show/Season N/file
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) >= 3 {
		showName = parts[0]
		seasonPart := strings.ToLower(parts[1])
		seasonPart = strings.TrimPrefix(seasonPart, "season ")
		seasonPart = strings.TrimPrefix(seasonPart, "season")
		seasonPart = strings.TrimSpace(seasonPart)
		if s, err := strconv.Atoi(seasonPart); err == nil {
			season = s
		}
	}
	return
}

func (s *Scanner) cleanShowName(name string) string {
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.TrimSpace(name)
	return name
}

func (s *Scanner) titleFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return strings.TrimSpace(name)
}

// ──────────────────── Auto Metadata Population ────────────────────

// autoPopulateMetadata searches external sources and applies the best match.
func (s *Scanner) autoPopulateMetadata(library *models.Library, item *models.MediaItem) {
	if len(s.scrapers) == 0 || !metadata.ShouldAutoMatch(item.MediaType) {
		return
	}

	// For TV shows, match at the show level (not per-episode)
	if item.MediaType == models.MediaTypeTVShows && item.TVShowID != nil {
		s.autoMatchTVShow(*item.TVShowID)
		return
	}

	// Build search query from cleaned title
	searchQuery := metadata.CleanTitleForSearch(item.Title)
	if searchQuery == "" {
		return
	}

	match := metadata.FindBestMatch(s.scrapers, searchQuery, item.MediaType)
	if match == nil {
		log.Printf("Auto-match: no match for %q", searchQuery)
		return
	}

	log.Printf("Auto-match: %q → %q (source=%s, confidence=%.2f)",
		item.Title, match.Title, match.Source, match.Confidence)

	// Download poster if available
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" {
		filename := item.ID.String() + ".jpg"
		saved, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: poster download failed for %s: %v", item.ID, err)
		} else {
			// Store as web-accessible path relative to preview root
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
			_ = saved
		}
	}

	// Update the media item with matched metadata
	if err := s.mediaRepo.UpdateMetadata(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath); err != nil {
		log.Printf("Auto-match: DB update failed for %s: %v", item.ID, err)
	}
}

// autoMatchTVShow searches for a TV show and applies metadata to the show record.
// Only runs once per show per scan.
func (s *Scanner) autoMatchTVShow(showID uuid.UUID) {
	if s.matchedShows[showID] {
		return
	}
	s.matchedShows[showID] = true

	show, err := s.tvRepo.GetShowByID(showID)
	if err != nil {
		return
	}

	// Skip if show already has metadata (description populated)
	if show.Description != nil && *show.Description != "" {
		return
	}

	searchQuery := metadata.CleanTitleForSearch(show.Title)
	if searchQuery == "" {
		return
	}

	match := metadata.FindBestMatch(s.scrapers, searchQuery, models.MediaTypeTVShows)
	if match == nil {
		log.Printf("Auto-match: no TV match for %q", searchQuery)
		return
	}

	log.Printf("Auto-match TV: %q → %q (source=%s, confidence=%.2f)",
		show.Title, match.Title, match.Source, match.Confidence)

	// Download poster for the show
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" {
		filename := "tvshow_" + showID.String() + ".jpg"
		saved, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: TV poster download failed for %s: %v", showID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
			_ = saved
		}
	}

	if err := s.tvRepo.UpdateShowMetadata(showID, match.Title, match.Year,
		match.Description, match.Rating, posterPath); err != nil {
		log.Printf("Auto-match: TV show DB update failed for %s: %v", showID, err)
	}
}

// extractYear tries to find a 4-digit year in a filename.
func (s *Scanner) extractYear(filename string) *int {
	for _, pattern := range yearPatterns {
		matches := pattern.FindStringSubmatch(filename)
		if len(matches) >= 2 {
			year, err := strconv.Atoi(matches[1])
			if err == nil && year >= 1900 && year <= 2100 {
				return &year
			}
		}
	}
	return nil
}
