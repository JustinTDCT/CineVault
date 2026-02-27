package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/JustinTDCT/CineVault/internal/detection"
	ffmpegPkg "github.com/JustinTDCT/CineVault/internal/ffmpeg"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/JustinTDCT/CineVault/internal/scanner"
)

type ScanHandler struct {
	scanner      *scanner.Scanner
	libRepo      *repository.LibraryRepository
	jobRepo      *repository.JobRepository
	settingsRepo *repository.SettingsRepository
	queue        *Queue
	notifier     EventNotifier
}

func NewScanHandler(sc *scanner.Scanner, libRepo *repository.LibraryRepository, jobRepo *repository.JobRepository, settingsRepo *repository.SettingsRepository, queue *Queue, notifier EventNotifier) *ScanHandler {
	return &ScanHandler{scanner: sc, libRepo: libRepo, jobRepo: jobRepo, settingsRepo: settingsRepo, queue: queue, notifier: notifier}
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
		progressFn = func(current, total, added int, filename string) {
			now := time.Now()
			// Throttle: broadcast at most every 500ms, plus always on last item
			if now.Sub(lastBroadcast) >= 500*time.Millisecond || current == total {
				lastBroadcast = now
				pct := 0
				if total > 0 {
					pct = int(float64(current) / float64(total) * 100)
				}
				h.notifier.Broadcast("scan:progress", map[string]interface{}{
					"library_id":  p.LibraryID,
					"current":     current,
					"total":       total,
					"files_added": added,
					"filename":    filename,
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

	if library.MediaType == models.MediaTypeMusic {
		if removed, err := h.scanner.CleanupMusicDuplicates(libID); err != nil {
			log.Printf("Job: music duplicate cleanup error: %v", err)
		} else if removed > 0 {
			log.Printf("Job: music cleanup removed %d duplicate albums", removed)
		}
	}

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
	// Only enqueue if the Find Duplicates setting is enabled (default: on)
	if h.queue != nil {
		dupEnabled := true
		if h.settingsRepo != nil {
			if val, err := h.settingsRepo.Get("find_duplicates_enabled"); err == nil && val == "false" {
				dupEnabled = false
			}
		}
		if dupEnabled {
			uniqueID := "phash:" + p.LibraryID
			if _, err := h.queue.EnqueueUnique(TaskPhashLibrary, PhashLibraryPayload{LibraryID: p.LibraryID}, uniqueID,
				asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour)); err != nil {
				log.Printf("Job: failed to enqueue phash job for library %s: %v", p.LibraryID, err)
			} else {
				log.Printf("Job: enqueued phash computation for library %s", p.LibraryID)
			}
		} else {
			log.Printf("Job: skipping phash computation for library %s (Find Duplicates disabled)", p.LibraryID)
		}

		// Enqueue timeline thumbnail generation as a follow-up background job
		spritesEnabled := true
		if h.settingsRepo != nil {
			if val, err := h.settingsRepo.Get("create_timeline_thumbnails_enabled"); err == nil && val == "false" {
				spritesEnabled = false
			}
		}
		if spritesEnabled && !library.CreateThumbnails {
			spritesEnabled = false
			log.Printf("Job: skipping thumbnails for library %s (disabled per-library)", p.LibraryID)
		}
		if spritesEnabled {
			uniqueID := "sprites:" + p.LibraryID
			if _, err := h.queue.EnqueueUnique(TaskSpritesLibrary, SpritesLibraryPayload{LibraryID: p.LibraryID}, uniqueID,
				asynq.Timeout(12*time.Hour), asynq.Retention(1*time.Hour)); err != nil {
				log.Printf("Job: failed to enqueue sprites job for library %s: %v", p.LibraryID, err)
			} else {
				log.Printf("Job: enqueued timeline thumbnails for library %s", p.LibraryID)
			}
		}

		// Enqueue preview clip generation as a follow-up background job
		previewsEnabled := true
		if h.settingsRepo != nil {
			if val, err := h.settingsRepo.Get("create_previews_enabled"); err == nil && val == "false" {
				previewsEnabled = false
			}
		}
		if previewsEnabled && !library.CreatePreviews {
			previewsEnabled = false
			log.Printf("Job: skipping previews for library %s (disabled per-library)", p.LibraryID)
		}
		if previewsEnabled {
			uniqueID := "previews:" + p.LibraryID
			if _, err := h.queue.EnqueueUnique(TaskPreviewsLibrary, PreviewsLibraryPayload{LibraryID: p.LibraryID}, uniqueID,
				asynq.Timeout(12*time.Hour), asynq.Retention(1*time.Hour)); err != nil {
				log.Printf("Job: failed to enqueue previews job for library %s: %v", p.LibraryID, err)
			} else {
				log.Printf("Job: enqueued preview clips for library %s", p.LibraryID)
			}
		}

		// Enqueue loudness analysis if the library has audio normalization enabled
		if library.AudioNormalization {
			uniqueID := "loudness:" + p.LibraryID
			if _, err := h.queue.EnqueueUnique(TaskLoudnessLibrary, LoudnessLibraryPayload{LibraryID: p.LibraryID}, uniqueID,
				asynq.Timeout(12*time.Hour), asynq.Retention(1*time.Hour)); err != nil {
				log.Printf("Job: failed to enqueue loudness job for library %s: %v", p.LibraryID, err)
			} else {
				log.Printf("Job: enqueued loudness analysis for library %s", p.LibraryID)
			}
		}
	}

	return nil
}

// ──────── Segment Detection Handler ────────

type DetectSegmentsHandler struct {
	detector    *detection.Detector
	segmentRepo *repository.SegmentRepository
	libRepo     *repository.LibraryRepository
	notifier    EventNotifier
}

func NewDetectSegmentsHandler(det *detection.Detector, segRepo *repository.SegmentRepository,
	libRepo *repository.LibraryRepository, notifier EventNotifier) *DetectSegmentsHandler {
	return &DetectSegmentsHandler{
		detector: det, segmentRepo: segRepo, libRepo: libRepo, notifier: notifier,
	}
}

func (h *DetectSegmentsHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p DetectSegmentsPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	libID, _ := uuid.Parse(p.LibraryID)
	library, err := h.libRepo.GetByID(libID)
	if err != nil {
		return fmt.Errorf("get library: %w", err)
	}

	taskID := "detect:" + p.LibraryID
	taskDesc := "Detecting skip segments: " + library.Name

	log.Printf("Detect: starting segment detection for library %q", library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskDetectSegments,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	// Phase 1: Cross-episode intro detection for TV seasons
	seasonIDs, err := h.segmentRepo.ListSeasonIDsInLibrary(libID)
	if err != nil {
		log.Printf("Detect: failed to list seasons: %v", err)
	}

	seasonsDone := 0
	for _, seasonID := range seasonIDs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		episodes, err := h.segmentRepo.ListEpisodesBySeasonID(seasonID)
		if err != nil || len(episodes) < 2 {
			continue
		}

		introResults := h.detector.DetectIntros(episodes)
		for mediaID, seg := range introResults {
			seg.MediaItemID = mediaID
			if err := h.segmentRepo.Upsert(seg); err != nil {
				log.Printf("Detect: failed to save intro for %s: %v", mediaID, err)
			}
		}
		seasonsDone++

		if h.notifier != nil {
			pct := int(float64(seasonsDone) / float64(len(seasonIDs)) * 40)
			desc := fmt.Sprintf("Detecting intros: %s (%d/%d seasons)", library.Name, seasonsDone, len(seasonIDs))
			h.notifier.Broadcast("task:update", map[string]interface{}{
				"task_id": taskID, "task_type": TaskDetectSegments,
				"status": "running", "progress": pct, "description": desc,
			})
		}
	}
	log.Printf("Detect: completed intro detection for %d seasons", seasonsDone)

	// Phase 2: Per-item credits, anime, and recap detection
	mediaTypes := []string{"tv_shows", "movies"}
	items, err := h.segmentRepo.ListItemsWithoutSegments(libID, mediaTypes)
	if err != nil {
		log.Printf("Detect: failed to list items: %v", err)
		items = nil
	}

	// Also include items that only have intro (need credits/recap)
	allItems, err := h.segmentRepo.ListItemsWithoutSegments(libID, mediaTypes)
	if err == nil {
		items = allItems
	}

	itemsDone := 0
	var lastBroadcast time.Time
	for _, item := range items {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		result := h.detector.DetectAll(item)
		for _, seg := range result.Segments {
			if err := h.segmentRepo.Upsert(seg); err != nil {
				log.Printf("Detect: failed to save %s for %s: %v", seg.SegmentType, item.FileName, err)
			}
		}
		itemsDone++

		if h.notifier != nil {
			now := time.Now()
			if now.Sub(lastBroadcast) >= 500*time.Millisecond || itemsDone == len(items) {
				lastBroadcast = now
				pct := 40 + int(float64(itemsDone)/float64(len(items))*60)
				desc := fmt.Sprintf("Detecting segments: %s · %s (%d/%d)", library.Name, item.FileName, itemsDone, len(items))
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskDetectSegments,
					"status": "running", "progress": pct, "description": desc,
				})
			}
		}
	}

	log.Printf("Detect: completed — %d seasons, %d items processed in %q", seasonsDone, itemsDone, library.Name)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskDetectSegments,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}

// ──────── Loudness Analysis Handler ────────

type LoudnessLibraryHandler struct {
	mediaRepo *repository.MediaRepository
	libRepo   *repository.LibraryRepository
	notifier  EventNotifier
	ffmpegPath string
}

func NewLoudnessLibraryHandler(mediaRepo *repository.MediaRepository, libRepo *repository.LibraryRepository, notifier EventNotifier, ffmpegPath string) *LoudnessLibraryHandler {
	return &LoudnessLibraryHandler{mediaRepo: mediaRepo, libRepo: libRepo, notifier: notifier, ffmpegPath: ffmpegPath}
}

func (h *LoudnessLibraryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p LoudnessLibraryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	libID, _ := uuid.Parse(p.LibraryID)

	// Verify the library still has normalization enabled
	if h.libRepo != nil {
		lib, err := h.libRepo.GetByID(libID)
		if err != nil {
			return fmt.Errorf("get library: %w", err)
		}
		if !lib.AudioNormalization {
			log.Printf("Loudness: skipping for library %s (Audio Normalization disabled)", p.LibraryID)
			return nil
		}
	}

	taskID := "loudness:" + p.LibraryID
	taskDesc := "Analyzing audio loudness"

	items, err := h.mediaRepo.ListItemsNeedingLoudness(libID)
	if err != nil {
		return fmt.Errorf("list items needing loudness: %w", err)
	}

	if len(items) == 0 {
		log.Printf("Loudness: all items already analyzed in library %s", p.LibraryID)
		return nil
	}

	log.Printf("Loudness: analyzing %d items in library %s", len(items), p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskLoudnessLibrary,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	analyzed := 0
	var lastTaskBroadcast time.Time
	for i, item := range items {
		select {
		case <-ctx.Done():
			log.Printf("Loudness: cancelled after %d/%d items", analyzed, len(items))
			return ctx.Err()
		default:
		}

		ffPath := h.ffmpegPath
		if ffPath == "" {
			ffPath = "ffmpeg"
		}

		result, err := ffmpegPkg.AnalyzeLoudness(ffPath, item.FilePath)
		if err != nil {
			log.Printf("Loudness: failed for %s: %v", item.FileName, err)
			continue
		}

		if err := h.mediaRepo.UpdateLoudness(item.ID, result.InputI, result.GainDB); err != nil {
			log.Printf("Loudness: failed to store for %s: %v", item.FileName, err)
			continue
		}
		analyzed++

		if h.notifier != nil {
			now := time.Now()
			if now.Sub(lastTaskBroadcast) >= 500*time.Millisecond || i == len(items)-1 {
				lastTaskBroadcast = now
				pct := int(float64(i+1) / float64(len(items)) * 100)
				desc := fmt.Sprintf("Audio loudness · %s (%d/%d)", item.FileName, i+1, len(items))
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskLoudnessLibrary,
					"status": "running", "progress": pct, "description": desc,
				})
			}
		}
	}

	log.Printf("Loudness: analyzed %d items in library %s", analyzed, p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskLoudnessLibrary,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}
