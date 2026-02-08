package scanner

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
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
	tagRepo       *repository.TagRepository
	performerRepo *repository.PerformerRepository
	settingsRepo  *repository.SettingsRepository
	scrapers      []metadata.Scraper
	posterDir     string
	// matchedShows tracks TV show IDs already matched this scan to avoid duplicate lookups
	matchedShows  map[uuid.UUID]bool
	// pendingEpisodeMeta tracks show IDs → TMDB external IDs for post-scan episode metadata fetch
	pendingEpisodeMeta map[uuid.UUID]string
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
	tagRepo *repository.TagRepository, performerRepo *repository.PerformerRepository,
	settingsRepo *repository.SettingsRepository,
	scrapers []metadata.Scraper, posterDir string,
) *Scanner {
	return &Scanner{
		ffprobe:            ffmpeg.NewFFprobe(ffprobePath),
		mediaRepo:          mediaRepo,
		tvRepo:             tvRepo,
		musicRepo:          musicRepo,
		audiobookRepo:      audiobookRepo,
		galleryRepo:        galleryRepo,
		tagRepo:            tagRepo,
		performerRepo:      performerRepo,
		settingsRepo:       settingsRepo,
		scrapers:           scrapers,
		posterDir:          posterDir,
		matchedShows:       make(map[uuid.UUID]bool),
		pendingEpisodeMeta: make(map[uuid.UUID]string),
	}
}

// ProgressFunc reports scan progress: current processed count, total eligible files, current filename.
type ProgressFunc func(current, total int, filename string)

func (s *Scanner) ScanLibrary(library *models.Library, progressFn ...ProgressFunc) (*models.ScanResult, error) {
	result := &models.ScanResult{}

	// Determine which folders to scan: use library.Folders if available, else fall back to library.Path
	scanPaths := []string{}
	if len(library.Folders) > 0 {
		for _, f := range library.Folders {
			if f.FolderPath != "" {
				scanPaths = append(scanPaths, f.FolderPath)
			}
		}
	}
	if len(scanPaths) == 0 && library.Path != "" {
		scanPaths = []string{library.Path}
	}

	// Pre-count eligible files for progress reporting
	var onProgress ProgressFunc
	if len(progressFn) > 0 && progressFn[0] != nil {
		onProgress = progressFn[0]
	}
	totalFiles := 0
	if onProgress != nil {
		totalFiles = s.countEligibleFiles(library.MediaType, scanPaths)
		log.Printf("Scan: pre-count found %d eligible files", totalFiles)
	}
	processed := 0

	// Determine if metadata should be retrieved for this library
	shouldRetrieveMetadata := library.RetrieveMetadata
	// Adult clips: never scrape metadata regardless of library setting
	if library.MediaType == models.MediaTypeAdultMovies && library.AdultContentType != nil && *library.AdultContentType == "clips" {
		shouldRetrieveMetadata = false
	}

	for _, scanPath := range scanPaths {
		log.Printf("Scanning folder: %s", scanPath)
		err := filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !s.isValidExtension(library.MediaType, ext) {
				return nil
			}
			result.FilesFound++
			processed++

			// Report progress
			if onProgress != nil {
				onProgress(processed, totalFiles, info.Name())
			}

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

			// Handle TV show hierarchy (only when season grouping is enabled)
			if library.MediaType == models.MediaTypeTVShows && library.SeasonGrouping {
				if err := s.handleTVHierarchy(library, item, path, scanPath); err != nil {
					log.Printf("TV hierarchy error for %s: %v", path, err)
				}
			}

			// Persist
			if err := s.mediaRepo.Create(item); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("insert failed %s: %v", path, err))
				return nil
			}

			// If TV with season grouping, increment season episode count
			if library.SeasonGrouping && item.TVSeasonID != nil {
				_ = s.tvRepo.IncrementEpisodeCount(*item.TVSeasonID)
			}

			// Compute MD5 hash and check for exact duplicates
			if md5Hash, err := computeMD5(path); err == nil {
				if err := s.mediaRepo.UpdateFileHash(item.ID, md5Hash); err != nil {
					log.Printf("MD5: failed to store hash for %s: %v", path, err)
				} else {
					item.FileHash = &md5Hash
					dupes, err := s.mediaRepo.FindByFileHash(md5Hash, item.ID)
					if err == nil && len(dupes) > 0 {
						_ = s.mediaRepo.UpdateDuplicateStatus(item.ID, "exact")
						for _, d := range dupes {
							if d.DuplicateStatus != "addressed" {
								_ = s.mediaRepo.UpdateDuplicateStatus(d.ID, "exact")
							}
						}
						log.Printf("MD5: exact duplicate found for %s (%d matches)", item.FileName, len(dupes))
					}
				}
			} else {
				log.Printf("MD5: failed to hash %s: %v", path, err)
			}

			// Auto-populate metadata from external sources (if enabled)
			if shouldRetrieveMetadata {
				s.autoPopulateMetadata(library, item)
			}

			result.FilesAdded++
			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("walk error for %s: %v", scanPath, err))
		}
	}

	// Post-scan: fetch episode-level metadata for matched TV shows
	if shouldRetrieveMetadata && len(s.pendingEpisodeMeta) > 0 {
		log.Printf("Fetching episode metadata for %d TV show(s)...", len(s.pendingEpisodeMeta))
		for showID, tmdbID := range s.pendingEpisodeMeta {
			s.fetchEpisodeMetadata(showID, tmdbID)
		}
	}

	// Post-scan: re-enrich existing items that are missing OMDb ratings or cast
	if shouldRetrieveMetadata && len(s.scrapers) > 0 {
		s.reEnrichExistingItems(library, onProgress)
	}

	// Reset trackers for next scan
	s.matchedShows = make(map[uuid.UUID]bool)
	s.pendingEpisodeMeta = make(map[uuid.UUID]string)

	return result, nil
}

// reEnrichExistingItems finds items in the library that were previously TMDB-matched
// but are missing OMDb ratings or cast/crew, and re-enriches them.
func (s *Scanner) reEnrichExistingItems(library *models.Library, onProgress ProgressFunc) {
	items, err := s.mediaRepo.ListItemsNeedingEnrichment(library.ID)
	if err != nil {
		log.Printf("Re-enrich: failed to list items: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}

	log.Printf("Re-enrich: %d items need OMDb ratings or cast enrichment", len(items))
	if onProgress != nil {
		onProgress(0, len(items), "Enriching metadata...")
	}

	for i, item := range items {
		if item.MetadataLocked {
			continue
		}

		// For TV shows with season grouping, skip per-episode enrichment (handled at show level)
		if item.MediaType == models.MediaTypeTVShows && item.TVShowID != nil && library.SeasonGrouping {
			continue
		}

		// Search TMDB to get the external ID for this item
		searchQuery := metadata.CleanTitleForSearch(item.Title)
		if searchQuery == "" {
			continue
		}

		match := metadata.FindBestMatch(s.scrapers, searchQuery, item.MediaType, item.Year)
		if match == nil || match.Source != "tmdb" || match.ExternalID == "" {
			continue
		}

		log.Printf("Re-enrich: %q → TMDB ID %s", item.Title, match.ExternalID)
		s.enrichWithDetails(item.ID, match.ExternalID, item.MediaType)

		if onProgress != nil && (i+1)%5 == 0 || i+1 == len(items) {
			onProgress(i+1, len(items), item.Title)
		}
	}

	log.Printf("Re-enrich: completed for %d items", len(items))
}

// MediaRepo exposes the media repository for post-scan jobs (e.g. phash).
func (s *Scanner) MediaRepo() *repository.MediaRepository {
	return s.mediaRepo
}

// computeMD5 returns the hex-encoded MD5 hash of a file.
func computeMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// countEligibleFiles walks the scan paths and counts files with valid extensions.
func (s *Scanner) countEligibleFiles(mediaType models.MediaType, scanPaths []string) int {
	count := 0
	for _, scanPath := range scanPaths {
		_ = filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if s.isValidExtension(mediaType, ext) {
				count++
			}
			return nil
		})
	}
	return count
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

func (s *Scanner) handleTVHierarchy(library *models.Library, item *models.MediaItem, path string, basePath ...string) error {
	// Try to parse show name, season, episode from path
	base := library.Path
	if len(basePath) > 0 && basePath[0] != "" {
		base = basePath[0]
	}
	relPath, _ := filepath.Rel(base, path)
	showName, seasonNum, episodeNum := s.parseTVInfo(relPath, base)

	if showName == "" {
		return nil
	}

	log.Printf("TV parse: show=%q season=%d episode=%d (from %s)", showName, seasonNum, episodeNum, relPath)

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

// seasonDirPattern matches "Season N" or "Season NN" directory names
var seasonDirPattern = regexp.MustCompile(`(?i)^season\s*(\d+)$`)

func (s *Scanner) parseTVInfo(relPath string, libraryPath string) (showName string, season, episode int) {
	parts := strings.Split(relPath, string(filepath.Separator))
	filename := parts[len(parts)-1]

	// --- Step 1: Extract season and episode from filename using SxxExx patterns ---
	for _, pattern := range tvPatterns {
		matches := pattern.FindStringSubmatch(filename)
		if len(matches) >= 4 {
			season, _ = strconv.Atoi(matches[2])
			episode, _ = strconv.Atoi(matches[3])
			break
		}
	}

	// --- Step 2: Determine show name from directory structure ---
	// Structure A: Show/Season N/file.mkv  → relPath has 3+ parts, show = parts[0]
	// Structure B: Season N/file.mkv        → library IS the show folder, use parent dir name
	// Structure C: file.mkv                 → library IS the show folder, use parent dir name

	if len(parts) >= 3 {
		// Show/Season N/file.mkv
		showName = parts[0]
	} else if len(parts) == 2 {
		// Check if first dir is "Season N" → library path is the show
		if seasonDirPattern.MatchString(parts[0]) {
			showName = filepath.Base(libraryPath)
			// Also extract season from directory if not from filename
			if season == 0 {
				if m := seasonDirPattern.FindStringSubmatch(parts[0]); len(m) >= 2 {
					season, _ = strconv.Atoi(m[1])
				}
			}
		} else {
			// First part is show name
			showName = parts[0]
		}
	} else {
		// Just a file directly in library root → library name is the show
		showName = filepath.Base(libraryPath)
	}

	// --- Step 3: If season still unknown, check directory parts for "Season N" ---
	if season == 0 {
		for _, part := range parts[:len(parts)-1] {
			if m := seasonDirPattern.FindStringSubmatch(part); len(m) >= 2 {
				season, _ = strconv.Atoi(m[1])
				break
			}
		}
	}

	showName = s.cleanShowName(showName)
	return
}

func (s *Scanner) cleanShowName(name string) string {
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	// Remove trailing year in parens/brackets: "Show Name (2020)" → "Show Name"
	name = regexp.MustCompile(`\s*[\(\[]\d{4}[\)\]]\s*$`).ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	return name
}

func (s *Scanner) titleFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Strip year in parens/brackets: "Title - (2020)" → "Title -"
	name = regexp.MustCompile(`[\(\[\{]\d{4}[\)\]\}]`).ReplaceAllString(name, "")
	// Strip anything in square brackets: "[Bluray-1080p x265]" etc.
	name = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(name, "")
	// Strip resolution, codec, and release junk tokens
	name = regexp.MustCompile(`(?i)\b(1080p|720p|480p|2160p|4k|uhd|bluray|blu-ray|brrip|bdrip|dvdrip|webrip|web-dl|webdl|hdtv|hdrip|x264|x265|h264|h265|hevc|aac|ac3|dts|atmos|remux|proper|repack|extended|unrated|directors cut|dc)\b`).ReplaceAllString(name, "")
	// Strip trailing dash/whitespace separator: "Title -  " → "Title"
	name = regexp.MustCompile(`\s*-\s*$`).ReplaceAllString(name, "")
	// Collapse multiple spaces
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	return strings.TrimSpace(name)
}

// ──────────────────── Auto Metadata Population ────────────────────

// autoPopulateMetadata searches external sources and applies the best match.
func (s *Scanner) autoPopulateMetadata(library *models.Library, item *models.MediaItem) {
	if len(s.scrapers) == 0 || !metadata.ShouldAutoMatch(item.MediaType) {
		return
	}

	// Skip items where the user has manually edited metadata
	if item.MetadataLocked {
		log.Printf("Auto-match: skipping %s (metadata locked by user edit)", item.ID)
		return
	}

	// For TV shows with season grouping, match at the show level (not per-episode)
	if item.MediaType == models.MediaTypeTVShows && item.TVShowID != nil {
		s.autoMatchTVShow(*item.TVShowID)
		return
	}
	// TV shows without season grouping fall through to per-item matching below

	// Build search query from cleaned title
	searchQuery := metadata.CleanTitleForSearch(item.Title)
	if searchQuery == "" {
		return
	}

	match := metadata.FindBestMatch(s.scrapers, searchQuery, item.MediaType, item.Year)
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

	// Get TMDB details for genres, IMDB ID, OMDb ratings, and cast
	if match.Source == "tmdb" {
		s.enrichWithDetails(item.ID, match.ExternalID, item.MediaType)
	}
}

// enrichWithDetails fetches TMDB details, creates genre tags, fetches OMDb ratings, and populates cast.
func (s *Scanner) enrichWithDetails(itemID uuid.UUID, tmdbExternalID string, mediaType models.MediaType) {
	// Find the TMDB scraper
	var tmdbScraper *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdbScraper = t
			break
		}
	}
	if tmdbScraper == nil {
		return
	}

	// Fetch details (movie vs TV have different endpoints)
	var details *models.MetadataMatch
	var err error
	if mediaType == models.MediaTypeTVShows {
		details, err = tmdbScraper.GetTVDetails(tmdbExternalID)
	} else {
		details, err = tmdbScraper.GetDetails(tmdbExternalID)
	}
	if err != nil {
		log.Printf("Auto-match: TMDB details failed for %s: %v", tmdbExternalID, err)
		return
	}

	// Create/link genre tags
	if s.tagRepo != nil && len(details.Genres) > 0 {
		s.linkGenreTags(itemID, details.Genres)
	}

	// Fetch OMDb ratings if key is configured
	if s.settingsRepo != nil && details.IMDBId != "" {
		omdbKey, err := s.settingsRepo.Get("omdb_api_key")
		if err != nil {
			log.Printf("Auto-match: settings lookup failed: %v", err)
		} else if omdbKey != "" {
			ratings, err := metadata.FetchOMDbRatings(details.IMDBId, omdbKey)
			if err != nil {
				log.Printf("Auto-match: OMDb fetch failed for %s: %v", details.IMDBId, err)
			} else {
				if err := s.mediaRepo.UpdateRatings(itemID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore); err != nil {
					log.Printf("Auto-match: ratings update failed for %s: %v", itemID, err)
				}
			}
		}
	}

	// Fetch and populate cast/crew from TMDB credits
	if s.performerRepo != nil {
		var credits *metadata.TMDBCredits
		if mediaType == models.MediaTypeTVShows {
			credits, err = tmdbScraper.GetTVCredits(tmdbExternalID)
		} else {
			credits, err = tmdbScraper.GetMovieCredits(tmdbExternalID)
		}
		if err != nil {
			log.Printf("Auto-match: TMDB credits failed for %s: %v", tmdbExternalID, err)
		} else {
			s.enrichWithCredits(itemID, credits)
		}
	}
}

// enrichWithCredits creates or finds performers from TMDB credits and links them to a media item.
// Imports top 20 cast members and key crew (Director, Producer, Writer).
func (s *Scanner) enrichWithCredits(itemID uuid.UUID, credits *metadata.TMDBCredits) {
	if credits == nil {
		return
	}

	// Import cast (top 20)
	maxCast := 20
	if len(credits.Cast) < maxCast {
		maxCast = len(credits.Cast)
	}
	for i := 0; i < maxCast; i++ {
		member := credits.Cast[i]
		if member.Name == "" {
			continue
		}
		performer, err := s.findOrCreatePerformer(member.Name, models.PerformerActor, member.ProfilePath)
		if err != nil {
			log.Printf("Auto-match: create performer %q failed: %v", member.Name, err)
			continue
		}
		charName := member.Character
		if err := s.performerRepo.LinkMedia(itemID, performer.ID, "actor", charName, member.Order); err != nil {
			log.Printf("Auto-match: link performer %q to %s failed: %v", member.Name, itemID, err)
		}
	}

	// Import key crew: Director, Producer, Writer
	importedCrew := 0
	for _, member := range credits.Crew {
		if member.Name == "" {
			continue
		}
		var perfType models.PerformerType
		switch member.Job {
		case "Director":
			perfType = models.PerformerDirector
		case "Producer", "Executive Producer":
			perfType = models.PerformerProducer
		case "Screenplay", "Writer", "Story":
			perfType = models.PerformerOther
		default:
			continue
		}
		performer, err := s.findOrCreatePerformer(member.Name, perfType, member.ProfilePath)
		if err != nil {
			log.Printf("Auto-match: create crew %q failed: %v", member.Name, err)
			continue
		}
		role := strings.ToLower(member.Job)
		if err := s.performerRepo.LinkMedia(itemID, performer.ID, role, "", 100+importedCrew); err != nil {
			log.Printf("Auto-match: link crew %q to %s failed: %v", member.Name, itemID, err)
		}
		importedCrew++
	}
}

// findOrCreatePerformer looks up an existing performer by name or creates a new one.
// If the performer exists but has no photo, and a profilePath is available, downloads the photo.
func (s *Scanner) findOrCreatePerformer(name string, perfType models.PerformerType, profilePath string) (*models.Performer, error) {
	existing, err := s.performerRepo.FindByName(name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Download photo if the performer doesn't have one yet
		if existing.PhotoPath == nil && profilePath != "" && s.posterDir != "" {
			photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
			filename := "performer_" + existing.ID.String() + ".jpg"
			if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.posterDir, "posters"), filename); dlErr == nil {
				webPath := "/previews/posters/" + filename
				existing.PhotoPath = &webPath
				_ = s.performerRepo.Update(existing)
			}
		}
		return existing, nil
	}

	// Create new performer
	p := &models.Performer{
		ID:            uuid.New(),
		Name:          name,
		PerformerType: perfType,
	}

	// Download profile photo from TMDB
	if profilePath != "" && s.posterDir != "" {
		photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
		filename := "performer_" + p.ID.String() + ".jpg"
		if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(s.posterDir, "posters"), filename); dlErr == nil {
			webPath := "/previews/posters/" + filename
			p.PhotoPath = &webPath
		}
	}

	if err := s.performerRepo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// linkGenreTags creates genre tags (if they don't exist) and links them to the media item.
func (s *Scanner) linkGenreTags(itemID uuid.UUID, genres []string) {
	for _, genre := range genres {
		// Look for existing tag with category=genre and matching name
		existing, _ := s.tagRepo.List("genre")
		var tagID uuid.UUID
		found := false
		for _, t := range existing {
			if strings.EqualFold(t.Name, genre) {
				tagID = t.ID
				found = true
				break
			}
		}
		if !found {
			// Create the genre tag
			tagID = uuid.New()
			tag := &models.Tag{
				ID:       tagID,
				Name:     genre,
				Category: models.TagCategoryGenre,
			}
			if err := s.tagRepo.Create(tag); err != nil {
				log.Printf("Auto-match: create genre tag %q failed: %v", genre, err)
				continue
			}
		}
		// Link tag to media item
		if err := s.tagRepo.AssignToMedia(itemID, tagID); err != nil {
			log.Printf("Auto-match: assign genre tag %q to %s failed: %v", genre, itemID, err)
		}
	}
}


// autoMatchTVShow searches for a TV show and applies metadata to the show record,
// then fetches episode-level metadata from TMDB for each season.
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
		// Still try to populate episode metadata if episodes lack it
		s.populateEpisodeMetadata(showID, show)
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

	log.Printf("Auto-match TV: %q → %q (source=%s, id=%s, confidence=%.2f)",
		show.Title, match.Title, match.Source, match.ExternalID, match.Confidence)

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

	// Enrich with genres and OMDb ratings using TMDB TV details
	if match.Source == "tmdb" && match.ExternalID != "" {
		s.enrichTVShowDetails(showID, match.ExternalID)
		// Queue episode-level metadata fetch for after all files are scanned
		s.pendingEpisodeMeta[showID] = match.ExternalID
	}
}

// enrichTVShowDetails fetches TV show details for genres and OMDb ratings,
// and applies them to all episodes in the show.
func (s *Scanner) enrichTVShowDetails(showID uuid.UUID, tmdbExternalID string) {
	var tmdbScraper *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdbScraper = t
			break
		}
	}
	if tmdbScraper == nil {
		return
	}

	details, err := tmdbScraper.GetTVDetails(tmdbExternalID)
	if err != nil {
		log.Printf("Auto-match: TMDB TV details failed for %s: %v", tmdbExternalID, err)
		return
	}

	// Get all episodes for this show to apply genres/ratings
	episodes, err := s.mediaRepo.ListByTVShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list episodes for genre/rating enrichment: %v", err)
		return
	}

	// Link genre tags to all episodes
	if s.tagRepo != nil && len(details.Genres) > 0 {
		for _, ep := range episodes {
			s.linkGenreTags(ep.ID, details.Genres)
		}
	}

	// Fetch OMDb ratings and apply to all episodes
	if s.settingsRepo != nil && details.IMDBId != "" {
		omdbKey, err := s.settingsRepo.Get("omdb_api_key")
		if err != nil {
			log.Printf("Auto-match: settings lookup failed: %v", err)
		} else if omdbKey != "" {
			ratings, err := metadata.FetchOMDbRatings(details.IMDBId, omdbKey)
			if err != nil {
				log.Printf("Auto-match: OMDb fetch failed for TV %s: %v", details.IMDBId, err)
			} else {
				for _, ep := range episodes {
					if err := s.mediaRepo.UpdateRatings(ep.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore); err != nil {
						log.Printf("Auto-match: ratings update failed for episode %s: %v", ep.ID, err)
					}
				}
			}
		}
	}

	// Fetch and link TV show cast to all episodes
	if s.performerRepo != nil {
		credits, err := tmdbScraper.GetTVCredits(tmdbExternalID)
		if err != nil {
			log.Printf("Auto-match: TMDB TV credits failed for %s: %v", tmdbExternalID, err)
		} else if len(episodes) > 0 {
			// Link cast to first episode as representative (avoid massive duplication)
			s.enrichWithCredits(episodes[0].ID, credits)
		}
	}
}

// fetchEpisodeMetadata uses the TMDB show ID to fetch season details and apply
// episode titles, descriptions, and still images to individual media items.
func (s *Scanner) fetchEpisodeMetadata(showID uuid.UUID, tmdbShowID string) {
	// Get the TMDB scraper
	var tmdb *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdb = t
			break
		}
	}
	if tmdb == nil {
		return
	}

	// Get all seasons for this show
	seasons, err := s.tvRepo.ListSeasonsByShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list seasons for show %s: %v", showID, err)
		return
	}

	// Get all episodes for this show
	episodes, err := s.mediaRepo.ListByTVShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list episodes for show %s: %v", showID, err)
		return
	}

	for _, season := range seasons {
		seasonData, err := tmdb.GetTVSeasonDetails(tmdbShowID, season.SeasonNumber)
		if err != nil {
			log.Printf("Auto-match: TMDB season %d fetch failed for show %s: %v", season.SeasonNumber, tmdbShowID, err)
			continue
		}

		// Download and save season poster
		if seasonData.PosterPath != "" && s.posterDir != "" {
			posterURL := "https://image.tmdb.org/t/p/w500" + seasonData.PosterPath
			filename := "season_" + season.ID.String() + ".jpg"
			if _, dlErr := metadata.DownloadPoster(posterURL, filepath.Join(s.posterDir, "posters"), filename); dlErr != nil {
				log.Printf("Auto-match: season poster download failed: %v", dlErr)
			} else {
				webPath := "/previews/posters/" + filename
				var title *string
				if seasonData.Name != "" {
					title = &seasonData.Name
				}
				var desc *string
				if seasonData.Overview != "" {
					desc = &seasonData.Overview
				}
				if err := s.tvRepo.UpdateSeasonMetadata(season.ID, title, desc, &webPath); err != nil {
					log.Printf("Auto-match: season metadata update failed: %v", err)
				} else {
					log.Printf("Auto-match season: S%02d poster saved", season.SeasonNumber)
				}
			}
		}

		// Build a map of TMDB episodes by episode number
		tmdbMap := make(map[int]metadata.TMDBEpisode)
		for _, ep := range seasonData.Episodes {
			tmdbMap[ep.EpisodeNumber] = ep
		}

		// Match local episodes to TMDB episodes
		for _, ep := range episodes {
			if ep.TVSeasonID == nil || *ep.TVSeasonID != season.ID || ep.EpisodeNumber == nil {
				continue
			}

			tmdbEp, ok := tmdbMap[*ep.EpisodeNumber]
			if !ok {
				continue
			}

			// Build episode title: "Episode Name" or keep original if TMDB has none
			epTitle := tmdbEp.Name
			if epTitle == "" {
				continue
			}

			var desc *string
			if tmdbEp.Overview != "" {
				desc = &tmdbEp.Overview
			}

			var rating *float64
			if tmdbEp.VoteAverage > 0 {
				rating = &tmdbEp.VoteAverage
			}

			// Download episode still image
			var posterPath *string
			if tmdbEp.StillPath != "" && s.posterDir != "" {
				stillURL := "https://image.tmdb.org/t/p/w500" + tmdbEp.StillPath
				filename := "ep_" + ep.ID.String() + ".jpg"
				saved, dlErr := metadata.DownloadPoster(stillURL, filepath.Join(s.posterDir, "posters"), filename)
				if dlErr != nil {
					log.Printf("Auto-match: episode still download failed: %v", dlErr)
				} else {
					webPath := "/previews/posters/" + filename
					posterPath = &webPath
					_ = saved
				}
			}

			if err := s.mediaRepo.UpdateMetadata(ep.ID, epTitle, nil, desc, rating, posterPath); err != nil {
				log.Printf("Auto-match: episode metadata update failed for %s: %v", ep.ID, err)
			} else {
				log.Printf("Auto-match episode: S%02dE%02d → %q", season.SeasonNumber, *ep.EpisodeNumber, epTitle)
			}
		}
	}
}

// populateEpisodeMetadata is called when the show already has metadata but episodes may not.
// It re-searches TMDB to get the show ID and queues episode metadata for post-scan.
func (s *Scanner) populateEpisodeMetadata(showID uuid.UUID, show *models.TVShow) {
	// Search TMDB to get the show's external ID
	searchQuery := metadata.CleanTitleForSearch(show.Title)
	if searchQuery == "" {
		return
	}
	match := metadata.FindBestMatch(s.scrapers, searchQuery, models.MediaTypeTVShows)
	if match == nil || match.Source != "tmdb" || match.ExternalID == "" {
		return
	}

	s.pendingEpisodeMeta[showID] = match.ExternalID
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
