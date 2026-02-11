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
	"github.com/lib/pq"
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
	seriesRepo    *repository.SeriesRepository
	tracksRepo    *repository.TracksRepository
	scrapers      []metadata.Scraper
	posterDir     string
	// matchedShows tracks TV show IDs already matched this scan to avoid duplicate lookups
	matchedShows  map[uuid.UUID]bool
	// pendingEpisodeMeta tracks show IDs → TMDB external IDs for post-scan episode metadata fetch
	pendingEpisodeMeta map[uuid.UUID]string
	// pendingMultiParts tracks multi-part files by "dir|baseTitle" for post-scan sister grouping
	pendingMultiParts map[string][]multiPartEntry
	// scanShowsByFolder maps cleaned folder show names → TVShow records for the current scan,
	// preventing duplicate show creation when autoMatchTVShow renames the title in the DB.
	scanShowsByFolder map[string]*models.TVShow
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

// TV episode regex patterns (used by parseTVInfo for path-based detection)
var tvPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(.+?)[.\s_-]+S(\d{1,2})E(\d{1,3})`),           // Show.S01E02
	regexp.MustCompile(`(?i)(.+?)[.\s_-]+(\d{1,2})x(\d{1,3})`),            // Show.1x02
	regexp.MustCompile(`(?i)(.+?)[/\\]Season\s*(\d{1,2})[/\\].*E(\d{1,3})`), // Show/Season 1/E02
	regexp.MustCompile(`(?i)(.+?)[.\s_-]+[Ss](?:eason)?\s*(\d{1,2})\s*[Ee](?:pisode)?\s*(\d{1,3})`), // Season 1 Episode 2
	regexp.MustCompile(`(?i)(.+?)[.\s_-]+[Ee](?:pisode)?\s*(\d{1,3})`), // Episode 2 (no season)
}

func NewScanner(ffprobePath, ffmpegPath string, mediaRepo *repository.MediaRepository,
	tvRepo *repository.TVRepository, musicRepo *repository.MusicRepository,
	audiobookRepo *repository.AudiobookRepository, galleryRepo *repository.GalleryRepository,
	tagRepo *repository.TagRepository, performerRepo *repository.PerformerRepository,
	settingsRepo *repository.SettingsRepository, sisterRepo *repository.SisterRepository,
	seriesRepo *repository.SeriesRepository, tracksRepo *repository.TracksRepository,
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
		seriesRepo:         seriesRepo,
		tracksRepo:         tracksRepo,
		scrapers:           scrapers,
		posterDir:          posterDir,
		matchedShows:       make(map[uuid.UUID]bool),
		pendingEpisodeMeta: make(map[uuid.UUID]string),
		pendingMultiParts:  make(map[string][]multiPartEntry),
		scanShowsByFolder:  make(map[string]*models.TVShow),
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

			// Skip extras/samples/trailers (Jellyfin/Plex-style filtering)
			if extraType := IsExtraFile(path, info.Size()); extraType != "" {
				log.Printf("Skipping extra (%s): %s", extraType, info.Name())
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

			// ── NFO sidecar support (Kodi/Jellyfin-compatible) ──
			var nfoData *metadata.NFOData
			if library.NFOImport {
				// Try full XML NFO parsing first
				nfoPath := metadata.FindNFOFile(path, library.MediaType)
				if nfoPath != "" {
					switch library.MediaType {
					case models.MediaTypeMovies, models.MediaTypeAdultMovies:
						nfoData, _ = metadata.ReadMovieNFO(nfoPath)
					case models.MediaTypeTVShows:
						nfoData, _ = metadata.ReadEpisodeNFO(nfoPath)
					}
					if nfoData != nil {
						log.Printf("NFO import: parsed %s for %s", nfoPath, info.Name())
						// Apply NFO data to parsed filename for matching
						if nfoData.GetIMDBID() != "" {
							parsed.IMDBID = nfoData.GetIMDBID()
						}
						if nfoData.GetTMDBID() != "" {
							parsed.TMDBID = nfoData.GetTMDBID()
						}
						if nfoData.GetTVDBID() != "" {
							parsed.TVDBID = nfoData.GetTVDBID()
						}
					}
				}
			}

			// Fallback: legacy NFO IMDB ID extraction (always active)
			if parsed.IMDBID == "" {
				nfoIMDB := ReadNFOIMDBID(path)
				if nfoIMDB != "" {
					parsed.IMDBID = nfoIMDB
					log.Printf("NFO sidecar: found %s for %s", nfoIMDB, info.Name())
				}
			}

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

			// Pre-fill resolution/container/source from filename (ffprobe will override if available)
			if parsed.Resolution != "" {
				item.Resolution = &parsed.Resolution
			}
			if parsed.Container != "" {
				item.Container = &parsed.Container
			}
			// Persist source type from filename parser (e.g. "bluray", "web", "hdtv")
			if parsed.Source != "" {
				item.SourceType = &parsed.Source
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
			var probeResult *ffmpeg.ProbeResult
			if s.isProbeableType(library.MediaType) {
				probe, probeErr := s.ffprobe.Probe(path)
				if probeErr != nil {
					log.Printf("ffprobe failed for %s: %v", path, probeErr)
					result.Errors = append(result.Errors, fmt.Sprintf("probe failed: %s", path))
				} else {
					probeResult = probe
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

			// ── Extract and store subtitle tracks, audio tracks, and chapters ──
			if probeResult != nil && s.tracksRepo != nil {
				s.extractAndStoreTracks(item.ID, path, probeResult)
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

			// ── Local artwork detection (Plex/Jellyfin-style) ──
			if library.PreferLocalArtwork {
				localArt := DetectLocalArtwork(path, library.MediaType)
				if localArt.PosterPath != "" && item.PosterPath == nil {
					item.PosterPath = &localArt.PosterPath
					log.Printf("Local artwork: using poster %s for %s", localArt.PosterPath, info.Name())
				}
				if localArt.BackdropPath != "" && item.BackdropPath == nil {
					item.BackdropPath = &localArt.BackdropPath
				}
				if localArt.LogoPath != "" {
					item.LogoPath = &localArt.LogoPath
				}
			}

			// ── Apply full NFO metadata if available and has full data ──
			if nfoData != nil && nfoData.HasFullMetadata() && nfoData.LockData {
				// NFO has complete data + lockdata=true: use as-is, skip external fetch
				s.applyNFOData(item, nfoData)
				item.MetadataLocked = true
				log.Printf("NFO import: applied full metadata for %s (locked)", info.Name())
				if err := s.mediaRepo.UpdateMetadata(item.ID, item.Title, item.Year,
					item.Description, item.Rating, item.PosterPath, item.ContentRating); err != nil {
					log.Printf("NFO import: metadata update failed for %s: %v", item.ID, err)
				}
				// Link genres from NFO
				if s.tagRepo != nil && len(nfoData.Genres) > 0 {
					s.linkGenreTags(item.ID, nfoData.Genres)
				}
			} else if shouldRetrieveMetadata {
				// Auto-populate metadata from external sources (if enabled)
				s.autoPopulateMetadata(library, item, parsed)
			}

			// ── Write NFO export if enabled ──
			if library.NFOExport && item.Description != nil {
				nfoExportPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".nfo"
				switch library.MediaType {
				case models.MediaTypeMovies, models.MediaTypeAdultMovies:
					if err := metadata.WriteMovieNFO(item, nil, nil, nil, nfoExportPath); err != nil {
						log.Printf("NFO export: write failed for %s: %v", info.Name(), err)
					}
				}
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
	s.scanShowsByFolder = make(map[string]*models.TVShow)

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

	// Filter items upfront (skip fully locked items)
	var enrichItems []*models.MediaItem
	for _, item := range items {
		if item.MetadataLocked || item.IsFieldLocked("*") {
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
			if item.GeneratedPoster && result.Match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
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
			if s.tagRepo != nil && len(result.Genres) > 0 && !item.IsFieldLocked("genres") {
				s.linkGenreTags(item.ID, result.Genres)
			}

			// Link mood tags and store keywords from cache
			if len(result.Keywords) > 0 {
				s.linkMoodTags(item.ID, result.Keywords)
				s.storeKeywords(item.ID, result.Keywords)
			}

			// Apply OMDb ratings from cache
			if result.Ratings != nil {
				_ = s.mediaRepo.UpdateRatingsWithLocks(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore, item.LockedFields)
			}

			// Use cast/crew from cache if available, otherwise fall back to TMDB
			if s.performerRepo != nil && !item.IsFieldLocked("cast") {
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

			// Apply extended metadata from cache (respecting per-field locks)
			tagline, origLang, country, trailerURL, logoURL := filterLockedExtended(item.LockedFields,
				result.Match.Tagline, result.Match.OriginalLanguage, result.Match.Country, result.Match.TrailerURL, result.LogoURL)
			extUpdate := &repository.ExtendedMetadataUpdate{
				Tagline:          tagline,
				OriginalLanguage: origLang,
				Country:          country,
				TrailerURL:       trailerURL,
				LogoPath:         logoURL,
			}
			if result.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
				extUpdate.OriginalTitle = result.OriginalTitle
			}
			if result.SortTitle != nil && !isFieldLocked(item.LockedFields, "title") {
				extUpdate.SortTitle = result.SortTitle
			}
			if result.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
				extUpdate.ReleaseDate = result.ReleaseDate
			}
			_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

			// Auto-create collection from cache
			if result.Match.CollectionID != nil && result.Match.CollectionName != nil {
				s.autoCreateCollection(item, result.Match)
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

	// Link genre tags (respecting per-field lock)
	if s.tagRepo != nil && len(combined.Details.Genres) > 0 && !item.IsFieldLocked("genres") {
		s.linkGenreTags(item.ID, combined.Details.Genres)
	}

	// Link mood tags and store keywords from TMDB
	if len(combined.Details.Keywords) > 0 {
		s.linkMoodTags(item.ID, combined.Details.Keywords)
		s.storeKeywords(item.ID, combined.Details.Keywords)
	}

	// Fetch OMDb ratings (respecting per-field locks)
	if omdbKey != "" && combined.Details.IMDBId != "" {
		ratings, err := metadata.FetchOMDbRatings(combined.Details.IMDBId, omdbKey)
		if err != nil {
			log.Printf("Re-enrich: OMDb fetch failed for %s: %v", combined.Details.IMDBId, err)
		} else {
			if err := s.mediaRepo.UpdateRatingsWithLocks(item.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore, item.LockedFields); err != nil {
				log.Printf("Re-enrich: ratings update failed for %s: %v", item.ID, err)
			}
		}
	}

	// Replace generated screenshot poster with TMDB poster (respecting poster lock)
	if item.GeneratedPoster && combined.Details.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
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

	// Populate cast/crew from the credits already fetched (respecting cast lock)
	if s.performerRepo != nil && combined.Credits != nil && !item.IsFieldLocked("cast") {
		s.enrichWithCredits(item.ID, combined.Credits)
	}

	// Store external IDs from direct TMDB match
	idsJSON := metadata.BuildExternalIDsFromMatch("tmdb", match.ExternalID, combined.Details.IMDBId, false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	// Apply extended metadata from TMDB details (respecting per-field locks)
	d := combined.Details
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(item.LockedFields,
		d.Tagline, d.OriginalLanguage, d.Country, d.TrailerURL, nil)
	extUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}
	if d.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.OriginalTitle = d.OriginalTitle
	}
	if d.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
		extUpdate.ReleaseDate = d.ReleaseDate
	}
	_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

	// Auto-create movie collection from TMDB belongs_to_collection
	if combined.Details.CollectionID != nil && combined.Details.CollectionName != nil {
		s.autoCreateCollection(item, combined.Details)
	}

	// Contribute to cache server with cast/crew, ratings, and extended metadata
	if cacheClient != nil {
		extras := metadata.ContributeExtras{
			Tagline:          d.Tagline,
			OriginalLanguage: d.OriginalLanguage,
			Country:          d.Country,
			TrailerURL:       d.TrailerURL,
			BackdropURL:      d.BackdropURL,
			CollectionID:     d.CollectionID,
			CollectionName:   d.CollectionName,
			OriginalTitle:    d.OriginalTitle,
			ReleaseDate:      d.ReleaseDate,
		}
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
	af := probe.GetAudioFormat()
	if af != "" {
		item.AudioFormat = &af
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
	// HDR/Dolby Vision detection from ffprobe color metadata
	hdrFmt := probe.GetHDRFormat()
	if hdrFmt != "" {
		item.HDRFormat = &hdrFmt
		item.DynamicRange = "HDR"
	} else {
		item.DynamicRange = "SDR"
	}
}

// extractAndStoreTracks detects external subtitle files, stores embedded subtitle/audio tracks,
// and chapter markers from ffprobe data into the tracks tables.
func (s *Scanner) extractAndStoreTracks(mediaItemID uuid.UUID, filePath string, probe *ffmpeg.ProbeResult) {
	// ── External subtitle files ──
	subtitleExts := map[string]string{
		".srt": "subrip", ".ass": "ass", ".ssa": "ssa",
		".sub": "subviewer", ".vtt": "webvtt",
	}

	dir := filepath.Dir(filePath)
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			format, isSub := subtitleExts[ext]
			if !isSub {
				continue
			}
			// Match subtitle files that start with the media filename
			nameWithoutExt := strings.TrimSuffix(name, ext)
			if !strings.HasPrefix(strings.ToLower(nameWithoutExt), strings.ToLower(baseName)) {
				continue
			}

			subPath := filepath.Join(dir, name)

			// Parse language and flags from the suffix after the base name
			// e.g. "Movie.en.forced.srt" → language="en", forced=true
			suffix := nameWithoutExt[len(baseName):]
			suffix = strings.TrimPrefix(suffix, ".")
			parts := strings.Split(strings.ToLower(suffix), ".")

			var lang string
			isForced := false
			isSDH := false
			isDefault := false
			for _, p := range parts {
				switch p {
				case "forced":
					isForced = true
				case "sdh", "hi", "cc":
					isSDH = true
				case "default":
					isDefault = true
				default:
					if len(p) == 2 || len(p) == 3 {
						lang = p // ISO 639 language code
					}
				}
			}

			sub := &models.MediaSubtitle{
				MediaItemID: mediaItemID,
				Format:      format,
				FilePath:    &subPath,
				Source:      models.SubtitleSourceExternal,
				IsDefault:   isDefault,
				IsForced:    isForced,
				IsSDH:       isSDH,
			}
			if lang != "" {
				sub.Language = &lang
			}

			if err := s.tracksRepo.CreateSubtitle(sub); err != nil {
				log.Printf("Failed to store external subtitle %s: %v", name, err)
			}
		}
	}

	// ── Embedded subtitle tracks ──
	for _, st := range probe.GetSubtitleTracks() {
		sub := &models.MediaSubtitle{
			MediaItemID: mediaItemID,
			Format:      st.CodecName,
			Source:      models.SubtitleSourceEmbedded,
			StreamIndex: &st.StreamIndex,
			IsDefault:   st.IsDefault,
			IsForced:    st.IsForced,
			IsSDH:       st.IsSDH,
		}
		if st.Language != "" {
			sub.Language = &st.Language
		}
		if st.Title != "" {
			sub.Title = &st.Title
		}
		if err := s.tracksRepo.CreateSubtitle(sub); err != nil {
			log.Printf("Failed to store embedded subtitle track %d: %v", st.StreamIndex, err)
		}
	}

	// ── Audio tracks ──
	for _, at := range probe.GetAudioTracks() {
		track := &models.MediaAudioTrack{
			MediaItemID:   mediaItemID,
			StreamIndex:   at.StreamIndex,
			Codec:         at.CodecName,
			Channels:      at.Channels,
			IsDefault:     at.IsDefault,
			IsCommentary:  at.IsCommentary,
		}
		if at.Language != "" {
			track.Language = &at.Language
		}
		if at.Title != "" {
			track.Title = &at.Title
		}
		if at.ChannelLayout != "" {
			track.ChannelLayout = &at.ChannelLayout
		}
		if at.BitRate > 0 {
			br := int(at.BitRate)
			track.Bitrate = &br
		}
		if at.SampleRate > 0 {
			track.SampleRate = &at.SampleRate
		}
		if err := s.tracksRepo.CreateAudioTrack(track); err != nil {
			log.Printf("Failed to store audio track %d: %v", at.StreamIndex, err)
		}
	}

	// ── Chapters ──
	for _, ch := range probe.GetChapters() {
		chapter := &models.MediaChapter{
			MediaItemID:  mediaItemID,
			StartSeconds: ch.StartSeconds,
			SortOrder:    ch.SortOrder,
		}
		if ch.EndSeconds > 0 {
			chapter.EndSeconds = &ch.EndSeconds
		}
		if ch.Title != "" {
			chapter.Title = &ch.Title
		}
		if err := s.tracksRepo.CreateChapter(chapter); err != nil {
			log.Printf("Failed to store chapter %d: %v", ch.SortOrder, err)
		}
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

	// Find or create show — use in-memory folder map to avoid duplicate creation
	// when autoMatchTVShow renames the title in the DB (e.g. "24 - Legacy" → "24").
	folderKey := library.ID.String() + "|" + strings.ToLower(showName)
	show, ok := s.scanShowsByFolder[folderKey]
	if !ok {
		var err error
		show, err = s.tvRepo.FindShowByTitle(library.ID, showName)
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
		s.scanShowsByFolder[folderKey] = show
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

// applyDirectMatch applies metadata from a direct TMDB lookup (by ID) to a media item.
func (s *Scanner) applyDirectMatch(item *models.MediaItem, match *models.MetadataMatch) {
	// Download poster if available (respecting poster lock)
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
		filename := item.ID.String() + ".jpg"
		_, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err == nil {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
		}
	}

	if err := s.mediaRepo.UpdateMetadataWithLocks(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating, item.LockedFields); err != nil {
		log.Printf("Direct match: DB update failed for %s: %v", item.ID, err)
	}
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Apply extended metadata (respecting per-field locks)
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(item.LockedFields,
		match.Tagline, match.OriginalLanguage, match.Country, match.TrailerURL, nil)
	extUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}
	if match.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.OriginalTitle = match.OriginalTitle
	}
	if match.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
		extUpdate.ReleaseDate = match.ReleaseDate
	}
	_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

	// Enrich with genres, ratings, etc.
	if match.Source == "tmdb" {
		s.enrichWithDetails(item.ID, match.ExternalID, item.MediaType, item.LockedFields)
	}

	// Store external IDs
	idsJSON := metadata.BuildExternalIDsFromMatch(match.Source, match.ExternalID, match.IMDBId, false)
	if idsJSON != nil {
		_ = s.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
	}

	log.Printf("Direct match: %q → %q (ID=%s)", item.Title, match.Title, match.ExternalID)
}

// applyDirectMatchWithCredits applies metadata + credits from a combined TMDB lookup.
func (s *Scanner) applyDirectMatchWithCredits(item *models.MediaItem, combined *metadata.DetailsWithCredits) {
	match := combined.Details
	s.applyDirectMatch(item, match)

	// Also populate cast/crew from the included credits (respecting cast lock)
	if s.performerRepo != nil && combined.Credits != nil && !item.IsFieldLocked("cast") {
		s.enrichWithCredits(item.ID, combined.Credits)
	}

	// Auto-create movie collection from TMDB belongs_to_collection
	if match.CollectionID != nil && match.CollectionName != nil {
		s.autoCreateCollection(item, match)
	}
}

// autoCreateCollection finds or creates a movie_series from TMDB collection data
// and links the media item to it. This auto-populates the movie series tables.
func (s *Scanner) autoCreateCollection(item *models.MediaItem, match *models.MetadataMatch) {
	if s.seriesRepo == nil || match.CollectionID == nil || match.CollectionName == nil {
		return
	}

	collectionIDStr := fmt.Sprintf("%d", *match.CollectionID)

	// First check if a series with this TMDB collection ID already exists
	series, err := s.seriesRepo.FindByExternalID(item.LibraryID, collectionIDStr)
	if err != nil {
		log.Printf("Auto-collection: lookup failed: %v", err)
		return
	}

	if series == nil {
		// Also check by name (user may have manually created it)
		series, err = s.seriesRepo.FindByName(item.LibraryID, *match.CollectionName)
		if err != nil {
			log.Printf("Auto-collection: name lookup failed: %v", err)
			return
		}
	}

	if series == nil {
		// Create a new movie series
		externalIDs := fmt.Sprintf(`{"tmdb_collection_id":"%s"}`, collectionIDStr)
		series = &models.MovieSeries{
			ID:          uuid.New(),
			LibraryID:   item.LibraryID,
			Name:        *match.CollectionName,
			ExternalIDs: &externalIDs,
		}
		if err := s.seriesRepo.Create(series); err != nil {
			log.Printf("Auto-collection: create failed for %q: %v", *match.CollectionName, err)
			return
		}
		log.Printf("Auto-collection: created %q (TMDB collection %s)", *match.CollectionName, collectionIDStr)
	}

	// Check if item is already in this (or any) series
	if s.seriesRepo.IsItemInSeries(item.ID) {
		return
	}

	// Link the item to the series
	seriesItem := &models.MovieSeriesItem{
		ID:          uuid.New(),
		SeriesID:    series.ID,
		MediaItemID: item.ID,
		SortOrder:   0, // will be sorted by year later
	}
	if item.Year != nil {
		seriesItem.SortOrder = *item.Year
	}
	if err := s.seriesRepo.AddItem(seriesItem); err != nil {
		log.Printf("Auto-collection: link item failed: %v", err)
		return
	}
	log.Printf("Auto-collection: linked %q to %q", item.Title, series.Name)
}

// applyNFOData populates a MediaItem with data from a parsed NFO file.
func (s *Scanner) applyNFOData(item *models.MediaItem, nfo *metadata.NFOData) {
	if nfo.Title != "" {
		item.Title = nfo.Title
	}
	if nfo.Plot != "" {
		item.Description = &nfo.Plot
	}
	if nfo.Tagline != "" {
		item.Tagline = &nfo.Tagline
	}
	if nfo.Year > 0 {
		item.Year = &nfo.Year
	}
	if nfo.MPAA != "" {
		item.ContentRating = &nfo.MPAA
	}
	if nfo.Country != "" {
		item.Country = &nfo.Country
	}
	if nfo.TrailerURL != "" {
		item.TrailerURL = &nfo.TrailerURL
	}
	if nfo.OriginalTitle != "" {
		item.OriginalTitle = &nfo.OriginalTitle
	}
	if nfo.SortTitle != "" {
		item.SortTitle = &nfo.SortTitle
	}
	// Apply rating from NFO
	if r := nfo.GetDefaultRating(); r != nil {
		item.Rating = r
	}
	// Apply technical metadata from NFO
	if nfo.Source != "" {
		item.SourceType = &nfo.Source
	}
	if nfo.HDRFormat != "" {
		item.HDRFormat = &nfo.HDRFormat
	}
	if nfo.DynamicRange != "" {
		item.DynamicRange = nfo.DynamicRange
	}
	if nfo.CustomNotes != "" {
		item.CustomNotes = &nfo.CustomNotes
	}
}

// autoPopulateMetadata searches external sources and applies the best match.
// When the cache server is enabled, it is tried first; direct TMDB is the fallback.
// If parsed contains inline provider IDs (TMDB, IMDB), does a direct lookup instead.
func (s *Scanner) autoPopulateMetadata(library *models.Library, item *models.MediaItem, parsed ...ParsedFilename) {
	if len(s.scrapers) == 0 || !metadata.ShouldAutoMatch(item.MediaType) {
		return
	}

	// Skip items where all fields are locked
	if item.MetadataLocked || item.IsFieldLocked("*") {
		log.Printf("Auto-match: skipping %s (metadata locked)", item.ID)
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

	// ── Direct TMDB lookup if we have an ID from NFO or inline filename ──
	// This bypasses fuzzy search entirely for guaranteed accuracy
	var pf ParsedFilename
	if len(parsed) > 0 {
		pf = parsed[0]
	}
	if pf.TMDBID != "" {
		log.Printf("Auto-match: direct TMDB lookup for ID %s (%q)", pf.TMDBID, item.Title)
		for _, sc := range s.scrapers {
			if t, ok := sc.(*metadata.TMDBScraper); ok {
				if item.MediaType == models.MediaTypeTVShows {
					details, err := t.GetTVDetails(pf.TMDBID)
					if err == nil && details != nil {
						s.applyDirectMatch(item, details)
						return
					}
				} else {
					details, err := t.GetDetailsWithCredits(pf.TMDBID)
					if err == nil && details != nil {
						s.applyDirectMatchWithCredits(item, details)
						return
					}
				}
				break
			}
		}
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

	// Download poster if available (respecting poster lock)
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
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

	// Update the media item with matched metadata (respecting per-field locks)
	if err := s.mediaRepo.UpdateMetadataWithLocks(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating, item.LockedFields); err != nil {
		log.Printf("Auto-match: DB update failed for %s: %v", item.ID, err)
	}

	// Sync in-memory poster so screenshot fallback doesn't overwrite the TMDB poster
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Get TMDB details for genres, IMDB ID, OMDb ratings, and cast
	if match.Source == "tmdb" {
		s.enrichWithDetails(item.ID, match.ExternalID, item.MediaType, item.LockedFields)
	}

	// For MusicBrainz/OpenLibrary, enrich with full details
	if match.Source == "musicbrainz" || match.Source == "openlibrary" {
		s.enrichNonTMDBDetails(item.ID, match, item.LockedFields)
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

	// Download poster if available (respecting poster lock)
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" && !item.IsFieldLocked("poster_path") {
		filename := item.ID.String() + ".jpg"
		_, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: poster download failed for %s: %v", item.ID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
		}
	}

	// Update metadata (respecting per-field locks)
	if err := s.mediaRepo.UpdateMetadataWithLocks(item.ID, match.Title, match.Year,
		match.Description, match.Rating, posterPath, match.ContentRating, item.LockedFields); err != nil {
		log.Printf("Auto-match: DB update failed for %s: %v", item.ID, err)
	}

	// Sync in-memory poster so screenshot fallback doesn't overwrite the TMDB poster
	if posterPath != nil {
		item.PosterPath = posterPath
	}

	// Link genre tags from cache (respecting genres lock)
	if s.tagRepo != nil && len(result.Genres) > 0 && !item.IsFieldLocked("genres") {
		s.linkGenreTags(item.ID, result.Genres)
	}

	// Link mood tags and store keywords from cache
	if len(result.Keywords) > 0 {
		s.linkMoodTags(item.ID, result.Keywords)
		s.storeKeywords(item.ID, result.Keywords)
	}

	// Apply OMDb ratings from cache (respecting per-field locks)
	if result.Ratings != nil {
		if err := s.mediaRepo.UpdateRatingsWithLocks(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore, item.LockedFields); err != nil {
			log.Printf("Auto-match: ratings update failed for %s: %v", item.ID, err)
		}
	}

	// Use cast/crew from cache if available (respecting cast lock)
	if s.performerRepo != nil && !item.IsFieldLocked("cast") {
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

	// Apply extended metadata from cache (respecting per-field locks)
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(item.LockedFields,
		match.Tagline, match.OriginalLanguage, match.Country, match.TrailerURL, result.LogoURL)
	extUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}

	// Apply original_title, sort_title, release_date from cache (respecting locks)
	if result.OriginalTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.OriginalTitle = result.OriginalTitle
	}
	if result.SortTitle != nil && !isFieldLocked(item.LockedFields, "title") {
		extUpdate.SortTitle = result.SortTitle
	}
	if result.ReleaseDate != nil && !isFieldLocked(item.LockedFields, "year") {
		extUpdate.ReleaseDate = result.ReleaseDate
	}

	_ = s.mediaRepo.UpdateExtendedMetadataFull(item.ID, extUpdate)

	// Auto-create movie collection from cache data
	if match.CollectionID != nil && match.CollectionName != nil {
		s.autoCreateCollection(item, match)
	}
}

// parseCacheCredits delegates to metadata.ParseCacheCredits.
func parseCacheCredits(castCrewJSON string) *metadata.TMDBCredits {
	return metadata.ParseCacheCredits(castCrewJSON)
}

// isFieldLocked checks if a specific field name is present in the locked_fields array.
func isFieldLocked(lf pq.StringArray, field string) bool {
	for _, f := range lf {
		if f == "*" || f == field {
			return true
		}
	}
	return false
}

// filterLockedExtended nils out extended metadata values for fields that are locked on the item.
func filterLockedExtended(lf pq.StringArray, tagline, origLang, country, trailerURL, logoPath *string) (*string, *string, *string, *string, *string) {
	if len(lf) == 0 {
		return tagline, origLang, country, trailerURL, logoPath
	}
	check := func(field string) bool {
		for _, f := range lf {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}
	if check("tagline") {
		tagline = nil
	}
	if check("original_language") {
		origLang = nil
	}
	if check("country") {
		country = nil
	}
	if check("trailer_url") {
		trailerURL = nil
	}
	if check("logo_path") {
		logoPath = nil
	}
	return tagline, origLang, country, trailerURL, logoPath
}

// enrichNonTMDBDetails fetches full details from MusicBrainz or OpenLibrary
// and applies genres to the media item.
func (s *Scanner) enrichNonTMDBDetails(itemID uuid.UUID, match *models.MetadataMatch, lockedFields pq.StringArray) {
	isLocked := func(field string) bool {
		for _, f := range lockedFields {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}

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

	// Apply genres from detailed metadata (respecting genres lock)
	if s.tagRepo != nil && len(details.Genres) > 0 && !isLocked("genres") {
		s.linkGenreTags(itemID, details.Genres)
	}

	// Link mood tags and store keywords from TMDB details
	if len(details.Keywords) > 0 {
		s.linkMoodTags(itemID, details.Keywords)
		s.storeKeywords(itemID, details.Keywords)
	}

	// Update description if we got a better one from details (respecting per-field locks)
	if details.Description != nil && *details.Description != "" {
		_ = s.mediaRepo.UpdateMetadataWithLocks(itemID, details.Title, details.Year,
			details.Description, details.Rating, nil, details.ContentRating, lockedFields)
	}

	// Update poster if details have one and we don't yet (respecting poster lock)
	if details.PosterURL != nil && s.posterDir != "" && !isLocked("poster_path") {
		filename := itemID.String() + ".jpg"
		_, dlErr := metadata.DownloadPoster(*details.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr == nil {
			webPath := "/previews/posters/" + filename
			_ = s.mediaRepo.UpdatePosterPath(itemID, webPath)
		}
	}
}

// EnrichMatchedItem is the exported entry-point for extended metadata enrichment.
// It fetches TMDB details (content rating, tagline, language, country, trailer),
// TMDB credits, OMDb ratings, and fanart.tv artwork — all with per-field lock awareness.
// Used by the metadata refresh handler after a base match is applied.
func (s *Scanner) EnrichMatchedItem(itemID uuid.UUID, tmdbExternalID string, mediaType models.MediaType, lockedFields pq.StringArray) {
	s.enrichWithDetails(itemID, tmdbExternalID, mediaType, lockedFields)
}

// enrichWithDetails fetches TMDB details, creates genre tags, fetches OMDb ratings, and populates cast.
// lockedFields is passed through to respect per-field metadata locks.
func (s *Scanner) enrichWithDetails(itemID uuid.UUID, tmdbExternalID string, mediaType models.MediaType, lockedFields pq.StringArray) {
	isLocked := func(field string) bool {
		for _, f := range lockedFields {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}
	_ = isLocked // ensure used
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

	// Update content rating if available (respecting content_rating lock)
	if details.ContentRating != nil && !isLocked("content_rating") {
		_ = s.mediaRepo.UpdateContentRating(itemID, *details.ContentRating)
	}

	// Apply extended metadata from TMDB details (respecting per-field locks)
	eTagline, eLang, eCountry, eTrailer, eLogo := filterLockedExtended(lockedFields,
		details.Tagline, details.OriginalLanguage, details.Country, details.TrailerURL, nil)
	detExtUpdate := &repository.ExtendedMetadataUpdate{
		Tagline:          eTagline,
		OriginalLanguage: eLang,
		Country:          eCountry,
		TrailerURL:       eTrailer,
		LogoPath:         eLogo,
	}
	if details.OriginalTitle != nil && !isLocked("title") {
		detExtUpdate.OriginalTitle = details.OriginalTitle
	}
	if details.ReleaseDate != nil && !isLocked("year") {
		detExtUpdate.ReleaseDate = details.ReleaseDate
	}
	_ = s.mediaRepo.UpdateExtendedMetadataFull(itemID, detExtUpdate)

	// Create/link genre tags (respecting genres lock)
	if s.tagRepo != nil && len(details.Genres) > 0 && !isLocked("genres") {
		s.linkGenreTags(itemID, details.Genres)
	}

	// Link mood tags and store keywords
	if len(details.Keywords) > 0 {
		s.linkMoodTags(itemID, details.Keywords)
		s.storeKeywords(itemID, details.Keywords)
	}

	// Fetch OMDb ratings if key is configured (respecting per-field locks)
	if s.settingsRepo != nil && details.IMDBId != "" {
		omdbKey, err := s.settingsRepo.Get("omdb_api_key")
		if err != nil {
			log.Printf("Auto-match: settings lookup failed: %v", err)
		} else if omdbKey != "" {
			ratings, err := metadata.FetchOMDbRatings(details.IMDBId, omdbKey)
			if err != nil {
				log.Printf("Auto-match: OMDb fetch failed for %s: %v", details.IMDBId, err)
			} else {
				if err := s.mediaRepo.UpdateRatingsWithLocks(itemID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore, lockedFields); err != nil {
					log.Printf("Auto-match: ratings update failed for %s: %v", itemID, err)
				}
			}
		}
	}

	// Fetch and populate cast/crew from TMDB credits (respecting cast lock)
	if s.performerRepo != nil && !isLocked("cast") {
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

	// Fetch extended artwork from fanart.tv (logos, banners, clearart)
	s.enrichWithFanart(itemID, tmdbExternalID, mediaType, lockedFields)
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

// enrichWithFanart fetches extended artwork from fanart.tv and applies logos, banners, etc.
// lockedFields is checked to skip updates to locked artwork fields.
func (s *Scanner) enrichWithFanart(itemID uuid.UUID, tmdbExternalID string, mediaType models.MediaType, lockedFields pq.StringArray) {
	if s.settingsRepo == nil {
		return
	}
	fanartKey, _ := s.settingsRepo.Get("fanart_api_key")
	if fanartKey == "" {
		return
	}

	isLocked := func(field string) bool {
		for _, f := range lockedFields {
			if f == "*" || f == field {
				return true
			}
		}
		return false
	}

	client := metadata.NewFanartTVClient(fanartKey)
	var art *metadata.FanartArtwork
	var err error

	if mediaType == models.MediaTypeTVShows {
		art, err = client.GetTVArtwork(tmdbExternalID)
	} else {
		art, err = client.GetMovieArtwork(tmdbExternalID)
	}

	if err != nil {
		log.Printf("fanart.tv: fetch failed for %s: %v", tmdbExternalID, err)
		return
	}
	if art == nil {
		return
	}

	// Download and save logo (respecting logo_path lock)
	if art.LogoURL != "" && s.posterDir != "" && !isLocked("logo_path") {
		filename := "logo_" + itemID.String() + ".png"
		_, dlErr := metadata.DownloadPoster(art.LogoURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr == nil {
			webPath := "/previews/posters/" + filename
			s.mediaRepo.DB().Exec(`UPDATE media_items SET logo_path = $1 WHERE id = $2`, webPath, itemID)
			log.Printf("fanart.tv: saved logo for %s", itemID)
		}
	}

	// Download backdrop if we don't have one yet (respecting backdrop_path lock)
	if art.BackdropURL != "" && s.posterDir != "" && !isLocked("backdrop_path") {
		filename := "backdrop_" + itemID.String() + ".jpg"
		_, dlErr := metadata.DownloadPoster(art.BackdropURL, filepath.Join(s.posterDir, "posters"), filename)
		if dlErr == nil {
			webPath := "/previews/posters/" + filename
			s.mediaRepo.DB().Exec(`UPDATE media_items SET backdrop_path = COALESCE(backdrop_path, $1) WHERE id = $2`, webPath, itemID)
		}
	}
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

// ──────────────────── Mood Tagging ────────────────────

// tmdbKeywordToMood maps TMDB keywords (lowercase) to mood tag names.
// These are the most common TMDB keywords mapped to user-friendly mood categories.
var tmdbKeywordToMood = map[string]string{
	// Feel-Good / Uplifting
	"feel-good":          "Feel-Good",
	"heartwarming":       "Feel-Good",
	"uplifting":          "Feel-Good",
	"inspirational":      "Feel-Good",
	"friendship":         "Feel-Good",
	"underdog":           "Feel-Good",
	"coming of age":      "Feel-Good",
	"road trip":          "Feel-Good",
	"buddy":              "Feel-Good",
	// Dark / Gritty
	"dark":               "Dark",
	"gritty":             "Dark",
	"dystopia":           "Dark",
	"neo-noir":           "Dark",
	"nihilism":           "Dark",
	"bleak":              "Dark",
	"post-apocalyptic":   "Dark",
	"apocalypse":         "Dark",
	// Intense / Suspenseful
	"suspense":           "Intense",
	"tension":            "Intense",
	"psychological":      "Intense",
	"thriller":           "Intense",
	"conspiracy":         "Intense",
	"paranoia":           "Intense",
	"chase":              "Intense",
	"hostage":            "Intense",
	"cat and mouse":      "Intense",
	"survival":           "Intense",
	// Romantic
	"romance":            "Romantic",
	"love":               "Romantic",
	"love triangle":      "Romantic",
	"forbidden love":     "Romantic",
	"wedding":            "Romantic",
	"first love":         "Romantic",
	"soulmates":          "Romantic",
	// Funny / Light-hearted
	"comedy":             "Funny",
	"slapstick":          "Funny",
	"satire":             "Funny",
	"parody":             "Funny",
	"absurd":             "Funny",
	"dark comedy":        "Funny",
	"farce":              "Funny",
	"quirky":             "Funny",
	"witty":              "Funny",
	// Emotional / Tearjerker
	"tearjerker":         "Emotional",
	"tragedy":            "Emotional",
	"grief":              "Emotional",
	"loss":               "Emotional",
	"death":              "Emotional",
	"dying":              "Emotional",
	"terminal illness":   "Emotional",
	"loss of loved one":  "Emotional",
	// Mind-bending
	"mind-bending":       "Mind-Bending",
	"twist ending":       "Mind-Bending",
	"time travel":        "Mind-Bending",
	"time loop":          "Mind-Bending",
	"alternate reality":  "Mind-Bending",
	"dream":              "Mind-Bending",
	"parallel universe":  "Mind-Bending",
	"nonlinear timeline": "Mind-Bending",
	"surreal":            "Mind-Bending",
	"hallucination":      "Mind-Bending",
	// Scary / Creepy
	"horror":             "Scary",
	"haunted house":      "Scary",
	"ghost":              "Scary",
	"demon":              "Scary",
	"slasher":            "Scary",
	"paranormal":         "Scary",
	"zombie":             "Scary",
	"vampire":            "Scary",
	"werewolf":           "Scary",
	"monster":            "Scary",
	"serial killer":      "Scary",
	"possession":         "Scary",
	"supernatural":       "Scary",
	"occult":             "Scary",
	// Epic / Grand
	"epic":               "Epic",
	"war":                "Epic",
	"battle":             "Epic",
	"medieval":           "Epic",
	"ancient":            "Epic",
	"mythology":          "Epic",
	"sword and sorcery":  "Epic",
	"historical":         "Epic",
	// Adrenaline / Action
	"action":             "Adrenaline",
	"explosion":          "Adrenaline",
	"car chase":          "Adrenaline",
	"martial arts":       "Adrenaline",
	"heist":              "Adrenaline",
	"revenge":            "Adrenaline",
	"gunfight":           "Adrenaline",
	"fight":              "Adrenaline",
}

// linkMoodTags maps TMDB keywords to mood categories and links them as mood tags.
func (s *Scanner) linkMoodTags(itemID uuid.UUID, keywords []string) {
	if s.tagRepo == nil || len(keywords) == 0 {
		return
	}

	// Deduplicate moods from keywords
	moodSet := make(map[string]bool)
	for _, kw := range keywords {
		if mood, ok := tmdbKeywordToMood[strings.ToLower(kw)]; ok {
			moodSet[mood] = true
		}
	}

	for mood := range moodSet {
		// Find or create the mood tag
		existing, _ := s.tagRepo.List("mood")
		var tagID uuid.UUID
		found := false
		for _, t := range existing {
			if strings.EqualFold(t.Name, mood) {
				tagID = t.ID
				found = true
				break
			}
		}
		if !found {
			tagID = uuid.New()
			tag := &models.Tag{
				ID:       tagID,
				Name:     mood,
				Category: models.TagCategoryMood,
			}
			if err := s.tagRepo.Create(tag); err != nil {
				log.Printf("Auto-match: create mood tag %q failed: %v", mood, err)
				continue
			}
		}
		if err := s.tagRepo.AssignToMedia(itemID, tagID); err != nil {
			log.Printf("Auto-match: assign mood tag %q to %s failed: %v", mood, itemID, err)
		}
	}
}

// storeKeywords saves TMDB keywords as a JSON array on the media item.
func (s *Scanner) storeKeywords(itemID uuid.UUID, keywords []string) {
	if len(keywords) == 0 {
		return
	}
	data, err := json.Marshal(keywords)
	if err != nil {
		return
	}
	kwStr := string(data)
	_, _ = s.mediaRepo.DB().Exec(`UPDATE media_items SET keywords = $1 WHERE id = $2`, kwStr, itemID)
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

	// ── Try cache server first ──
	cacheClient := s.getCacheClient()
	if cacheClient != nil {
		result := cacheClient.Lookup(searchQuery, nil, models.MediaTypeTVShows)
		if result != nil && result.Match != nil {
			log.Printf("Auto-match TV: %q → %q (source=cache/%s, confidence=%.2f)",
				show.Title, result.Match.Title, result.Source, result.Confidence)
			s.applyTVShowCacheResult(showID, show, result)
			return
		}
	}

	// ── Fall back to direct TMDB ──
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

	// Contribute TV show to cache server in background
	if cacheClient != nil {
		go cacheClient.Contribute(match, metadata.ContributeExtras{
			MediaType: models.MediaTypeTVShows,
		})
	}
}

// applyTVShowCacheResult applies a cache server hit to a TV show record,
// including poster, genres, ratings, and queuing episode metadata fetch.
func (s *Scanner) applyTVShowCacheResult(showID uuid.UUID, show *models.TVShow, result *metadata.CacheLookupResult) {
	match := result.Match

	// Download poster if available
	var posterPath *string
	if match.PosterURL != nil && s.posterDir != "" {
		filename := "tvshow_" + showID.String() + ".jpg"
		_, err := metadata.DownloadPoster(*match.PosterURL, filepath.Join(s.posterDir, "posters"), filename)
		if err != nil {
			log.Printf("Auto-match: TV poster download failed for %s: %v", showID, err)
		} else {
			webPath := "/previews/posters/" + filename
			posterPath = &webPath
		}
	}

	if err := s.tvRepo.UpdateShowMetadata(showID, match.Title, match.Year,
		match.Description, match.Rating, posterPath); err != nil {
		log.Printf("Auto-match: TV show DB update failed for %s: %v", showID, err)
	}

	// Get all episodes for this show to apply genres/ratings
	episodes, err := s.mediaRepo.ListByTVShow(showID)
	if err != nil {
		log.Printf("Auto-match: failed to list episodes for genre/rating enrichment: %v", err)
		episodes = nil
	}

	// Link genre tags to all episodes
	if s.tagRepo != nil && len(result.Genres) > 0 && episodes != nil {
		for _, ep := range episodes {
			if !ep.IsFieldLocked("genres") {
				s.linkGenreTags(ep.ID, result.Genres)
			}
		}
	}

	// Apply OMDb ratings from cache to all episodes
	if result.Ratings != nil && episodes != nil {
		for _, ep := range episodes {
			_ = s.mediaRepo.UpdateRatingsWithLocks(ep.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore, ep.LockedFields)
		}
	}

	// Use cast/crew from cache if available
	if s.performerRepo != nil && result.CastCrewJSON != nil && *result.CastCrewJSON != "" && len(episodes) > 0 {
		credits := parseCacheCredits(*result.CastCrewJSON)
		if credits != nil {
			s.enrichWithCredits(episodes[0].ID, credits)
		}
	}

	// Queue episode-level metadata fetch if we have a TMDB external ID
	if match.ExternalID != "" {
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

	// Link genre tags to all episodes (respecting per-episode locks)
	if s.tagRepo != nil && len(details.Genres) > 0 {
		for _, ep := range episodes {
			if !ep.IsFieldLocked("genres") {
				s.linkGenreTags(ep.ID, details.Genres)
			}
		}
	}

	// Fetch OMDb ratings and apply to all episodes (respecting per-episode locks)
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
					if err := s.mediaRepo.UpdateRatingsWithLocks(ep.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore, ep.LockedFields); err != nil {
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

			// Skip fully locked episodes
			if ep.MetadataLocked || ep.IsFieldLocked("*") {
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

			// Download episode still image (respecting poster lock)
			var posterPath *string
			if tmdbEp.StillPath != "" && s.posterDir != "" && !ep.IsFieldLocked("poster_path") {
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

			if err := s.mediaRepo.UpdateMetadataWithLocks(ep.ID, epTitle, nil, desc, rating, posterPath, nil, ep.LockedFields); err != nil {
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

// extractYear tries to find a 4-digit year in a filename using the improved patterns.
func (s *Scanner) extractYear(filename string) *int {
	// Try parens/brackets first: (2020) or [2020]
	if m := yearInParensRx.FindStringSubmatch(filename); len(m) >= 2 {
		year, err := strconv.Atoi(m[1])
		if err == nil && year >= 1900 && year <= 2100 {
			return &year
		}
	}
	// Try delimited: .2020. -2020-
	if m := yearRx.FindStringSubmatch(filename); len(m) >= 2 {
		year, err := strconv.Atoi(m[1])
		if err == nil && year >= 1900 && year <= 2100 {
			return &year
		}
	}
	return nil
}
