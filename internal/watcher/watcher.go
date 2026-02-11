package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

// OnFileEvent is called when a file is created or removed.
type OnFileEvent func(libraryID uuid.UUID, path string, isCreate bool)

// Watcher monitors library folders for filesystem changes.
type Watcher struct {
	libRepo  *repository.LibraryRepository
	callback OnFileEvent
	watcher  *fsnotify.Watcher
	mu       sync.Mutex
	watched  map[string]uuid.UUID // folder path → library ID
	debounce map[string]*time.Timer
	stop     chan struct{}
}

// New creates a filesystem watcher.
func New(libRepo *repository.LibraryRepository, cb OnFileEvent) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		libRepo:  libRepo,
		callback: cb,
		watcher:  fw,
		watched:  make(map[string]uuid.UUID),
		debounce: make(map[string]*time.Timer),
		stop:     make(chan struct{}),
	}, nil
}

// Start begins watching all enabled libraries and processes events.
func (w *Watcher) Start() {
	go w.eventLoop()
	w.Refresh()
	log.Println("[watcher] filesystem watcher started")
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.stop)
	w.watcher.Close()
}

// Refresh reloads watched library folders.
func (w *Watcher) Refresh() {
	libs, err := w.libRepo.GetWatchEnabled()
	if err != nil {
		log.Printf("[watcher] error loading watch-enabled libraries: %v", err)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Collect desired paths
	desired := make(map[string]uuid.UUID)
	for _, lib := range libs {
		for _, f := range lib.Folders {
			desired[f.FolderPath] = lib.ID
		}
		if len(lib.Folders) == 0 {
			desired[lib.Path] = lib.ID
		}
	}

	// Remove paths no longer desired
	for p := range w.watched {
		if _, ok := desired[p]; !ok {
			w.watcher.Remove(p)
			delete(w.watched, p)
		}
	}

	// Add new paths
	for p, libID := range desired {
		if _, ok := w.watched[p]; ok {
			continue
		}
		if err := w.addRecursive(p, libID); err != nil {
			log.Printf("[watcher] error adding %s: %v", p, err)
		}
	}

	log.Printf("[watcher] watching %d paths across %d libraries", len(w.watched), len(libs))
}

func (w *Watcher) addRecursive(root string, libID uuid.UUID) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible dirs
		}
		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				return nil
			}
			w.watched[path] = libID
		}
		return nil
	})
}

func (w *Watcher) eventLoop() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[watcher] error: %v", err)
		case <-w.stop:
			return
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Skip hidden files and temp files
	base := filepath.Base(event.Name)
	if strings.HasPrefix(base, ".") || strings.HasSuffix(base, ".tmp") ||
		strings.HasSuffix(base, ".part") {
		return
	}

	isCreate := event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
	isRemove := event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename)

	if !isCreate && !isRemove {
		return
	}

	// For created dirs, add them to the watch list
	if isCreate {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			libID := w.resolveLibrary(event.Name)
			if libID != uuid.Nil {
				w.mu.Lock()
				w.watcher.Add(event.Name)
				w.watched[event.Name] = libID
				w.mu.Unlock()
			}
			return
		}
	}

	// Only process media files
	ext := strings.ToLower(filepath.Ext(event.Name))
	if !isMediaExtension(ext) {
		return
	}

	libID := w.resolveLibrary(event.Name)
	if libID == uuid.Nil {
		return
	}

	// Debounce: 1 second
	w.mu.Lock()
	if timer, ok := w.debounce[event.Name]; ok {
		timer.Stop()
	}
	eventName := event.Name
	w.debounce[eventName] = time.AfterFunc(1*time.Second, func() {
		w.mu.Lock()
		delete(w.debounce, eventName)
		w.mu.Unlock()

		if isCreate {
			w.callback(libID, eventName, true)
		}
		if isRemove && !isCreate {
			w.callback(libID, eventName, false)
		}
	})
	w.mu.Unlock()
}

func (w *Watcher) resolveLibrary(path string) uuid.UUID {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Walk up directory tree to find watched parent
	dir := filepath.Dir(path)
	for dir != "/" && dir != "." {
		if libID, ok := w.watched[dir]; ok {
			return libID
		}
		dir = filepath.Dir(dir)
	}
	return uuid.Nil
}

func isMediaExtension(ext string) bool {
	media := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".m4v": true, ".wmv": true, ".flv": true, ".webm": true,
		".ts": true, ".m2ts": true, ".mpg": true, ".mpeg": true,
		".mp3": true, ".flac": true, ".aac": true, ".ogg": true,
		".wav": true, ".m4a": true, ".m4b": true, ".opus": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true,
	}
	return media[ext]
}

// ExtraType classifies a filename as an extra type based on folder names and patterns.
func ClassifyExtra(filePath string) (models.MediaType, string) {
	lower := strings.ToLower(filePath)

	patterns := map[string]string{
		"trailer":            "trailer",
		"featurette":         "featurette",
		"behind the scenes":  "behind-the-scenes",
		"behind_the_scenes":  "behind-the-scenes",
		"behindthescenes":    "behind-the-scenes",
		"deleted scene":      "deleted-scene",
		"deleted_scene":      "deleted-scene",
		"deletedscene":       "deleted-scene",
		"interview":          "interview",
		"short":              "short",
		"sample":             "sample",
	}

	for pattern, extraType := range patterns {
		if strings.Contains(lower, pattern) {
			return "", extraType
		}
	}

	return "", ""
}
