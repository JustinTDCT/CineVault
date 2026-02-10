package scanner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/JustinTDCT/CineVault/internal/ffmpeg"
	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
)

type Scanner struct {
	ffprobe       *ffmpeg.FFprobe
	ffmpegPath    string
	mediaRepo     *repository.MediaRepository
	tvRepo        *repository.TVRepository
	musicRepo     *repository.MusicRepository
	audiobookRepo *repository.AudiobookRepository
	galleryRepo   *repository.GalleryRepository
	tagRepo       *repository.TagRepository
	performerRepo *repository.PerformerRepository
	settingsRepo  *repository.SettingsRepository
	sisterRepo    *repository.SisterRepository
	scrapers      []metadata.Scraper
	posterDir     string
	// matchedShows tracks TV show IDs already matched this scan to avoid duplicate lookups
	matchedShows  map[uuid.UUID]bool
	// pendingEpisodeMeta tracks show IDs → TMDB external IDs for post-scan episode metadata fetch
	pendingEpisodeMeta map[uuid.UUID]string
	// pendingMultiParts tracks multi-part files by "dir|baseTitle" for post-scan sister grouping
	pendingMultiParts map[string][]multiPartEntry
	// mu protects concurrent access during parallel enrichment
	mu sync.Mutex
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

// Edition extraction pattern: {edition-XXX} (Radarr/Sonarr convention)
var editionPattern = regexp.MustCompile(`(?i)\{edition-([^}]+)\}`)

func NewScanner(ffprobePath, ffmpegPath string, mediaRepo *repository.MediaRepository,
	tvRepo *repository.TVRepository, musicRepo *repository.MusicRepository,
	audiobookRepo *repository.AudiobookRepository, galleryRepo *repository.GalleryRepository,
	tagRepo *repository.TagRepository, performerRepo *repository.PerformerRepository,
	settingsRepo *repository.SettingsRepository, sisterRepo *repository.SisterRepository,
	scrapers []metadata.Scraper, posterDir string,
) *Scanner {
	return &Scanner{
		ffprobe:            ffmpeg.NewFFprobe(ffprobePath),
		ffmpegPath:         ffmpegPath,
		mediaRepo:          mediaRepo,
		tvRepo:             tvRepo,
		musicRepo:          musicRepo,
		audiobookRepo:      audiobookRepo,
		galleryRepo:        galleryRepo,
		tagRepo:            tagRepo,
		performerRepo:      performerRepo,
		settingsRepo:       settingsRepo,
		sisterRepo:         sisterRepo,
		scrapers:           scrapers,
		posterDir:          posterDir,
		matchedShows:       make(map[uuid.UUID]bool),
		pendingEpisodeMeta: make(map[uuid.UUID]string),
		pendingMultiParts:  make(map[string][]multiPartEntry),
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
				// Backfill: generate screenshot poster for existing items that don't have one
				if existing.PosterPath == nil && s.isProbeableType(library.MediaType) {
					s.generateScreenshotPoster(existing)
				}
				result.FilesSkipped++
				return nil
			}

			// Parse filename based on media type to pre-fill metadata
			parsed := s.parseFilename(info.Name(), library.MediaType)

			// Create media item with parsed metadata
			item := &models.MediaItem{
				ID:          uuid.New(),
				LibraryID:   library.ID,
				MediaType:   library.MediaType,
				FilePath:    path,
				FileName:    info.Name(),
				FileSize:    info.Size(),
				Title:       parsed.Title,
				Year:        parsed.Year,
				EditionType: parsed.Edition,
			}

			// Pre-fill resolution/container from filename (ffprobe will override if available)
			if parsed.Resolution != "" {
				item.Resolution = &parsed.Resolution
			}
			if parsed.Container != "" {
				item.Container = &parsed.Container
			}
			// Pre-fill music disc/track numbers
			if parsed.DiscNumber != nil {
				item.DiscNumber = parsed.DiscNumber
			}
			if parsed.TrackNumber != nil {
				item.TrackNumber = parsed.TrackNumber
			}
			// Set sort position for multi-part files (part number determines play order)
			if parsed.PartNumber != nil {
				item.SortPosition = *parsed.PartNumber
			}

			// Probe with ffprobe for video/audio types (overrides filename-parsed values)
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

			// Handle music hierarchy: find/create artist and album from parsed filename
			if (library.MediaType == models.MediaTypeMusic || library.MediaType == models.MediaTypeMusicVideos) && parsed.Artist != "" {
				if err := s.handleMusicHierarchy(library, item, parsed); err != nil {
					log.Printf("Music hierarchy error for %s: %v", path, err)
				}
			}

			// Persist
			if err := s.mediaRepo.Create(item); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("insert failed %s: %v", path, err))
				return nil
			}

			// Track multi-part files for post-scan sister grouping
			if parsed.PartNumber != nil && parsed.BaseTitle != "" {
				dir := filepath.Dir(path)
				key := dir + "|" + parsed.BaseTitle
				s.pendingMultiParts[key] = append(s.pendingMultiParts[key], multiPartEntry{
					ItemID:     item.ID,
					PartNumber: *parsed.PartNumber,
				})
			}

			// If TV with season grouping, increment season episode count
			if library.SeasonGrouping && item.TVSeasonID != nil {
				_ = s.tvRepo.IncrementEpisodeCount(*item.TVSeasonID)
			}

			// Auto-populate metadata from external sources (if enabled)
			if shouldRetrieveMetadata {
				s.autoPopulateMetadata(library, item)
			}

			// For items without metadata retrieval (or that didn't get a poster),
			// extract a screenshot from the video to use as the poster image.
			if item.PosterPath == nil && s.isProbeableType(library.MediaType) {
				s.generateScreenshotPoster(item)
			}

			result.FilesAdded++
			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("walk error for %s: %v", scanPath, err))
		}
	}

	// Post-scan: group multi-part files (CD-x, DISC-x, PART-x) into sister groups
	if len(s.pendingMultiParts) > 0 {
		s.groupMultiPartFiles()
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
	s.pendingMultiParts = make(map[string][]multiPartEntry)

	return result, nil
}

// reEnrichExistingItems finds items in the library that were previously TMDB-matched
// but are missing OMDb ratings or cast/crew, and re-enriches them using concurrent workers.
func (s *Scanner) reEnrichExistingItems(library *models.Library, onProgress ProgressFunc) {
	items, err := s.mediaRepo.ListItemsNeedingEnrichment(library.ID)
	if err != nil {
		log.Printf("Re-enrich: failed to list items: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}

	// Pre-fetch and cache the OMDb API key once
	var omdbKey string
	if s.settingsRepo != nil {
		omdbKey, _ = s.settingsRepo.Get("omdb_api_key")
	}

	// Find the TMDB scraper once
	var tmdbScraper *metadata.TMDBScraper
	for _, sc := range s.scrapers {
		if t, ok := sc.(*metadata.TMDBScraper); ok {
			tmdbScraper = t
			break
		}
	}
	if tmdbScraper == nil {
		log.Printf("Re-enrich: no TMDB scraper available")
		return
	}

	// Filter items upfront
	var enrichItems []*models.MediaItem
	for _, item := range items {
		if item.MetadataLocked {
			continue
		}
		if item.MediaType == models.MediaTypeTVShows && item.TVShowID != nil && library.SeasonGrouping {
			continue
		}
		enrichItems = append(enrichItems, item)
	}
	if len(enrichItems) == 0 {
		return
	}

	total := len(enrichItems)
	log.Printf("Re-enrich: %d items need OMDb ratings or cast enrichment (using 5 workers)", total)
	if onProgress != nil {
		onProgress(0, total, "Enriching metadata...")
	}

	// Concurrent worker pool
	const numWorkers = 5
	itemCh := make(chan *models.MediaItem, numWorkers*2)
	var wg sync.WaitGroup
	var processed int64

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemCh {
				s.enrichItemFast(item, tmdbScraper, omdbKey)
				cur := atomic.AddInt64(&processed, 1)
				if onProgress != nil && (cur%10 == 0 || int(cur) == total) {
					onProgress(int(cur), total, item.Title)
				}
			}
		}()
	}

	for _, item := range enrichItems {
		itemCh <- item
	}
	close(itemCh)
	wg.Wait()

	log.Printf("Re-enrich: completed %d items", total)
}

// enrichItemFast enriches a single item using the cache server or the combined
// TMDB details+credits endpoint, plus OMDb for ratings. Thread-safe for concurrent use.
func (s *Scanner) enrichItemFast(item *models.MediaItem, tmdbScraper *metadata.TMDBScraper, omdbKey string) {
	searchQuery := metadata.CleanTitleForSearch(item.Title)
	if searchQuery == "" {
		return
	}

	// ── Try cache server first ──
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		result := cacheClient.Lookup(searchQuery, item.Year, item.MediaType)
		if result != nil && result.Match != nil {
			log.Printf("Re-enrich: %q → %q (source=cache/%s)", item.Title, result.Match.Title, result.Source)

			// Download poster if cache provides one and current poster is a generated screenshot
			if item.GeneratedPoster && result.Match.PosterURL != nil && s.posterDir != "" {
				filename := item.ID.String() + ".jpg"
				posterDir := filepath.Join(s.posterDir, "posters")
				// Remove generated screenshot so dedup doesn't save TMDB poster as _alt
				_ = os.Remove(filepath.Join(posterDir, filename))
				_, err := metadata.DownloadPoster(*result.Match.PosterURL, posterDir, filename)
				if err != nil {
					log.Printf("Re-enrich: poster download failed for %s: %v", item.ID, err)
				} else {
					webPath := "/previews/posters/" + filename
					_ = s.mediaRepo.UpdatePosterPath(item.ID, webPath)
				}
			}

			// Link genre tags from cache
			if s.tagRepo != nil && len(result.Genres) > 0 {
				s.linkGenreTags(item.ID, result.Genres)
			}

			// Apply OMDb ratings from cache
			if result.Ratings != nil {
				_ = s.mediaRepo.UpdateRatings(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore)
			}

			// Use cast/crew from cache if available, otherwise fall back to TMDB
			if s.performerRepo != nil {
				if result.CastCrewJSON != nil && *result.CastCrewJSON != "" {
					credits := parseCacheCredits(*result.CastCrewJSON)
					if credits != nil {
						s.enrichWithCredits(item.ID, credits)
					}
				} else if result.Match.ExternalID != "" && tmdbScraper != nil {
					combined, err := tmdbScraper.GetDetailsWithCredits(result.Match.ExternalID)
					if err == nil && combined.Credits != nil {
						s.enrichWithCredits(item.ID, combined.Credits)
					}
				}
			}

			// Store external IDs from cache
			if result.ExternalIDsJSON != nil {
				_ = s.mediaRepo.UpdateExternalIDs(item.ID, *result.ExternalIDsJSON)
			}
			return
		}
	}

	match := metadata.FindBestMatch(s.scrapers, searchQuery, item.MediaType, item.Year)
	if match == nil || match.Source != "tmdb" || match.ExternalID == "" {
		return
	}

	log.Printf("Re-enrich: %q → TMDB ID %s", item.Title, match.ExternalID)

	// Use combined details+credits call (1 TMDB request instead of 2)
	combined, err := tmdbScraper.GetDetailsWithCredits(match.ExternalID)
	if err != nil {
		log.Printf("Re-enrich: TMDB details+credits failed for %s: %v", match.ExternalID, err)
		return
	}

	// Link genre tags
	if s.tagRepo != nil && len(combined.Details.Genres) > 0 {
		s.linkGenreTags(item.ID, combined.Details.Genres)
	}

	// Fetch OMDb ratings
	if omdbKey != "" && combined.Details.IMDBId != "" {
		ratings, err := metadata.FetchOMDbRatings(combined.Details.IMDBId, omdbKey)
		if err != nil {
			log.Printf("Re-enrich: OMDb fetch failed for %s: %v", combined.Details.IMDBId, err)
		} else {
			if err := s.mediaRepo.UpdateRatings(item.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore); err != nil {
				log.Printf("Re-enrich: ratings update failed for %s: %v", item.ID, err)
			}
		}
	}

	// Replace generated screenshot poster with TMDB poster
	if item.GeneratedPoster && combined.Details.PosterURL != nil && s.posterDir != "" {
		filename := item.ID.String() + ".jpg"
		pDir := filepath.Join(s.posterDir, "posters")
		_ = os.Remove(filepath.Join(pDir, filename))
		_, dlErr := metadata.DownloadPoster(*combined.Details.PosterURL, pDir, filename)
		if dlErr != nil {
			log.Printf("Re-enrich: poster download failed for %s: %v", item.ID, dlErr)
		} else {
			webPath := "/previews/posters/" + filename
			_ = s.mediaRepo.UpdatePosterPath(item.ID, webPath)
			log.Printf("Re-enrich: replaced generated poster for %q", item.Title)
		}
	}

	// Populate cast/crew from the credits already fetched
	if s.performerRepo != nil && combined.Credits != nil {
		s.enrichWithCredits(item.ID, combined.Credits)
	}

	// Store external IDs from direct TMDB match
	idsJSON := metadata.BuildExternalIDsFromMatch("tmdb", match.ExternalID, combined.Details.IMDBId, false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	// Contribute to cache server with cast/crew and ratings
	if cacheClient != nil {
		extras := metadata.ContributeExtras{}
		if combined.Credits != nil {
			creditsJSON, err := json.Marshal(combined.Credits)
			if err == nil {
				s := string(creditsJSON)
				extras.CastCrewJSON = &s
			}
		}
		go cacheClient.Contribute(combined.Details, extras)
	}
}

// MediaRepo exposes the media repository for post-scan jobs (e.g. phash).
func (s *Scanner) MediaRepo() *repository.MediaRepository {
	return s.mediaRepo
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

// GenerateScreenshotPoster extracts a frame from a video file at ~50% (halfway mark)
// into the duration and saves it as the poster image for libraries that don't
// pull external metadata or when no poster was found from scrapers.
// Exported so background job handlers can use it for metadata refresh fallback.
func (s *Scanner) GenerateScreenshotPoster(item *models.MediaItem) {
	s.generateScreenshotPoster(item)
}

// generateScreenshotPoster is the internal implementation.
func (s *Scanner) generateScreenshotPoster(item *models.MediaItem) {
	if s.ffmpegPath == "" || s.posterDir == "" {
		return
	}

	// Determine seek position: 50% into the video (halfway mark)
	seekSec := 5 // default for very short/unknown duration
	if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
		seekSec = *item.DurationSeconds / 2 // 50% — halfway mark
		if seekSec < 1 {
			seekSec = 1
		}
	}

	// Ensure output directory exists
	posterDir := filepath.Join(s.posterDir, "posters")
	if err := os.MkdirAll(posterDir, 0755); err != nil {
		log.Printf("Screenshot: failed to create poster dir: %v", err)
		return
	}

	filename := item.ID.String() + ".jpg"
	outPath := filepath.Join(posterDir, filename)

	cmd := exec.Command(s.ffmpegPath,
		"-ss", fmt.Sprintf("%d", seekSec),
		"-i", item.FilePath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Screenshot: failed for %s: %s", item.FileName, string(output))
		return
	}

	// Verify the file was actually created
	if _, err := os.Stat(outPath); err != nil {
		return
	}

	webPath := "/previews/posters/" + filename
	if err := s.mediaRepo.UpdatePosterPath(item.ID, webPath); err != nil {
		log.Printf("Screenshot: failed to update poster path for %s: %v", item.FileName, err)
		return
	}
	_ = s.mediaRepo.SetGeneratedPoster(item.ID, true)
	item.PosterPath = &webPath
	item.GeneratedPoster = true
	log.Printf("Screenshot: generated poster for %s at %ds", item.FileName, seekSec)
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

// IsProbeableType returns true if the media type supports video/audio probing.
func (s *Scanner) IsProbeableType(mediaType models.MediaType) bool {
	return mediaType != models.MediaTypeImages
}

func (s *Scanner) isProbeableType(mediaType models.MediaType) bool {
	return s.IsProbeableType(mediaType)
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

	// Strip edition tags: {edition-Remastered} etc.
	name = regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(name, "")
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

// extractEdition parses {edition-XXX} from a filename. Returns "Theatrical" if none found.
func (s *Scanner) extractEdition(filename string) string {
	matches := editionPattern.FindStringSubmatch(filename)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return "Theatrical"
}

// ──────────────────── Auto Metadata Population ────────────────────

// getCacheClient returns a CacheClient if the cache server is enabled,
// auto-registering if no API key exists yet.
// Returns nil if the cache is disabled or registration fails.
func (s *Scanner) getCacheClient() *metadata.CacheClient {
	if s.settingsRepo == nil {
		return nil
	}
	enabled, _ := s.settingsRepo.Get("cache_server_enabled")
	if enabled == "false" {
		return nil
	}
	return metadata.EnsureRegistered(s.settingsRepo)
}

// autoPopulateMetadata searches external sources and applies the best match.
// When the cache server is enabled, it is tried first; direct TMDB is the fallback.
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

	// ── Try cache server first ──
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		result := cacheClient.Lookup(searchQuery, item.Year, item.MediaType)
		if result != nil && result.Match != nil {
			log.Printf("Auto-match: %q → %q (source=cache/%s, confidence=%.2f)",
				item.Title, result.Match.Title, result.Source, result.Confidence)
			s.applyCacheResult(item, result)
			return
		}
		// Cache miss or unreachable – fall through to direct TMDB
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
		match.Description, match.Rating, posterPath, match.ContentRating); err != nil {
		log.Printf("Auto-match: DB update failed for %s: %v", item.ID, err)
	}

	// Sync in-memory poster so screenshot fallback doesn't overwrite the TMDB poster
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Get TMDB details for genres, IMDB ID, OMDb ratings, and cast
	if match.Source == "tmdb" {
		s.enrichWithDetails(item.ID, match.ExternalID, item.MediaType)
	}

	// For MusicBrainz/OpenLibrary, enrich with full details
	if match.Source == "musicbrainz" || match.Source == "openlibrary" {
		s.enrichNonTMDBDetails(item.ID, match)
	}

	// Store external IDs from direct match
	idsJSON := metadata.BuildExternalIDsFromMatch(match.Source, match.ExternalID, match.IMDBId, false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	// Contribute to cache server in background (all sources)
	if cacheClient != nil {
		go cacheClient.Contribute(match)
	}
}

// applyCacheResult uses a cache server hit to populate metadata, genres, and ratings
// without making any direct TMDB/OMDb API calls.
func (s *Scanner) applyCacheResult(item *models.MediaItem, result *metadata.CacheLookupResult) {
	match := result.Match

	// Download poster if available
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" {
		filename := item.ID.String() + ".jpg"
		_, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: poster download failed for %s: %v", item.ID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
		}
	}

	// Update metadata
	if err := s.mediaRepo.UpdateMetadata(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating); err != nil {
		log.Printf("Auto-match: DB update failed for %s: %v", item.ID, err)
	}

	// Sync in-memory poster so screenshot fallback doesn't overwrite the TMDB poster
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Link genre tags from cache
	if s.tagRepo != nil && len(result.Genres) > 0 {
		s.linkGenreTags(item.ID, result.Genres)
	}

	// Apply OMDb ratings from cache
	if result.Ratings != nil {
		if err := s.mediaRepo.UpdateRatings(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore); err != nil {
			log.Printf("Auto-match: ratings update failed for %s: %v", item.ID, err)
		}
	}

	// Use cast/crew from cache if available, otherwise fall back to TMDB credits API
	if s.performerRepo != nil {
		if result.CastCrewJSON != nil && *result.CastCrewJSON != "" {
			credits := parseCacheCredits(*result.CastCrewJSON)
			if credits != nil {
				s.enrichWithCredits(item.ID, credits)
			}
		} else if match.ExternalID != "" {
			var tmdbScraper *metadata.TMDBScraper
			for _, sc := range s.scrapers {
				if t, ok := sc.(*metadata.TMDBScraper); ok {
					tmdbScraper = t
					break
				}
			}
			if tmdbScraper != nil {
				var credits *metadata.TMDBCredits
				var err error
				if item.MediaType == models.MediaTypeTVShows {
					credits, err = tmdbScraper.GetTVCredits(match.ExternalID)
				} else {
					credits, err = tmdbScraper.GetMovieCredits(match.ExternalID)
				}
				if err != nil {
					log.Printf("Auto-match: TMDB credits failed for %s: %v", match.ExternalID, err)
				} else {
					s.enrichWithCredits(item.ID, credits)
				}
			}
		}
	}

	// Store external IDs from cache
	if result.ExternalIDsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *result.ExternalIDsJSON)
	}
}

// parseCacheCredits delegates to metadata.ParseCacheCredits.
func parseCacheCredits(castCrewJSON string) *metadata.TMDBCredits {
	return metadata.ParseCacheCredits(castCrewJSON)
}

// enrichNonTMDBDetails fetches full details from MusicBrainz or OpenLibrary
// and applies genres to the media item.
func (s *Scanner) enrichNonTMDBDetails(itemID uuid.UUID, match *models.MetadataMatch) {
	var scraper metadata.Scraper
	for _, sc := range s.scrapers {
		if sc.Name() == match.Source {
			scraper = sc
			break
		}
	}
	if scraper == nil {
		return
	}

	details, err := scraper.GetDetails(match.ExternalID)
	if err != nil {
		log.Printf("Auto-match: %s details failed for %s: %v", match.Source, match.ExternalID, err)
		return
	}

	// Apply genres from detailed metadata
	if s.tagRepo != nil && len(details.Genres) > 0 {
		s.linkGenreTags(itemID, details.Genres)
	}

	// Update description if we got a better one from details
	if details.Description != nil && *details.Description != "" {
		_ = s.mediaRepo.UpdateMetadata(itemID, details.Title, details.Year,
			details.Description, details.Rating, nil, details.ContentRating)
	}

	// Update poster if details have one and we don't yet
	if details.PosterURL != nil && s.posterDir != "" {
		filename := itemID.String() + ".jpg"
		_, dlErr := metadata.DownloadPoster(*details.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr == nil {
			webPath := "/previews/posters/" + filename
			_ = s.mediaRepo.UpdatePosterPath(itemID, webPath)
		}
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

	// Update content rating if available (from TMDB release_dates)
	if details.ContentRating != nil {
		_ = s.mediaRepo.UpdateContentRating(itemID, *details.ContentRating)
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
// Thread-safe: uses mutex to prevent duplicate creation from concurrent workers.
func (s *Scanner) findOrCreatePerformer(name string, perfType models.PerformerType, profilePath string) (*models.Performer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

			if err := s.mediaRepo.UpdateMetadata(ep.ID, epTitle, nil, desc, rating, posterPath, nil); err != nil {
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
