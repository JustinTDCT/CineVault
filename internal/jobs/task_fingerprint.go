package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/JustinTDCT/CineVault/internal/fingerprint"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
)

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
	mediaRepo     *repository.MediaRepository
	settingsRepo  *repository.SettingsRepository
	fingerprinter *fingerprint.Fingerprinter
	notifier      EventNotifier
}

func NewPhashLibraryHandler(mediaRepo *repository.MediaRepository, settingsRepo *repository.SettingsRepository, fp *fingerprint.Fingerprinter, notifier EventNotifier) *PhashLibraryHandler {
	return &PhashLibraryHandler{mediaRepo: mediaRepo, settingsRepo: settingsRepo, fingerprinter: fp, notifier: notifier}
}

func (h *PhashLibraryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p PhashLibraryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	// Check if Find Duplicates is enabled (default: on)
	if h.settingsRepo != nil {
		if val, err := h.settingsRepo.Get("find_duplicates_enabled"); err == nil && val == "false" {
			log.Printf("Phash: skipping phash computation for library %s (Find Duplicates disabled)", p.LibraryID)
			return nil
		}
	}

	taskID := "phash:" + p.LibraryID
	taskDesc := "Analyzing duplicates"

	libID, _ := uuid.Parse(p.LibraryID)

	items, err := h.mediaRepo.ListItemsNeedingPhash(libID)
	if err != nil {
		return fmt.Errorf("list items needing phash: %w", err)
	}

	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskPhashLibrary,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	var computed int64
	if len(items) > 0 {
		log.Printf("Phash: computing phash for %d items in library %s (4 workers, hwaccel enabled)", len(items), p.LibraryID)

		const phashWorkers = 4
		total := int64(len(items))
		var processed int64

		work := make(chan *models.MediaItem, phashWorkers)
		var wg sync.WaitGroup

		progressDone := make(chan struct{})
		go func() {
			defer close(progressDone)
			if h.notifier == nil {
				return
			}
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					done := atomic.LoadInt64(&processed)
					pct := int(float64(done) / float64(total) * 100)
					desc := fmt.Sprintf("Analyzing duplicates · %d/%d", done, total)
					h.notifier.Broadcast("task:update", map[string]interface{}{
						"task_id": taskID, "task_type": TaskPhashLibrary,
						"status": "running", "progress": pct, "description": desc,
					})
					if done >= total {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		for w := 0; w < phashWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range work {
					dur := 0
					if item.DurationSeconds != nil {
						dur = *item.DurationSeconds
					}
					phash, err := h.fingerprinter.ComputePHash(item.FilePath, dur)
					if err != nil {
						log.Printf("Phash: failed for %s: %v", item.FileName, err)
						atomic.AddInt64(&processed, 1)
						continue
					}
					if err := h.mediaRepo.UpdatePhash(item.ID, phash); err != nil {
						log.Printf("Phash: failed to store for %s: %v", item.FileName, err)
						atomic.AddInt64(&processed, 1)
						continue
					}
					atomic.AddInt64(&computed, 1)
					atomic.AddInt64(&processed, 1)
				}
			}()
		}

		for _, item := range items {
			select {
			case <-ctx.Done():
				log.Printf("Phash: cancelled after %d/%d items", atomic.LoadInt64(&processed), total)
				close(work)
				wg.Wait()
				<-progressDone
				return ctx.Err()
			case work <- item:
			}
		}
		close(work)
		wg.Wait()
		<-progressDone
	} else {
		log.Printf("Phash: all items already hashed in library %s, proceeding to comparison", p.LibraryID)
	}

	log.Printf("Phash: computed %d new hashes, checking for potential duplicates", atomic.LoadInt64(&computed))

	// Check all phash items in the library for potential duplicates
	allHashed, err := h.mediaRepo.ListPhashesInLibrary(libID)
	if err != nil {
		return fmt.Errorf("list phashes: %w", err)
	}

	log.Printf("Phash: comparing %d hashed items for duplicates in library %s", len(allHashed), p.LibraryID)

	// Log hash length distribution for debugging
	lengthCounts := make(map[int]int)
	for _, item := range allHashed {
		if item.Phash != nil {
			lengthCounts[len(*item.Phash)]++
		}
	}
	for l, c := range lengthCounts {
		log.Printf("Phash: %d items have hash length %d chars", c, l)
	}

	dupCount := 0
	comparisons := 0
	skippedLen := 0
	skippedDur := 0
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
						skippedDur++
						continue
					}
				}
			}

			// Check hash length compatibility
			if len(*allHashed[i].Phash) != len(*allHashed[j].Phash) {
				skippedLen++
				continue
			}

			comparisons++
			sim := fingerprint.Similarity(*allHashed[i].Phash, *allHashed[j].Phash)
			if sim >= 0.90 {
				log.Printf("Phash: DUPLICATE FOUND sim=%.3f: %q <-> %q", sim, allHashed[i].FileName, allHashed[j].FileName)
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

	log.Printf("Phash: %d comparisons, %d skipped (duration), %d skipped (hash length mismatch), %d duplicate pairs found in library %s",
		comparisons, skippedDur, skippedLen, dupCount, p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("phash:complete", map[string]interface{}{
			"library_id":      p.LibraryID,
			"computed":        atomic.LoadInt64(&computed),
			"duplicate_pairs": dupCount,
		})
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskPhashLibrary,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}
