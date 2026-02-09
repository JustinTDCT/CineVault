package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
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

		best := metadata.FindBestMatch(h.scrapers, query, item.MediaType, yearHint)
		if best == nil {
			continue
		}

		// Enrich with TMDB details (content_rating, full genres, IMDB ID)
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

		// Fetch OMDb ratings if we have an IMDB ID
		if best.IMDBId != "" && omdbKey != "" {
			ratings, omdbErr := metadata.FetchOMDbRatings(best.IMDBId, omdbKey)
			if omdbErr == nil {
				_ = h.mediaRepo.UpdateRatings(item.ID, ratings.IMDBRating, ratings.RTScore, ratings.AudienceScore)
			}
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

	// Ignore unused import
	_ = models.JobPending
}
