package scanner

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JustinTDCT/CineVault/internal/config"
	"github.com/JustinTDCT/CineVault/internal/media"
)

type Scanner struct {
	db        *sql.DB
	cfg       *config.Config
	mediaRepo *media.Repository
}

func New(db *sql.DB, cfg *config.Config, mediaRepo *media.Repository) *Scanner {
	return &Scanner{db: db, cfg: cfg, mediaRepo: mediaRepo}
}

func (s *Scanner) ScanLibrary(libraryID string, folders []string) ScanResult {
	result := ScanResult{LibraryID: libraryID, StartedAt: time.Now()}

	s.db.Exec(`
		INSERT INTO scan_state (library_id, last_scan_started, status)
		VALUES ($1, NOW(), 'scanning')
		ON CONFLICT (library_id) DO UPDATE SET last_scan_started=NOW(), status='scanning'`,
		libraryID)

	for _, folder := range folders {
		s.scanFolder(folder, libraryID, &result)
	}

	result.CompletedAt = time.Now()
	s.db.Exec(`
		UPDATE scan_state SET last_scan_completed=NOW(), files_scanned=$2, files_added=$3,
		       files_removed=$4, status='idle'
		WHERE library_id=$1`,
		libraryID, result.FilesScanned, result.FilesAdded, result.FilesRemoved)

	return result
}

func (s *Scanner) scanFolder(root, libraryID string, result *ScanResult) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if isHiddenDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if !IsMediaFile(path) {
			return nil
		}

		result.FilesScanned++

		existing, err := s.mediaRepo.GetByFilePath(path)
		if err == nil && existing != nil {
			if existing.FileModTime != nil && info.ModTime().Equal(*existing.FileModTime) {
				return nil
			}
		}

		parsed := ParseFilename(path)
		modTime := info.ModTime()
		fileSize := info.Size()

		item := &media.MediaItem{
			LibraryID:   libraryID,
			FilePath:    path,
			FileSize:    &fileSize,
			FileModTime: &modTime,
			Metadata:    json.RawMessage("{}"),
		}

		if parsed.Title != "" {
			item.Title = &parsed.Title
			sortTitle := strings.ToLower(parsed.Title)
			item.SortTitle = &sortTitle
		}
		if parsed.Year > 0 {
			item.ReleaseYear = &parsed.Year
		}
		if parsed.Season > 0 {
			item.SeasonNumber = &parsed.Season
		}
		if parsed.Episode > 0 {
			item.EpisodeNumber = &parsed.Episode
		}

		probe, err := Probe(s.cfg.FFprobePath, path)
		if err == nil && probe != nil {
			if probe.VideoCodec != "" {
				item.VideoCodec = &probe.VideoCodec
			}
			if probe.AudioCodec != "" {
				item.AudioCodec = &probe.AudioCodec
			}
			if probe.Resolution != "" {
				item.Resolution = &probe.Resolution
			}
			if probe.Bitrate > 0 {
				item.Bitrate = &probe.Bitrate
			}
			if probe.Duration > 0 {
				mins := int(probe.Duration / 60)
				item.RuntimeMinutes = &mins
			}
		}

		if existing != nil {
			s.mediaRepo.UpdateTechnical(existing.ID, item.VideoCodec, item.AudioCodec,
				item.Resolution, item.Bitrate, item.FileSize)
			return nil
		}

		if err := s.mediaRepo.Create(item); err != nil {
			log.Printf("scanner: failed to create item for %s: %v", path, err)
			return nil
		}
		result.FilesAdded++
		return nil
	})
}

func isHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@")
}

type ScanResult struct {
	LibraryID    string    `json:"library_id"`
	FilesScanned int       `json:"files_scanned"`
	FilesAdded   int       `json:"files_added"`
	FilesRemoved int       `json:"files_removed"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}
