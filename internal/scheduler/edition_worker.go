package scheduler

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/JustinTDCT/CineVault/internal/metadata"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
)

// EditionWorker periodically re-queries the cache server for media items
// that are awaiting edition discovery (editions_pending = true).
type EditionWorker struct {
	mediaRepo    *repository.MediaRepository
	settingsRepo *repository.SettingsRepository
	interval     time.Duration
	batchSize    int
	stop         chan struct{}
}

func NewEditionWorker(mediaRepo *repository.MediaRepository, settingsRepo *repository.SettingsRepository) *EditionWorker {
	return &EditionWorker{
		mediaRepo:    mediaRepo,
		settingsRepo: settingsRepo,
		interval:     5 * time.Minute,
		batchSize:    50,
		stop:         make(chan struct{}),
	}
}

func (w *EditionWorker) Start() {
	go w.run()
	log.Printf("[edition-worker] started (interval=%s, batch=%d)", w.interval, w.batchSize)
}

func (w *EditionWorker) Stop() {
	close(w.stop)
}

func (w *EditionWorker) run() {
	time.Sleep(30 * time.Second)
	w.check()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.check()
		case <-w.stop:
			log.Println("[edition-worker] stopped")
			return
		}
	}
}

func (w *EditionWorker) check() {
	cacheEnabled, _ := w.settingsRepo.Get("cache_server_enabled")
	if cacheEnabled == "false" {
		return
	}

	cacheClient := metadata.EnsureRegistered(w.settingsRepo)
	if cacheClient == nil {
		return
	}

	items, err := w.mediaRepo.GetEditionsPendingItems(w.batchSize)
	if err != nil {
		log.Printf("[edition-worker] query error: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}

	log.Printf("[edition-worker] processing %d pending items", len(items))
	resolved := 0

	for _, item := range items {
		tmdbID := extractTMDBID(item.ExternalIDs)
		if tmdbID == 0 {
			_ = w.mediaRepo.SetEditionsPending(item.ID, false)
			resolved++
			continue
		}

		mt := models.MediaType(item.MediaType)
		result := cacheClient.LookupByTMDB(tmdbID, mt)
		if result == nil {
			continue
		}

		if !result.EditionsDiscovered {
			continue
		}

		_ = w.mediaRepo.SetEditionsPending(item.ID, false)
		resolved++

		if len(result.AvailableEditions) > 0 {
			log.Printf("[edition-worker] %q: %d edition(s) available", item.Title, len(result.AvailableEditions))
		}
	}

	if resolved > 0 {
		log.Printf("[edition-worker] resolved %d/%d pending items", resolved, len(items))
	}
}

func extractTMDBID(externalIDs *string) int {
	if externalIDs == nil || *externalIDs == "" {
		return 0
	}
	var ids map[string]interface{}
	if err := json.Unmarshal([]byte(*externalIDs), &ids); err != nil {
		return 0
	}
	switch v := ids["tmdb_id"].(type) {
	case string:
		id, _ := strconv.Atoi(v)
		return id
	case float64:
		return int(v)
	}
	return 0
}
