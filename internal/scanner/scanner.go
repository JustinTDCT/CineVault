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
	"github.com/JustinTDCT/CineVault/internal/libraries"
	"github.com/JustinTDCT/CineVault/internal/media"
)

const progressInterval = 50

type MetadataMatcher interface {
	AutoMatch(item *media.MediaItem, libType libraries.LibraryType) error
}

type Scanner struct {
	db        *sql.DB
	cfg       *config.Config
	mediaRepo *media.Repository
	matcher   MetadataMatcher
}

func New(db *sql.DB, cfg *config.Config, mediaRepo *media.Repository, matcher MetadataMatcher) *Scanner {
	return &Scanner{db: db, cfg: cfg, mediaRepo: mediaRepo, matcher: matcher}
}

func (s *Scanner) ResetStuckScans() {
	res, err := s.db.Exec(`UPDATE scan_state SET status='idle' WHERE status='scanning'`)
	if err != nil {
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("[scanner] reset %d stuck scan(s) to idle", n)
	}
}

func (s *Scanner) ScanLibrary(libraryID string, libType libraries.LibraryType, folders []string) ScanResult {
	result := ScanResult{LibraryID: libraryID, StartedAt: time.Now()}
	log.Printf("[scanner] starting scan for library %s (%s), %d folder(s)", libraryID, libType, len(folders))

	s.db.Exec(`
		INSERT INTO scan_state (library_id, last_scan_started, status)
		VALUES ($1, NOW(), 'scanning')
		ON CONFLICT (library_id) DO UPDATE SET last_scan_started=NOW(), status='scanning',
		       files_scanned=0, files_added=0, files_removed=0`,
		libraryID)

	for _, folder := range folders {
		s.scanFolder(folder, libraryID, libType, &result)
	}

	if s.matcher != nil {
		s.matchUnmatched(libraryID, libType, &result)
	}

	result.CompletedAt = time.Now()
	elapsed := result.CompletedAt.Sub(result.StartedAt).Round(time.Millisecond)
	log.Printf("[scanner] scan complete for library %s: %d scanned, %d added, %d removed, %d matched (%s)",
		libraryID, result.FilesScanned, result.FilesAdded, result.FilesRemoved, result.FilesMatched, elapsed)

	s.db.Exec(`
		UPDATE scan_state SET last_scan_completed=NOW(), files_scanned=$2, files_added=$3,
		       files_removed=$4, status='idle'
		WHERE library_id=$1`,
		libraryID, result.FilesScanned, result.FilesAdded, result.FilesRemoved)

	return result
}

func (s *Scanner) flushProgress(libraryID string, result *ScanResult) {
	s.db.Exec(`UPDATE scan_state SET files_scanned=$2, files_added=$3 WHERE library_id=$1`,
		libraryID, result.FilesScanned, result.FilesAdded)
}

func (s *Scanner) scanFolder(root, libraryID string, libType libraries.LibraryType, result *ScanResult) {
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

		if result.FilesScanned%progressInterval == 0 {
			log.Printf("[scanner] progress: %d scanned, %d added so far", result.FilesScanned, result.FilesAdded)
			s.flushProgress(libraryID, result)
		}

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
			log.Printf("[scanner] failed to create item for %s: %v", path, err)
			return nil
		}
		result.FilesAdded++

		if s.matcher != nil {
			if err := s.matcher.AutoMatch(item, libType); err != nil {
				log.Printf("[scanner] automatch failed for %q: %v", path, err)
			} else {
				result.FilesMatched++
			}
		}

		return nil
	})
}

func (s *Scanner) matchUnmatched(libraryID string, libType libraries.LibraryType, result *ScanResult) {
	rows, err := s.db.Query(`
		SELECT id, library_id, title, release_year, season_number, episode_number, file_path
		FROM media_items WHERE library_id=$1 AND cache_id IS NULL AND title IS NOT NULL`,
		libraryID)
	if err != nil {
		log.Printf("[scanner] failed to query unmatched items: %v", err)
		return
	}
	defer rows.Close()

	var unmatched []media.MediaItem
	for rows.Next() {
		var m media.MediaItem
		if err := rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.ReleaseYear,
			&m.SeasonNumber, &m.EpisodeNumber, &m.FilePath); err != nil {
			continue
		}
		unmatched = append(unmatched, m)
	}

	if len(unmatched) == 0 {
		return
	}

	log.Printf("[scanner] matching %d unmatched items in library %s", len(unmatched), libraryID)
	matched := 0
	for i := range unmatched {
		if err := s.matcher.AutoMatch(&unmatched[i], libType); err != nil {
			log.Printf("[scanner] automatch retry failed for %q: %v", unmatched[i].FilePath, err)
		} else {
			matched++
		}
		if (i+1)%progressInterval == 0 {
			log.Printf("[scanner] match progress: %d/%d processed, %d matched", i+1, len(unmatched), matched)
		}
	}
	result.FilesMatched += matched
	log.Printf("[scanner] unmatched sweep done: %d/%d matched", matched, len(unmatched))
}

func isHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@")
}

type ScanResult struct {
	LibraryID    string    `json:"library_id"`
	FilesScanned int       `json:"files_scanned"`
	FilesAdded   int       `json:"files_added"`
	FilesRemoved int       `json:"files_removed"`
	FilesMatched int       `json:"files_matched"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}
