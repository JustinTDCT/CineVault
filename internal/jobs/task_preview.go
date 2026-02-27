package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/JustinTDCT/CineVault/internal/scanner"
)

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

// ──────── Sprites Library Handler ────────

type SpritesLibraryHandler struct {
	mediaRepo    *repository.MediaRepository
	libRepo      *repository.LibraryRepository
	settingsRepo *repository.SettingsRepository
	sc           *scanner.Scanner
	notifier     EventNotifier
}

func NewSpritesLibraryHandler(mediaRepo *repository.MediaRepository, libRepo *repository.LibraryRepository, settingsRepo *repository.SettingsRepository, sc *scanner.Scanner, notifier EventNotifier) *SpritesLibraryHandler {
	return &SpritesLibraryHandler{mediaRepo: mediaRepo, libRepo: libRepo, settingsRepo: settingsRepo, sc: sc, notifier: notifier}
}

func (h *SpritesLibraryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p SpritesLibraryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	// Global kill-switch
	if h.settingsRepo != nil {
		if val, err := h.settingsRepo.Get("create_timeline_thumbnails_enabled"); err == nil && val == "false" {
			log.Printf("Sprites: skipping for library %s (Timeline Thumbnails globally disabled)", p.LibraryID)
			return nil
		}
	}

	// Per-library setting
	if h.libRepo != nil {
		libID, _ := uuid.Parse(p.LibraryID)
		if lib, err := h.libRepo.GetByID(libID); err == nil && !lib.CreateThumbnails {
			log.Printf("Sprites: skipping for library %s (Timeline Thumbnails disabled for this library)", p.LibraryID)
			return nil
		}
	}

	taskID := "sprites:" + p.LibraryID
	taskDesc := "Generating timeline thumbnails"
	libID, _ := uuid.Parse(p.LibraryID)

	items, err := h.mediaRepo.ListItemsNeedingSprites(libID)
	if err != nil {
		return fmt.Errorf("list items needing sprites: %w", err)
	}

	if len(items) == 0 {
		log.Printf("Sprites: no items need timeline thumbnails in library %s", p.LibraryID)
		return nil
	}

	log.Printf("Sprites: generating timeline thumbnails for %d items in library %s (2 workers, keyframe-only)", len(items), p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskSpritesLibrary,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	const spriteWorkers = 2
	var completed int64
	total := int64(len(items))

	work := make(chan *models.MediaItem, spriteWorkers)
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
				done := atomic.LoadInt64(&completed)
				pct := int(float64(done) / float64(total) * 100)
				desc := fmt.Sprintf("Timeline thumbnails · %d/%d", done, total)
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskSpritesLibrary,
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

	for w := 0; w < spriteWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				h.sc.GenerateTimelineThumbnails(item)
				atomic.AddInt64(&completed, 1)
			}
		}()
	}

	for _, item := range items {
		select {
		case <-ctx.Done():
			log.Printf("Sprites: cancelled after %d/%d items", atomic.LoadInt64(&completed), total)
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

	log.Printf("Sprites: completed %d timeline thumbnails for library %s", len(items), p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskSpritesLibrary,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}

// ──────── Previews Library Handler ────────

type PreviewsLibraryHandler struct {
	mediaRepo    *repository.MediaRepository
	libRepo      *repository.LibraryRepository
	settingsRepo *repository.SettingsRepository
	sc           *scanner.Scanner
	notifier     EventNotifier
}

func NewPreviewsLibraryHandler(mediaRepo *repository.MediaRepository, libRepo *repository.LibraryRepository, settingsRepo *repository.SettingsRepository, sc *scanner.Scanner, notifier EventNotifier) *PreviewsLibraryHandler {
	return &PreviewsLibraryHandler{mediaRepo: mediaRepo, libRepo: libRepo, settingsRepo: settingsRepo, sc: sc, notifier: notifier}
}

func (h *PreviewsLibraryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p PreviewsLibraryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	// For non-rebuild jobs, respect the global + per-library settings; rebuild always runs
	if !p.Rebuild {
		if h.settingsRepo != nil {
			if val, err := h.settingsRepo.Get("create_previews_enabled"); err == nil && val == "false" {
				log.Printf("Previews: skipping for library %s (Create Previews globally disabled)", p.LibraryID)
				return nil
			}
		}
		if h.libRepo != nil {
			libID, _ := uuid.Parse(p.LibraryID)
			if lib, err := h.libRepo.GetByID(libID); err == nil && !lib.CreatePreviews {
				log.Printf("Previews: skipping for library %s (Create Previews disabled for this library)", p.LibraryID)
				return nil
			}
		}
	}

	taskID := "previews:" + p.LibraryID
	taskDesc := "Generating preview clips"
	if p.Rebuild {
		taskDesc = "Rebuilding preview clips"
	}
	libID, _ := uuid.Parse(p.LibraryID)

	// For rebuild: clear existing previews and delete files, then regenerate all
	if p.Rebuild {
		log.Printf("Previews: rebuild requested — clearing existing previews for library %s", p.LibraryID)
		oldPaths, err := h.mediaRepo.ClearLibraryPreviews(libID)
		if err != nil {
			log.Printf("Previews: failed to clear DB preview paths: %v", err)
		}
		previewBaseDir := h.sc.PreviewDir()
		for _, webPath := range oldPaths {
			// webPath is like /previews/previews/{id}.mp4 or .webp
			relPath := strings.TrimPrefix(webPath, "/previews/")
			diskPath := filepath.Join(previewBaseDir, relPath)
			if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
				log.Printf("Previews: failed to remove %s: %v", diskPath, err)
			}
		}
	}

	var items []*models.MediaItem
	var err error
	if p.Rebuild {
		items, err = h.mediaRepo.ListAllItemsForPreviews(libID)
	} else {
		items, err = h.mediaRepo.ListItemsNeedingPreviews(libID)
	}
	if err != nil {
		return fmt.Errorf("list items for previews: %w", err)
	}

	if len(items) == 0 {
		log.Printf("Previews: no items need preview clips in library %s", p.LibraryID)
		return nil
	}

	log.Printf("Previews: generating preview clips for %d items in library %s", len(items), p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskPreviewsLibrary,
			"status": "running", "progress": 0, "description": taskDesc,
		})
	}

	const previewWorkers = 2
	var completed int64
	total := int64(len(items))

	work := make(chan *models.MediaItem, previewWorkers)
	var wg sync.WaitGroup

	// Progress reporter: runs in a separate goroutine so it doesn't
	// contend with workers. Reports at most every 500ms.
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
				done := atomic.LoadInt64(&completed)
				pct := int(float64(done) / float64(total) * 100)
				desc := fmt.Sprintf("Preview clips · %d/%d", done, total)
				h.notifier.Broadcast("task:update", map[string]interface{}{
					"task_id": taskID, "task_type": TaskPreviewsLibrary,
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

	// Start workers
	for w := 0; w < previewWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				h.sc.GeneratePreviewClip(item)
				atomic.AddInt64(&completed, 1)
			}
		}()
	}

	// Feed items to workers
	for _, item := range items {
		select {
		case <-ctx.Done():
			log.Printf("Previews: cancelled after %d/%d items", atomic.LoadInt64(&completed), total)
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

	log.Printf("Previews: completed %d preview clips for library %s", len(items), p.LibraryID)
	if h.notifier != nil {
		h.notifier.Broadcast("task:update", map[string]interface{}{
			"task_id": taskID, "task_type": TaskPreviewsLibrary,
			"status": "complete", "progress": 100, "description": taskDesc,
		})
	}
	return nil
}
