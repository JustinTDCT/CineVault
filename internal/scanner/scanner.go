package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JustinTDCT/CineVault/internal/ffmpeg"
	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/google/uuid"
)

type Scanner struct {
	ffprobe       *ffmpeg.FFprobe
	ffmpegPath    string
	hwaccel       string
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
	// artistCache avoids repeated DB lookups for the same artist within a scan
	artistCache map[string]*models.Artist // key: "libraryID|artistName"
	// albumCache avoids repeated DB lookups for the same album within a scan
	albumCache map[string]*models.Album // key: "artistID|albumTitle"
	// genreCache avoids per-track DB queries for genre tag resolution
	genreCache map[string]uuid.UUID // key: lowercase name or slug → tag ID
	// pendingMeta collects items for deferred batch cache lookup
	pendingMeta []pendingMetaItem
	// mu protects concurrent access during parallel enrichment
	mu sync.RWMutex
}

// scanFile holds path info for the concurrent processing queue.
type scanFile struct {
	path string
	name string
	size int64
}

// pendingMetaItem is queued during scan for deferred batch cache lookup.
type pendingMetaItem struct {
	Item   *models.MediaItem
	Query  string
	Parsed ParsedFilename
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

func NewScanner(ffprobePath, ffmpegPath, hwaccel string, mediaRepo *repository.MediaRepository,
	tvRepo *repository.TVRepository, musicRepo *repository.MusicRepository,
	audiobookRepo *repository.AudiobookRepository, galleryRepo *repository.GalleryRepository,
	tagRepo *repository.TagRepository, performerRepo *repository.PerformerRepository,
	settingsRepo *repository.SettingsRepository, sisterRepo *repository.SisterRepository,
	seriesRepo *repository.SeriesRepository, tracksRepo *repository.TracksRepository,
	scrapers []metadata.Scraper, posterDir string,
) *Scanner {
	if hwaccel == "" {
		hwaccel = "none"
	}
	return &Scanner{
		ffprobe:            ffmpeg.NewFFprobe(ffprobePath),
		ffmpegPath:         ffmpegPath,
		hwaccel:            hwaccel,
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
		artistCache:        make(map[string]*models.Artist),
		albumCache:         make(map[string]*models.Album),
		genreCache:         make(map[string]uuid.UUID),
	}
}

// PreviewDir returns the base preview output directory.
func (s *Scanner) PreviewDir() string {
	return s.posterDir
}

// ProgressFunc reports scan progress: current processed count, total eligible files, files added so far, current filename.
type ProgressFunc func(current, total, added int, filename string)

func (s *Scanner) ScanLibrary(library *models.Library, progressFn ...ProgressFunc) (*models.ScanResult, error) {
	result := &models.ScanResult{}

	// Reset per-scan caches
	s.artistCache = make(map[string]*models.Artist)
	s.albumCache = make(map[string]*models.Album)
	s.genreCache = make(map[string]uuid.UUID)
	s.pendingMeta = nil

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

	// Progress reporting — count files in background so processing starts immediately
	var onProgress ProgressFunc
	if len(progressFn) > 0 && progressFn[0] != nil {
		onProgress = progressFn[0]
	}
	var totalFiles int64
	if onProgress != nil {
		go func() {
			n := s.countEligibleFiles(library.MediaType, scanPaths)
			atomic.StoreInt64(&totalFiles, int64(n))
			log.Printf("Scan: background count found %d eligible files", n)
		}()
	}

	// Determine if metadata should be retrieved for this library
	shouldRetrieveMetadata := library.RetrieveMetadata
	// Adult clips: never scrape metadata regardless of library setting
	if library.MediaType == models.MediaTypeAdultMovies && library.AdultContentType != nil && *library.AdultContentType == "clips" {
		shouldRetrieveMetadata = false
	}

	// Atomic counters for concurrent processing (8C)
	var filesFound, filesSkipped, filesAdded int64
	var errorsMu sync.Mutex
	var scanErrors []string

	for _, scanPath := range scanPaths {
		log.Printf("Scanning folder: %s", scanPath)

		// 8B: Network mount timeout — prevent hung NFS/SMB from blocking
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		statDone := make(chan struct{})
		var statErr error
		go func() {
			_, statErr = os.Stat(scanPath)
			close(statDone)
		}()
		select {
		case <-ctx.Done():
			cancel()
			log.Printf("Mount timeout: %s (skipping, possible hung NFS/SMB)", scanPath)
			errorsMu.Lock()
			scanErrors = append(scanErrors, fmt.Sprintf("mount timeout for %s", scanPath))
			errorsMu.Unlock()
			continue
		case <-statDone:
			cancel()
			if statErr != nil {
				errorsMu.Lock()
				scanErrors = append(scanErrors, fmt.Sprintf("stat failed for %s: %v", scanPath, statErr))
				errorsMu.Unlock()
				continue
			}
		}

		// 8A: Symlink cycle protection
		visitedDirs := make(map[string]bool)

		// 8C: Buffered channel and worker pool
		const numWorkers = 8
		fileCh := make(chan scanFile, numWorkers*4)
		var wg sync.WaitGroup

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for f := range fileCh {
					s.processScanFile(library, scanPath, f, shouldRetrieveMetadata,
						&filesFound, &filesSkipped, &filesAdded, &errorsMu, &scanErrors,
						onProgress, &totalFiles)
				}
			}()
		}

		// WalkDir: collect files, symlink check for dirs
		err := filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				realPath, eerr := filepath.EvalSymlinks(path)
				if eerr != nil {
					return nil
				}
				if visitedDirs[realPath] {
					return filepath.SkipDir
				}
				visitedDirs[realPath] = true
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !s.isValidExtension(library.MediaType, ext) {
				return nil
			}

			info, ierr := d.Info()
			if ierr != nil {
				return nil
			}

			extraType := IsExtraFile(path, info.Size())
			if extraType == "sample" {
				log.Printf("Skipping sample file: %s", info.Name())
				return nil
			}

			fileCh <- scanFile{path: path, name: info.Name(), size: info.Size()}
			return nil
		})

		close(fileCh)
		wg.Wait()

		if err != nil {
			errorsMu.Lock()
			scanErrors = append(scanErrors, fmt.Sprintf("walk error for %s: %v", scanPath, err))
			errorsMu.Unlock()
		}
	}

	// Copy atomic counters to result
	result.FilesFound = int(filesFound)
	result.FilesSkipped = int(filesSkipped)
	result.FilesAdded = int(filesAdded)
	result.Errors = scanErrors

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

	// Post-scan: batch-lookup deferred metadata from cache server
	if shouldRetrieveMetadata && len(s.pendingMeta) > 0 {
		s.flushPendingMeta()
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

// processScanFile handles a single file: DB check, ffprobe, metadata, persist.
// Used by concurrent workers; updates counters via atomics and mutex.
func (s *Scanner) processScanFile(library *models.Library, scanPath string, f scanFile,
	shouldRetrieveMetadata bool,
	filesFound, filesSkipped, filesAdded *int64,
	errorsMu *sync.Mutex, errors *[]string,
	onProgress ProgressFunc, totalFiles *int64,
) {
	path, name, size := f.path, f.name, f.size

	atomic.AddInt64(filesFound, 1)
	if onProgress != nil {
		onProgress(int(atomic.LoadInt64(filesFound)), int(atomic.LoadInt64(totalFiles)), int(atomic.LoadInt64(filesAdded)), name)
	}

	existing, err := s.mediaRepo.GetByFilePath(path)
	if err != nil {
		errorsMu.Lock()
		*errors = append(*errors, fmt.Sprintf("db check failed for %s: %v", path, err))
		errorsMu.Unlock()
		return
	}
	if existing != nil {
		if existing.PosterPath == nil && s.isScreenshottableType(library.MediaType) {
			s.generateScreenshotPoster(existing)
		}
		atomic.AddInt64(filesSkipped, 1)
		return
	}

	parsed := s.parseFilename(name, library.MediaType)
	extraType := IsExtraFile(path, size)

	// Also check parent folder name for provider IDs (Plex convention)
	parentFolder := filepath.Base(filepath.Dir(path))
	if parentFolder != "." && parentFolder != "/" {
		extractInlineProviderIDs(parentFolder, &parsed)
	}

	// Folder-first title resolution for movies: use parent folder name as
	// primary title source (like Plex/Jellyfin) to avoid edition keywords
	// like "Extended Cut" or "Theatrical" polluting the title.
	if (library.MediaType == models.MediaTypeMovies || library.MediaType == models.MediaTypeAdultMovies) &&
		parentFolder != "." && parentFolder != "/" {
		folderTitle, folderYear := ParseFolderName(parentFolder)
		if folderTitle != "" && folderYear != nil {
			parsed.Title = folderTitle
			parsed.Year = folderYear
			parsed.Edition = EditionFromFileVsFolder(name, folderTitle)
			log.Printf("Folder-first: %q → title=%q year=%d edition=%q",
				name, parsed.Title, *parsed.Year, parsed.Edition)
		}
	}

	var nfoData *metadata.NFOData
	if library.NFOImport {
		nfoPath := metadata.FindNFOFile(path, library.MediaType)
		if nfoPath != "" {
			switch library.MediaType {
			case models.MediaTypeMovies, models.MediaTypeAdultMovies:
				nfoData, _ = metadata.ReadMovieNFO(nfoPath)
			case models.MediaTypeTVShows:
				nfoData, _ = metadata.ReadEpisodeNFO(nfoPath)
			}
			if nfoData != nil {
				log.Printf("NFO import: parsed %s for %s", nfoPath, name)
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
	if parsed.IMDBID == "" {
		nfoIMDB := ReadNFOIMDBID(path)
		if nfoIMDB != "" {
			parsed.IMDBID = nfoIMDB
			log.Printf("NFO sidecar: found %s for %s", nfoIMDB, name)
		}
	}

	item := &models.MediaItem{
		ID:          uuid.New(),
		LibraryID:   library.ID,
		MediaType:   library.MediaType,
		FilePath:    path,
		FileName:    name,
		FileSize:    size,
		Title:       parsed.Title,
		Year:        parsed.Year,
		EditionType: parsed.Edition,
	}
	if extraType != "" {
		item.ExtraType = &extraType
	}
	if parsed.Resolution != "" {
		item.Resolution = &parsed.Resolution
	}
	if parsed.Container != "" {
		item.Container = &parsed.Container
	}
	if parsed.Source != "" {
		item.SourceType = &parsed.Source
	}
	if parsed.DiscNumber != nil {
		item.DiscNumber = parsed.DiscNumber
	}
	if parsed.TrackNumber != nil {
		item.TrackNumber = parsed.TrackNumber
	}
	if parsed.PartNumber != nil {
		item.SortPosition = *parsed.PartNumber
	}

	var probeResult *ffmpeg.ProbeResult
	if s.isProbeableType(library.MediaType) {
		probe, probeErr := s.ffprobe.Probe(path)
		if probeErr != nil {
			log.Printf("ffprobe failed for %s: %v", path, probeErr)
			errorsMu.Lock()
			*errors = append(*errors, fmt.Sprintf("probe failed: %s", path))
			errorsMu.Unlock()
		} else {
			probeResult = probe
			s.applyProbeData(item, probe)
		}
	}

	if library.MediaType == models.MediaTypeTVShows && library.SeasonGrouping {
		if err := s.handleTVHierarchy(library, item, path, scanPath); err != nil {
			log.Printf("TV hierarchy error for %s: %v", path, err)
		}
	}

	if library.MediaType == models.MediaTypeMusic || library.MediaType == models.MediaTypeMusicVideos {
		// Tags-first: embedded tags are the primary source of truth for music.
		// Filename-parsed values are only used as fallback when tags are absent.
		if probeResult != nil && len(probeResult.Format.Tags) > 0 {
			var tagArtist, tagAlbumArtist, tagAlbum, tagTitle, tagGenre string
			var tagTrack, tagDisc *int
			var tagYear *int
			for k, v := range probeResult.Format.Tags {
				switch strings.ToLower(k) {
				case "album_artist":
					tagAlbumArtist = strings.TrimSpace(v)
				case "artist":
					tagArtist = strings.TrimSpace(v)
				case "album":
					tagAlbum = strings.TrimSpace(v)
				case "title":
					tagTitle = strings.TrimSpace(v)
				case "genre":
					tagGenre = strings.TrimSpace(v)
				case "track":
					if n, err := strconv.Atoi(strings.Split(strings.TrimSpace(v), "/")[0]); err == nil {
						tagTrack = &n
					}
				case "disc", "discnumber", "disc_number":
					if n, err := strconv.Atoi(strings.Split(strings.TrimSpace(v), "/")[0]); err == nil {
						tagDisc = &n
					}
				case "date":
					v = strings.TrimSpace(v)
					if len(v) >= 4 {
						if y, err := strconv.Atoi(v[:4]); err == nil && y > 1000 && y < 3000 {
							tagYear = &y
						}
					}
				}
			}

			// Use album_artist for hierarchy, fall back to track artist.
			// Normalize by stripping featured-artist suffixes ("feat.", "ft.",
			// "featuring", "introducing") so all tracks on the same album
			// resolve to one canonical artist and one album record — matching
			// the Plex/Jellyfin model where AlbumArtist is the grouping key.
			hierarchyArtist := tagAlbumArtist
			if hierarchyArtist == "" {
				hierarchyArtist = tagArtist
			}
			if hierarchyArtist != "" {
				parsed.Artist = normalizeArtistForGrouping(hierarchyArtist)
			}
			if tagAlbumArtist != "" {
				item.AlbumArtist = &tagAlbumArtist
			}
			if tagAlbum != "" {
				parsed.Album = tagAlbum
			}
			if tagTitle != "" {
				item.Title = tagTitle
			}
			if tagTrack != nil {
				parsed.TrackNumber = tagTrack
				item.TrackNumber = tagTrack
			}
			if tagDisc != nil {
				parsed.DiscNumber = tagDisc
				item.DiscNumber = tagDisc
			}
			if tagYear != nil {
				item.Year = tagYear
			}
			// Store genre tag for later linking
			if tagGenre != "" {
				parsed.Genre = tagGenre
			}
		}

		// Multi-disc detection from folder names (e.g., "Disc 1", "CD 2", "Vol 3")
		if item.DiscNumber == nil {
			parentDir := filepath.Base(filepath.Dir(path))
			if dn := parseDiscFromFolder(parentDir); dn > 0 {
				item.DiscNumber = &dn
				parsed.DiscNumber = &dn
			}
		}

		if parsed.Artist != "" {
			if err := s.handleMusicHierarchy(library, item, parsed); err != nil {
				log.Printf("Music hierarchy error for %s: %v", path, err)
			}
		}

	}

	if err := s.mediaRepo.Create(item); err != nil {
		errorsMu.Lock()
		*errors = append(*errors, fmt.Sprintf("insert failed %s: %v", path, err))
		errorsMu.Unlock()
		return
	}

	// Link genre tag from embedded metadata (must be after Create)
	if parsed.Genre != "" && s.tagRepo != nil {
		s.linkGenreTags(item.ID, []string{parsed.Genre})
	}

	if item.ExtraType != nil {
		parentDir := filepath.Dir(filepath.Dir(path))
		parent, _ := s.mediaRepo.FindParentByDirectory(library.ID, parentDir)
		if parent != nil {
			item.ParentMediaID = &parent.ID
			s.mediaRepo.UpdateParentMediaID(item.ID, parent.ID)
		}
	}

	if probeResult != nil && s.tracksRepo != nil {
		s.extractAndStoreTracks(item.ID, path, probeResult)
	}

	if parsed.PartNumber != nil && parsed.BaseTitle != "" {
		dir := filepath.Dir(path)
		key := dir + "|" + parsed.BaseTitle
		s.mu.Lock()
		s.pendingMultiParts[key] = append(s.pendingMultiParts[key], multiPartEntry{
			ItemID:     item.ID,
			PartNumber: *parsed.PartNumber,
		})
		s.mu.Unlock()
	}

	if library.SeasonGrouping && item.TVSeasonID != nil {
		_ = s.tvRepo.IncrementEpisodeCount(*item.TVSeasonID)
	}

	if library.MediaType == models.MediaTypeTVShows && parsed.EpisodeEnd > 0 && parsed.Episode > 0 && parsed.EpisodeEnd > parsed.Episode {
		for epNum := parsed.Episode + 1; epNum <= parsed.EpisodeEnd; epNum++ {
			extraEp := &models.MediaItem{
				ID:          uuid.New(),
				LibraryID:   library.ID,
				MediaType:   library.MediaType,
				FilePath:    path,
				FileName:    name,
				FileSize:    size,
				Title:       parsed.Title,
				Year:        parsed.Year,
				TVShowID:    item.TVShowID,
				TVSeasonID:  item.TVSeasonID,
			}
			epn := epNum
			extraEp.EpisodeNumber = &epn
			if item.DurationSeconds != nil {
				totalEps := parsed.EpisodeEnd - parsed.Episode + 1
				epDur := *item.DurationSeconds / totalEps
				extraEp.DurationSeconds = &epDur
			}
			if err := s.mediaRepo.Create(extraEp); err != nil {
				log.Printf("Multi-episode: failed to create ep %d item: %v", epNum, err)
			} else {
				if library.SeasonGrouping && extraEp.TVSeasonID != nil {
					_ = s.tvRepo.IncrementEpisodeCount(*extraEp.TVSeasonID)
				}
			}
		}
	}

	if library.PreferLocalArtwork {
		localArt := DetectLocalArtwork(path, library.MediaType)
		if localArt.PosterPath != "" && item.PosterPath == nil {
			item.PosterPath = &localArt.PosterPath
			log.Printf("Local artwork: using poster %s for %s", localArt.PosterPath, name)
		}
		if localArt.BackdropPath != "" && item.BackdropPath == nil {
			item.BackdropPath = &localArt.BackdropPath
		}
		if localArt.LogoPath != "" {
			item.LogoPath = &localArt.LogoPath
		}
	}

	if nfoData != nil && nfoData.HasFullMetadata() && nfoData.LockData {
		s.applyNFOData(item, nfoData)
		item.MetadataLocked = true
		log.Printf("NFO import: applied full metadata for %s (locked)", name)
		if err := s.mediaRepo.UpdateMetadata(item.ID, item.Title, item.Year,
			item.Description, item.Rating, item.PosterPath, item.ContentRating); err != nil {
			log.Printf("NFO import: metadata update failed for %s: %v", item.ID, err)
		}
		if s.tagRepo != nil && len(nfoData.Genres) > 0 {
			s.linkGenreTags(item.ID, nfoData.Genres)
		}
	} else if shouldRetrieveMetadata && item.ExtraType == nil {
		s.autoPopulateMetadata(library, item, parsed)
	}

	if library.NFOExport && item.Description != nil {
		nfoExportPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".nfo"
		switch library.MediaType {
		case models.MediaTypeMovies, models.MediaTypeAdultMovies:
			if err := metadata.WriteMovieNFO(item, nil, nil, nil, nfoExportPath); err != nil {
				log.Printf("NFO export: write failed for %s: %v", name, err)
			}
		}
	}

	if item.PosterPath == nil && s.isScreenshottableType(library.MediaType) {
		s.generateScreenshotPoster(item)
	}

	atomic.AddInt64(filesAdded, 1)
}

// reEnrichExistingItems finds items in the library that were previously TMDB-matched
// flushPendingMeta sends all deferred items to the cache server in batches of 50,
// then applies results. Much faster than per-file individual HTTP calls.
func (s *Scanner) ScanSingleFile(library *models.Library, filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if !s.isValidExtension(library.MediaType, ext) {
		return nil
	}

	// Check if already scanned
	existing, _ := s.mediaRepo.GetByFilePath(filePath)
	if existing != nil {
		return nil
	}

	extraType := IsExtraFile(filePath, info.Size())
	if extraType == "sample" {
		return nil
	}

	parsed := s.parseFilename(info.Name(), library.MediaType)

	// Folder-first title resolution for movies (same as processScanFile)
	parentFolder := filepath.Base(filepath.Dir(filePath))
	if (library.MediaType == models.MediaTypeMovies || library.MediaType == models.MediaTypeAdultMovies) &&
		parentFolder != "." && parentFolder != "/" {
		folderTitle, folderYear := ParseFolderName(parentFolder)
		if folderTitle != "" && folderYear != nil {
			parsed.Title = folderTitle
			parsed.Year = folderYear
			parsed.Edition = EditionFromFileVsFolder(info.Name(), folderTitle)
			log.Printf("Folder-first: %q → title=%q year=%d edition=%q",
				info.Name(), parsed.Title, *parsed.Year, parsed.Edition)
		}
	}

	item := &models.MediaItem{
		ID:          uuid.New(),
		LibraryID:   library.ID,
		MediaType:   library.MediaType,
		FilePath:    filePath,
		FileName:    info.Name(),
		FileSize:    info.Size(),
		Title:       parsed.Title,
		Year:        parsed.Year,
		EditionType: parsed.Edition,
	}

	if extraType != "" {
		item.ExtraType = &extraType
	}
	if parsed.Source != "" {
		item.SourceType = &parsed.Source
	}

	// Probe if needed
	if s.isProbeableType(library.MediaType) {
		probe, probeErr := s.ffprobe.Probe(filePath)
		if probeErr == nil {
			s.applyProbeData(item, probe)
		}
	}

	if err := s.mediaRepo.Create(item); err != nil {
		return err
	}

	// Link extras to parent
	if item.ExtraType != nil {
		parentDir := filepath.Dir(filepath.Dir(filePath))
		parent, _ := s.mediaRepo.FindParentByDirectory(library.ID, parentDir)
		if parent != nil {
			s.mediaRepo.UpdateParentMediaID(item.ID, parent.ID)
		}
	}

	// Auto-populate metadata for non-extras
	if library.RetrieveMetadata && item.ExtraType == nil {
		s.autoPopulateMetadata(library, item, parsed)
	}

	log.Printf("[watcher] scanned new file: %s", info.Name())
	return nil
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

// IsScreenshottableType returns true for media types that have a video stream
// from which a screenshot poster can be extracted. Audio-only types are excluded.
func (s *Scanner) IsScreenshottableType(mediaType models.MediaType) bool {
	switch mediaType {
	case models.MediaTypeMusic, models.MediaTypeAudiobooks, models.MediaTypeImages:
		return false
	default:
		return true
	}
}

func (s *Scanner) isScreenshottableType(mediaType models.MediaType) bool {
	return s.IsScreenshottableType(mediaType)
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
