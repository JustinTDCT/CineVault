package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/fingerprint"
	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/JustinTDCT/CineVault/internal/scanner"
)

// ──────── Payloads ────────

type ScanPayload struct {
	LibraryID string `json:"library_id"`
}

type FingerprintPayload struct {
	MediaItemID string `json:"media_item_id"`
}

type PreviewPayload struct {
	MediaItemID string `json:"media_item_id"`
}

type PhashLibraryPayload struct {
	LibraryID string `json:"library_id"`
}

type MetadataPayload struct {
	MediaItemID string `json:"media_item_id"`
	Source      string `json:"source"`
}

// ──────── Handlers ────────

type ScanHandler struct {
	scanner  *scanner.Scanner
	libRepo  *repository.LibraryRepository
	jobRepo  *repository.JobRepository
	queue    *Queue
	notifier EventNotifier
}

type EventNotifier interface {
	Broadcast(event string, data interface{})
}

func NewScanHandler(sc *scanner.Scanner, libRepo *repository.LibraryRepository, jobRepo *repository.JobRepository, queue *Queue, notifier EventNotifier) *ScanHandler {
	return &ScanHandler{scanner: sc, libRepo: libRepo, jobRepo: jobRepo, queue: queue, notifier: notifier}
}

func (h *ScanHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p ScanPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	libID, _ := uuid.Parse(p.LibraryID)
	library, err := h.libRepo.GetByID(libID)
	if err != nil {
		return fmt.Errorf("get library: %w", err)
	}

	taskID := "scan:" + p.LibraryID
	taskDesc := "Scanning: " + library.Name

	log.Printf("Job: scanning library %q", library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("scan:start", map[string]string{"library_id": p.LibraryID, "name": library.Name})
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskScanLibrary,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	// Build a throttled progress callback to broadcast scan progress via WebSocket
	var progressFn scanner.ProgressFunc
	if h.notifier != nil {
		var lastBroadcast time.Time
		progressFn = func(current, total int, filename string) {
			now := time.Now()
			// Throttle: broadcast at most every 500ms, plus always on last item
			if now.Sub(lastBroadcast) >= 500*time.Millisecond || current == total {
				lastBroadcast = now
				pct := 0
				if total > 0 {
					pct = int(float64(current) / float64(total) * 100)
				}
				h.notifier.Broadcast("scan:progress", map[string]interface{}{
					"library_id": p.LibraryID,
					"current":    current,
					"total":      total,
					"filename":   filename,
				})
				// Build descriptive status: "Scanning Movies · filename.mp4 (5/120)"
				desc := fmt.Sprintf("Scanning %s · %s (%d/%d)", library.Name, filename, current, total)
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskScanLibrary,
					"status": "running", "progress": pct, "description": desc,
				})
			}
		}
	}

	result, err := h.scanner.ScanLibrary(library, progressFn)
	if err != nil {
		if h.notifier != nil {
			h.notifier.Broadcast("task:update", map[string]interface{}{
				"task_id": taskID, "task_type": TaskScanLibrary,
				"status": "failed", "progress": 0, "description": taskDesc,
			})
		}
		return fmt.Errorf("scan: %w", err)
	}

	_ = h.libRepo.UpdateLastScan(libID)

	log.Printf("Job: scan complete - %d found, %d added", result.FilesFound, result.FilesAdded)
	if h.notifier != nil {
		h.notifier.Broadcast("scan:complete", map[string]interface{}{
			"library_id": p.LibraryID,
			"result":     result,
		})
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskScanLibrary,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}

	// Trigger phash computation as a follow-up background job (deduplicated by library ID)
	if h.queue != nil {
		uniqueID := "phash:" + p.LibraryID
		if _, err := h.queue.EnqueueUnique(TaskPhashLibrary, PhashLibraryPayload{LibraryID: p.LibraryID}, uniqueID,
			asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour)); err != nil {
			log.Printf("Job: failed to enqueue phash job for library %s: %v", p.LibraryID, err)
		} else {
			log.Printf("Job: enqueued phash computation for library %s", p.LibraryID)
		}
	}

	return nil
}

// ──────── Fingerprint Handler (single item - stub) ────────

type FingerprintHandler struct {
	mediaRepo *repository.MediaRepository
}

func NewFingerprintHandler(mediaRepo *repository.MediaRepository) *FingerprintHandler {
	return &FingerprintHandler{mediaRepo: mediaRepo}
}

func (h *FingerprintHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p FingerprintPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Printf("Job: fingerprinting media %s", p.MediaItemID)
	return nil
}

// ──────── Phash Library Handler ────────

type PhashLibraryHandler struct {
	mediaRepo    *repository.MediaRepository
	fingerprinter *fingerprint.Fingerprinter
	notifier     EventNotifier
}

func NewPhashLibraryHandler(mediaRepo *repository.MediaRepository, fp *fingerprint.Fingerprinter, notifier EventNotifier) *PhashLibraryHandler {
	return &PhashLibraryHandler{mediaRepo: mediaRepo, fingerprinter: fp, notifier: notifier}
}

func (h *PhashLibraryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p PhashLibraryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	taskID := "phash:" + p.LibraryID
	taskDesc := "Analyzing duplicates"

	libID, _ := uuid.Parse(p.LibraryID)
	items, err := h.mediaRepo.ListItemsNeedingPhash(libID)
	if err != nil {
		return fmt.Errorf("list items needing phash: %w", err)
	}
	if len(items) == 0 {
		log.Printf("Phash: no items need hashing in library %s", p.LibraryID)
		return nil
	}

	log.Printf("Phash: computing phash for %d items in library %s", len(items), p.LibraryID)

	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskPhashLibrary,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	computed := 0
	var lastTaskBroadcast time.Time
	for i, item := range items {
		select {
		case <-ctx.Done():
			log.Printf("Phash: cancelled after %d/%d items", computed, len(items))
			return ctx.Err()
		default:
		}

		// Use the item's actual duration for multi-point sampling
		dur := 0
		if item.DurationSeconds != nil {
			dur = *item.DurationSeconds
		}
		phash, err := h.fingerprinter.ComputePHash(item.FilePath, dur)
		if err != nil {
			log.Printf("Phash: failed for %s: %v", item.FileName, err)
			continue
		}
		if err := h.mediaRepo.UpdatePhash(item.ID, phash); err != nil {
			log.Printf("Phash: failed to store for %s: %v", item.FileName, err)
			continue
		}
		computed++

		// Broadcast task progress (throttled to every 500ms, plus always on last item)
		if h.notifier != nil {
			now := time.Now()
			if now.Sub(lastTaskBroadcast) >= 500*time.Millisecond || i == len(items)-1 {
				lastTaskBroadcast = now
				pct := int(float64(i+1) / float64(len(items)) * 100)
				desc := fmt.Sprintf("Analyzing duplicates · %s (%d/%d)", item.FileName, i+1, len(items))
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskPhashLibrary,
					"status": "running", "progress": pct, "description": desc,
				})
			}
		}
	}

	log.Printf("Phash: computed %d/%d hashes, checking for potential duplicates", computed, len(items))

	// Check all phash items in the library for potential duplicates
	allHashed, err := h.mediaRepo.ListPhashesInLibrary(libID)
	if err != nil {
		return fmt.Errorf("list phashes: %w", err)
	}

	dupCount := 0
	for i := 0; i < len(allHashed); i++ {
		for j := i + 1; j < len(allHashed); j++ {
			if allHashed[i].Phash == nil || allHashed[j].Phash == nil {
				continue
			}

			// Duration pre-filter: skip comparison if durations differ by more than 5%
			if allHashed[i].DurationSeconds != nil && allHashed[j].DurationSeconds != nil {
				durA := float64(*allHashed[i].DurationSeconds)
				durB := float64(*allHashed[j].DurationSeconds)
				if durA > 0 && durB > 0 {
					ratio := durA / durB
					if ratio < 0.95 || ratio > 1.05 {
						continue
					}
				}
			}

			sim := fingerprint.Similarity(*allHashed[i].Phash, *allHashed[j].Phash)
			if sim >= 0.90 {
				if allHashed[i].DuplicateStatus != "addressed" {
					_ = h.mediaRepo.UpdateDuplicateStatus(allHashed[i].ID, "potential")
				}
				if allHashed[j].DuplicateStatus != "addressed" {
					_ = h.mediaRepo.UpdateDuplicateStatus(allHashed[j].ID, "potential")
				}
				dupCount++
			}
		}
	}

	log.Printf("Phash: found %d potential duplicate pairs in library %s", dupCount, p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("phash:complete", map[string]interface{}{
			"library_id":      p.LibraryID,
			"computed":        computed,
			"duplicate_pairs": dupCount,
		})
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskPhashLibrary,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}

// ──────── Preview Handler (stub) ────────

type PreviewHandler struct{}

func NewPreviewHandler() *PreviewHandler {
	return &PreviewHandler{}
}

func (h *PreviewHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p PreviewPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Printf("Job: generating preview for %s", p.MediaItemID)
	return nil
}

// ──────── Metadata Scrape Handler ────────

type MetadataScrapeHandler struct {
	mediaRepo    *repository.MediaRepository
	libRepo      *repository.LibraryRepository
	settingsRepo *repository.SettingsRepository
	scrapers     []metadata.Scraper
	cfg          *config.Config
	notifier     EventNotifier
}

func NewMetadataScrapeHandler(mediaRepo *repository.MediaRepository, libRepo *repository.LibraryRepository,
	settingsRepo *repository.SettingsRepository, scrapers []metadata.Scraper,
	cfg *config.Config, notifier EventNotifier) *MetadataScrapeHandler {
	return &MetadataScrapeHandler{
		mediaRepo: mediaRepo, libRepo: libRepo, settingsRepo: settingsRepo,
		scrapers: scrapers, cfg: cfg, notifier: notifier,
	}
}

func (h *MetadataScrapeHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	// The auto-match endpoint sends {"library_id":"..."} under the metadata:scrape task type
	var payload struct {
		LibraryID string `json:"library_id"`
	}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	libID, _ := uuid.Parse(payload.LibraryID)
	library, err := h.libRepo.GetByID(libID)
	if err != nil {
		return fmt.Errorf("get library: %w", err)
	}

	taskID := "metadata:" + payload.LibraryID
	taskDesc := "Metadata refresh: " + library.Name

	items, err := h.mediaRepo.ListUnlockedByLibrary(libID)
	if err != nil {
		return fmt.Errorf("list unlocked items: %w", err)
	}
	if len(items) == 0 {
		log.Printf("Metadata: no unlocked items in library %s", library.Name)
		return nil
	}

	log.Printf("Metadata: refreshing %d unlocked items in %q", len(items), library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskMetadataScrape,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	// Fetch OMDb API key once
	omdbKey, _ := h.settingsRepo.Get("omdb_api_key")

	// Check if cache server is enabled and auto-register if needed
	var cacheClient *metadata.CacheClient
	cacheEnabled, _ := h.settingsRepo.Get("cache_server_enabled")
	if cacheEnabled != "false" {
		cacheClient = metadata.EnsureRegistered(h.settingsRepo)
	}

	updated := 0
	var lastBroadcast time.Time
	for i, item := range items {
		select {
		case <-ctx.Done():
			log.Printf("Metadata: cancelled after %d/%d items", i, len(items))
			return ctx.Err()
		default:
		}

		// Broadcast progress
		if h.notifier != nil {
			now := time.Now()
			if now.Sub(lastBroadcast) >= 500*time.Millisecond || i == len(items)-1 {
				lastBroadcast = now
				pct := int(float64(i+1) / float64(len(items)) * 100)
				desc := fmt.Sprintf("Metadata refresh: %s · %s (%d/%d)", library.Name, item.Title, i+1, len(items))
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskMetadataScrape,
					"status": "running", "progress": pct, "description": desc,
				})
			}
		}

		// Skip types that don't support auto-match
		if !metadata.ShouldAutoMatch(item.MediaType) {
			continue
		}

		// Clean the title and find best match
		query := metadata.CleanTitleForSearch(item.Title)
		if query == "" {
			query = item.Title
		}

		var yearHint *int
		if item.Year != nil && *item.Year > 0 {
			yearHint = item.Year
		}

		// ── Try cache server first ──
		if cacheClient != nil {
			result := cacheClient.Lookup(query, yearHint, item.MediaType)
			if result != nil && result.Match != nil {
				best := result.Match

				// Download poster
				var posterPath *string
				if best.PosterURL != nil && *best.PosterURL != "" && h.cfg.Paths.Preview != "" {
					filename := item.ID.String() + ".jpg"
					_, dlErr := metadata.DownloadPoster(*best.PosterURL, filepath.Join(h.cfg.Paths.Preview, "posters"), filename)
					if dlErr == nil {
						webPath := "/previews/posters/" + filename
						posterPath = &webPath
					}
				}

				// Apply metadata
				if err := h.mediaRepo.UpdateMetadata(item.ID, best.Title, best.Year, best.Description, best.Rating, posterPath, best.ContentRating); err != nil {
					log.Printf("Metadata: update failed for %s: %v", item.FileName, err)
				} else {
					// Apply cached ratings
					if result.Ratings != nil {
						_ = h.mediaRepo.UpdateRatings(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore)
					}
					// Store external IDs from cache
					if result.ExternalIDsJSON != nil {
						_ = h.mediaRepo.UpdateExternalIDs(item.ID, *result.ExternalIDsJSON)
					}
					updated++
				}
				// No delay needed for cache hits — it's our own server
				continue
			}
			// Cache miss – fall through to direct TMDB
		}

		best := metadata.FindBestMatch(h.scrapers, query, item.MediaType, yearHint)
		if best == nil {
			continue
		}

		// Enrich with source-specific details
		if best.Source == "tmdb" {
			for _, sc := range h.scrapers {
				if tmdb, ok := sc.(*metadata.TMDBScraper); ok {
					if details, err := tmdb.GetDetails(best.ExternalID); err == nil {
						if details.ContentRating != nil {
							best.ContentRating = details.ContentRating
						}
						if details.IMDBId != "" {
							best.IMDBId = details.IMDBId
						}
						// Merge genres from details
						if len(details.Genres) > 0 && len(best.Genres) == 0 {
							best.Genres = details.Genres
						}
					}
					break
				}
			}
		} else if best.Source == "musicbrainz" || best.Source == "openlibrary" {
			// Fetch full details from non-TMDB sources
			for _, sc := range h.scrapers {
				if sc.Name() == best.Source {
					if details, err := sc.GetDetails(best.ExternalID); err == nil {
						if details.Description != nil && best.Description == nil {
							best.Description = details.Description
						}
						if len(details.Genres) > 0 && len(best.Genres) == 0 {
							best.Genres = details.Genres
						}
						if details.PosterURL != nil && best.PosterURL == nil {
							best.PosterURL = details.PosterURL
						}
					}
					break
				}
			}
		}

		// Download poster if available
		var posterPath *string
		if best.PosterURL != nil && *best.PosterURL != "" && h.cfg.Paths.Preview != "" {
			filename := item.ID.String() + ".jpg"
			_, dlErr := metadata.DownloadPoster(*best.PosterURL, filepath.Join(h.cfg.Paths.Preview, "posters"), filename)
			if dlErr == nil {
				webPath := "/previews/posters/" + filename
				posterPath = &webPath
			}
		}

		// Apply metadata
		if err := h.mediaRepo.UpdateMetadata(item.ID, best.Title, best.Year, best.Description, best.Rating, posterPath, best.ContentRating); err != nil {
			log.Printf("Metadata: update failed for %s: %v", item.FileName, err)
			continue
		}

		// Fetch OMDb ratings if we have an IMDB ID (TMDB sources only)
		if best.IMDBId != "" && omdbKey != "" {
			ratings, omdbErr := metadata.FetchOMDbRatings(best.IMDBId, omdbKey)
			if omdbErr == nil {
				_ = h.mediaRepo.UpdateRatings(item.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore)
			}
		}

		// Store external IDs from direct match
		idsJSON := metadata.BuildExternalIDsFromMatch(best.Source, best.ExternalID, best.IMDBId, false)
		if idsJSON != nil {
			_ = h.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
		}

		// Contribute to cache in background (all sources)
		if cacheClient != nil {
			go cacheClient.Contribute(best)
		}

		updated++
		// Rate-limit API calls
		time.Sleep(300 * time.Millisecond)
	}

	log.Printf("Metadata: updated %d/%d items in %q", updated, len(items), library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskMetadataScrape,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}

// ──────── Metadata Refresh Handler ────────
// Unlike MetadataScrape (which only fills missing metadata), Refresh clears
// all existing metadata for unlocked items, re-queries sources, and generates
// a screenshot poster when no external match is found.

type MetadataRefreshHandler struct {
	mediaRepo    *repository.MediaRepository
	libRepo      *repository.LibraryRepository
	settingsRepo *repository.SettingsRepository
	scrapers     []metadata.Scraper
	cfg          *config.Config
	scanner      *scanner.Scanner
	notifier     EventNotifier
}

func NewMetadataRefreshHandler(mediaRepo *repository.MediaRepository, libRepo *repository.LibraryRepository,
	settingsRepo *repository.SettingsRepository, scrapers []metadata.Scraper,
	cfg *config.Config, sc *scanner.Scanner, notifier EventNotifier) *MetadataRefreshHandler {
	return &MetadataRefreshHandler{
		mediaRepo: mediaRepo, libRepo: libRepo, settingsRepo: settingsRepo,
		scrapers: scrapers, cfg: cfg, scanner: sc, notifier: notifier,
	}
}

func (h *MetadataRefreshHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload struct {
		LibraryID string `json:"library_id"`
	}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	libID, _ := uuid.Parse(payload.LibraryID)
	library, err := h.libRepo.GetByID(libID)
	if err != nil {
		return fmt.Errorf("get library: %w", err)
	}

	taskID := "metadata-refresh:" + payload.LibraryID
	taskDesc := "Metadata refresh: " + library.Name

	// Get ALL items in the library (we check lock status per-item)
	items, err := h.mediaRepo.ListAllByLibrary(libID)
	if err != nil {
		return fmt.Errorf("list library items: %w", err)
	}
	if len(items) == 0 {
		log.Printf("Metadata refresh: no items in library %s", library.Name)
		return nil
	}

	log.Printf("Metadata refresh: processing %d items in %q", len(items), library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskMetadataRefresh,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	// Fetch OMDb API key once
	omdbKey, _ := h.settingsRepo.Get("omdb_api_key")

	// Check cache server
	var cacheClient *metadata.CacheClient
	cacheEnabled, _ := h.settingsRepo.Get("cache_server_enabled")
	if cacheEnabled != "false" {
		cacheClient = metadata.EnsureRegistered(h.settingsRepo)
	}

	updated := 0
	cleared := 0
	generated := 0
	var lastBroadcast time.Time

	for i, item := range items {
		select {
		case <-ctx.Done():
			log.Printf("Metadata refresh: cancelled after %d/%d items", i, len(items))
			return ctx.Err()
		default:
		}

		// Broadcast progress
		if h.notifier != nil {
			now := time.Now()
			if now.Sub(lastBroadcast) >= 500*time.Millisecond || i == len(items)-1 {
				lastBroadcast = now
				pct := int(float64(i+1) / float64(len(items)) * 100)
				desc := fmt.Sprintf("Metadata refresh: %s · %s (%d/%d)", library.Name, item.Title, i+1, len(items))
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskMetadataRefresh,
					"status": "running", "progress": pct, "description": desc,
				})
			}
		}

		// Skip locked items
		if item.MetadataLocked {
			continue
		}

		// Skip types that don't support auto-match
		if !metadata.ShouldAutoMatch(item.MediaType) {
			continue
		}

		// ── Step 1: Clear existing metadata ──
		fileTitle := metadata.TitleFromFilename(item.FileName)
		if fileTitle == "" {
			fileTitle = item.FileName
		}
		if err := h.mediaRepo.ClearItemMetadata(item.ID, fileTitle); err != nil {
			log.Printf("Metadata refresh: clear failed for %s: %v", item.FileName, err)
			continue
		}
		// Remove genre tags and performer links
		_ = h.mediaRepo.RemoveAllMediaTags(item.ID)
		_ = h.mediaRepo.RemoveAllMediaPerformers(item.ID)
		cleared++

		// Use filename-derived title and year for the search
		query := metadata.CleanTitleForSearch(fileTitle)
		if query == "" {
			query = fileTitle
		}
		yearHint := metadata.YearFromFilename(item.FileName)

		// ── Step 2: Re-query (cache first) ──
		matched := false

		if cacheClient != nil {
			result := cacheClient.Lookup(query, yearHint, item.MediaType)
			if result != nil && result.Match != nil {
				best := result.Match

				// Download poster
				var posterPath *string
				if best.PosterURL != nil && *best.PosterURL != "" && h.cfg.Paths.Preview != "" {
					filename := item.ID.String() + ".jpg"
					posterDir := filepath.Join(h.cfg.Paths.Preview, "posters")
					// Remove old poster file so dedup doesn't save new one as _alt
					_ = removeExistingPosters(posterDir, filename)
					_, dlErr := metadata.DownloadPoster(*best.PosterURL, posterDir, filename)
					if dlErr == nil {
						webPath := "/previews/posters/" + filename
						posterPath = &webPath
					}
				}

				// Apply metadata
				if err := h.mediaRepo.UpdateMetadata(item.ID, best.Title, best.Year, best.Description, best.Rating, posterPath, best.ContentRating); err != nil {
					log.Printf("Metadata refresh: update failed for %s: %v", item.FileName, err)
				} else {
					if result.Ratings != nil {
						_ = h.mediaRepo.UpdateRatings(item.ID, result.Ratings.IMDBRating, result.Ratings.RTScore, result.Ratings.AudienceScore)
					}
					if result.ExternalIDsJSON != nil {
						_ = h.mediaRepo.UpdateExternalIDs(item.ID, *result.ExternalIDsJSON)
					}
					// Link genres from cache
					if len(result.Genres) > 0 {
						h.linkGenreTags(item.ID, result.Genres)
					}
					// Apply cast/crew from cache
					if result.CastCrewJSON != nil && *result.CastCrewJSON != "" {
						credits := metadata.ParseCacheCredits(*result.CastCrewJSON)
						if credits != nil {
							h.enrichWithCredits(item.ID, credits)
						}
					}
					updated++
					matched = true
				}
				if matched {
					continue
				}
			}
		}

		// ── Cache miss: fall through to direct scraper ──
		best := metadata.FindBestMatch(h.scrapers, query, item.MediaType, yearHint)
		if best != nil {
			// Enrich with source-specific details
			if best.Source == "tmdb" {
				for _, sc := range h.scrapers {
					if tmdb, ok := sc.(*metadata.TMDBScraper); ok {
						if details, err := tmdb.GetDetails(best.ExternalID); err == nil {
							if details.ContentRating != nil {
								best.ContentRating = details.ContentRating
							}
							if details.IMDBId != "" {
								best.IMDBId = details.IMDBId
							}
							if len(details.Genres) > 0 && len(best.Genres) == 0 {
								best.Genres = details.Genres
							}
						}
						break
					}
				}
			} else if best.Source == "musicbrainz" || best.Source == "openlibrary" {
				for _, sc := range h.scrapers {
					if sc.Name() == best.Source {
						if details, err := sc.GetDetails(best.ExternalID); err == nil {
							if details.Description != nil && best.Description == nil {
								best.Description = details.Description
							}
							if len(details.Genres) > 0 && len(best.Genres) == 0 {
								best.Genres = details.Genres
							}
							if details.PosterURL != nil && best.PosterURL == nil {
								best.PosterURL = details.PosterURL
							}
						}
						break
					}
				}
			}

			// Download poster
			var posterPath *string
			if best.PosterURL != nil && *best.PosterURL != "" && h.cfg.Paths.Preview != "" {
				filename := item.ID.String() + ".jpg"
				posterDir := filepath.Join(h.cfg.Paths.Preview, "posters")
				_ = removeExistingPosters(posterDir, filename)
				_, dlErr := metadata.DownloadPoster(*best.PosterURL, posterDir, filename)
				if dlErr == nil {
					webPath := "/previews/posters/" + filename
					posterPath = &webPath
				}
			}

			// Apply metadata
			if err := h.mediaRepo.UpdateMetadata(item.ID, best.Title, best.Year, best.Description, best.Rating, posterPath, best.ContentRating); err != nil {
				log.Printf("Metadata refresh: update failed for %s: %v", item.FileName, err)
			} else {
				// Link genres
				if len(best.Genres) > 0 {
					h.linkGenreTags(item.ID, best.Genres)
				}

				// OMDb ratings
				if best.IMDBId != "" && omdbKey != "" {
					ratings, omdbErr := metadata.FetchOMDbRatings(best.IMDBId, omdbKey)
					if omdbErr == nil {
						_ = h.mediaRepo.UpdateRatings(item.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore)
					}
				}

				// Store external IDs
				idsJSON := metadata.BuildExternalIDsFromMatch(best.Source, best.ExternalID, best.IMDBId, false)
				if idsJSON != nil {
					_ = h.mediaRepo.UpdateExternalIDs(item.ID, *idsJSON)
				}

				// Contribute to cache
				if cacheClient != nil {
					go cacheClient.Contribute(best)
				}

				updated++
				matched = true
			}

			// Rate-limit API calls
			time.Sleep(300 * time.Millisecond)
		}

		// ── Step 3: No match — generate screenshot poster ──
		if !matched && h.scanner != nil && h.scanner.IsProbeableType(item.MediaType) {
			// Re-read the item to get current state (metadata was cleared)
			freshItem, err := h.mediaRepo.GetByID(item.ID)
			if err == nil && freshItem.PosterPath == nil {
				h.scanner.GenerateScreenshotPoster(freshItem)
				generated++
			}
		}
	}

	log.Printf("Metadata refresh: cleared %d, matched %d, generated posters %d in %q",
		cleared, updated, generated, library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskMetadataRefresh,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}

// linkGenreTags creates genre tags and links them to a media item.
func (h *MetadataRefreshHandler) linkGenreTags(mediaItemID uuid.UUID, genres []string) {
	for _, genre := range genres {
		// Look for existing tag
		var tagID uuid.UUID
		row := h.mediaRepo.DB().QueryRow(
			`SELECT id FROM tags WHERE category = 'genre' AND LOWER(name) = LOWER($1)`, genre)
		if err := row.Scan(&tagID); err != nil {
			// Create it
			tagID = uuid.New()
			h.mediaRepo.DB().Exec(
				`INSERT INTO tags (id, name, slug, category) VALUES ($1, $2, $3, 'genre') ON CONFLICT DO NOTHING`,
				tagID, genre, strings.ToLower(strings.ReplaceAll(genre, " ", "-")))
			// Re-read in case of race
			_ = h.mediaRepo.DB().QueryRow(
				`SELECT id FROM tags WHERE category = 'genre' AND LOWER(name) = LOWER($1)`, genre).Scan(&tagID)
		}
		h.mediaRepo.DB().Exec(
			`INSERT INTO media_tags (id, media_item_id, tag_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			uuid.New(), mediaItemID, tagID)
	}
}

// enrichWithCredits creates performers from credits and links them to a media item.
func (h *MetadataRefreshHandler) enrichWithCredits(mediaItemID uuid.UUID, credits *metadata.TMDBCredits) {
	if credits == nil {
		return
	}
	maxCast := 20
	if len(credits.Cast) < maxCast {
		maxCast = len(credits.Cast)
	}
	for i := 0; i < maxCast; i++ {
		member := credits.Cast[i]
		if member.Name == "" {
			continue
		}
		perfID := h.findOrCreatePerformer(member.Name, "actor", member.ProfilePath)
		if perfID != uuid.Nil {
			h.mediaRepo.DB().Exec(
				`INSERT INTO media_performers (id, media_item_id, performer_id, role, character_name, sort_order)
				 VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
				uuid.New(), mediaItemID, perfID, "actor", member.Character, member.Order)
		}
	}
	importedCrew := 0
	for _, member := range credits.Crew {
		if member.Name == "" {
			continue
		}
		var role string
		var perfType string
		switch member.Job {
		case "Director":
			role, perfType = "director", "director"
		case "Producer", "Executive Producer":
			role, perfType = strings.ToLower(member.Job), "producer"
		case "Screenplay", "Writer", "Story":
			role, perfType = strings.ToLower(member.Job), "other"
		default:
			continue
		}
		perfID := h.findOrCreatePerformer(member.Name, perfType, member.ProfilePath)
		if perfID != uuid.Nil {
			h.mediaRepo.DB().Exec(
				`INSERT INTO media_performers (id, media_item_id, performer_id, role, character_name, sort_order)
				 VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
				uuid.New(), mediaItemID, perfID, role, "", 100+importedCrew)
			importedCrew++
		}
	}
}

func (h *MetadataRefreshHandler) findOrCreatePerformer(name, perfType, profilePath string) uuid.UUID {
	var perfID uuid.UUID
	err := h.mediaRepo.DB().QueryRow(
		`SELECT id FROM performers WHERE LOWER(name) = LOWER($1)`, name).Scan(&perfID)
	if err == nil {
		return perfID
	}
	perfID = uuid.New()
	var photoPath *string
	if profilePath != "" && h.cfg.Paths.Preview != "" {
		photoURL := "https://image.tmdb.org/t/p/w185" + profilePath
		filename := "performer_" + perfID.String() + ".jpg"
		if _, dlErr := metadata.DownloadPoster(photoURL, filepath.Join(h.cfg.Paths.Preview, "posters"), filename); dlErr == nil {
			wp := "/previews/posters/" + filename
			photoPath = &wp
		}
	}
	_, err = h.mediaRepo.DB().Exec(
		`INSERT INTO performers (id, name, performer_type, photo_path) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		perfID, name, perfType, photoPath)
	if err != nil {
		return uuid.Nil
	}
	return perfID
}

// removeExistingPosters deletes old poster files for an item so fresh downloads aren't saved as _alt.
func removeExistingPosters(dir, filename string) error {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	pattern := filepath.Join(dir, base+"*"+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, m := range matches {
		_ = os.Remove(m)
	}
	return nil
}

// ──────── Register all handlers ────────

func RegisterHandlers(q *Queue, sc *scanner.Scanner, libRepo *repository.LibraryRepository,
	mediaRepo *repository.MediaRepository, jobRepo *repository.JobRepository,
	fp *fingerprint.Fingerprinter, notifier EventNotifier,
	scrapers []metadata.Scraper, settingsRepo *repository.SettingsRepository, cfg *config.Config) {

	q.RegisterHandler(TaskScanLibrary, NewScanHandler(sc, libRepo, jobRepo, q, notifier))
	q.RegisterHandler(TaskFingerprint, NewFingerprintHandler(mediaRepo))
	q.RegisterHandler(TaskPhashLibrary, NewPhashLibraryHandler(mediaRepo, fp, notifier))
	q.RegisterHandler(TaskGeneratePreview, NewPreviewHandler())
	q.RegisterHandler(TaskMetadataScrape, NewMetadataScrapeHandler(mediaRepo, libRepo, settingsRepo, scrapers, cfg, notifier))
	q.RegisterHandler(TaskMetadataRefresh, NewMetadataRefreshHandler(mediaRepo, libRepo, settingsRepo, scrapers, cfg, sc, notifier))

	// Ignore unused import
	_ = models.JobPending
}
