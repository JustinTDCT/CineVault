// Package scan walks library folders and records titles, editions and versions.
package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/mediainfo"
)

// KnownVersion is a file already recorded for a library.
type KnownVersion struct {
	ID   int64
	Size int64
}

// NewVersion is everything recorded about one scanned file.
type NewVersion struct {
	Path        string
	SizeBytes   int64
	Fingerprint string
	Info        mediainfo.Info
	// EpisodeID links a TV file to its episode; 0 for movies. LastEpisode is
	// the last episode number of a multi-episode file, 0 otherwise.
	EpisodeID   int64
	LastEpisode int
}

// Store is what the scanner needs from the database.
type Store interface {
	EditionTypes(ctx context.Context) ([]catalog.EditionType, error)
	// KnownVersions maps path to the files already recorded for the library.
	KnownVersions(ctx context.Context, libraryID int64) (map[string]KnownVersion, error)
	// FindOrCreateTitle matches on library, name (case-insensitive) and year.
	FindOrCreateTitle(ctx context.Context, libraryID int64, mediaType string, p ParsedName) (int64, error)
	// FindOrCreateEdition sets runtimeSeconds only if the edition has none yet.
	FindOrCreateEdition(ctx context.Context, titleID int64, editionTypeID string, runtimeSeconds int) (int64, error)
	// FindOrCreateEpisode finds a show's episode, creating it (and its season)
	// with the name from the file if it's new.
	FindOrCreateEpisode(ctx context.Context, titleID int64, season, episode int, name string) (int64, error)
	// UpsertVersion inserts or updates the file by path and clears its missing
	// flag. It never moves a version whose edition the user locked.
	UpsertVersion(ctx context.Context, editionID int64, v NewVersion) error
	// SetMissing marks files as missing (offline) or present again.
	SetMissing(ctx context.Context, ids []int64, missing bool) error
}

// FileError is a file the scan skipped, with the reason.
type FileError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Result summarizes a scan.
type Result struct {
	FilesSeen int         `json:"files_seen"`
	Added     int         `json:"added"`
	Updated   int         `json:"updated"`
	Unchanged int         `json:"unchanged"`
	Missing   int         `json:"missing"`
	Errors    []FileError `json:"errors"`
}

// Scanner scans libraries.
type Scanner struct {
	Store  Store
	Prober mediainfo.Prober
}

// VideoExtensions are the file types scanned in movie and TV libraries.
var VideoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true,
	".ts": true, ".m2ts": true, ".webm": true, ".wmv": true, ".mpg": true, ".mpeg": true,
}

// Scan walks every folder of the library and brings the database in line
// with what's on disk. Unchanged files (same path and size) aren't probed again.
func (s Scanner) Scan(ctx context.Context, lib catalog.Library) (Result, error) {
	res := Result{Errors: []FileError{}}
	if lib.MediaType != "movie" && lib.MediaType != "tv" && lib.MediaType != "musicvideo" {
		return res, fmt.Errorf("scanning %s libraries isn't supported yet", lib.MediaType)
	}

	allTypes, err := s.Store.EditionTypes(ctx)
	if err != nil {
		return res, fmt.Errorf("load Edition List: %w", err)
	}
	var types []catalog.EditionType
	defaultEdition := ""
	for _, e := range allTypes {
		if e.AppliesTo(lib.MediaType) {
			types = append(types, e)
			if e.IsDefault {
				defaultEdition = e.ID
			}
		}
	}
	if defaultEdition == "" {
		return res, fmt.Errorf("the Edition List has no default edition for %s", lib.MediaType)
	}

	known, err := s.Store.KnownVersions(ctx, lib.ID)
	if err != nil {
		return res, fmt.Errorf("load known files: %w", err)
	}

	seen := map[string]bool{}
	var unchanged []int64
	for _, root := range lib.Paths {
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil {
				res.Errors = append(res.Errors, FileError{Path: path, Error: describe(err)})
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if path != root && strings.HasPrefix(name, ".") {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || strings.HasPrefix(name, ".") || !VideoExtensions[strings.ToLower(filepath.Ext(name))] || isSample(name) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				res.Errors = append(res.Errors, FileError{Path: path, Error: describe(err)})
				return nil
			}

			seen[path] = true
			res.FilesSeen++
			k, existed := known[path]
			if existed && k.Size == info.Size() {
				unchanged = append(unchanged, k.ID)
				res.Unchanged++
				return nil
			}
			add := s.addFile
			switch lib.MediaType {
			case "tv":
				add = s.addEpisodeFile
			case "musicvideo":
				add = s.addMusicVideoFile
			}
			if err := add(ctx, lib, root, types, defaultEdition, path, info.Size()); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				res.Errors = append(res.Errors, FileError{Path: path, Error: err.Error()})
				return nil
			}
			if existed {
				res.Updated++
			} else {
				res.Added++
			}
			return nil
		})
		if walkErr != nil {
			return res, walkErr
		}
	}

	var missing []int64
	for path, k := range known {
		if !seen[path] {
			missing = append(missing, k.ID)
		}
	}
	res.Missing = len(missing)
	if err := s.Store.SetMissing(ctx, missing, true); err != nil {
		return res, fmt.Errorf("mark missing files: %w", err)
	}
	if err := s.Store.SetMissing(ctx, unchanged, false); err != nil {
		return res, fmt.Errorf("mark files present: %w", err)
	}
	return res, nil
}

func (s Scanner) addFile(ctx context.Context, lib catalog.Library, _ string, types []catalog.EditionType, defaultEdition, path string, size int64) error {
	info, err := s.Prober.Probe(ctx, path)
	if err != nil {
		return err
	}
	fp, err := Fingerprint(path)
	if err != nil {
		return fmt.Errorf("fingerprint: %w", describeErr(err))
	}
	p := ParseName(path, types, defaultEdition)
	if p.Title == "" {
		return errors.New("could not work out a title from the file name")
	}
	titleID, err := s.Store.FindOrCreateTitle(ctx, lib.ID, lib.MediaType, p)
	if err != nil {
		return fmt.Errorf("save title: %w", err)
	}
	editionID, err := s.Store.FindOrCreateEdition(ctx, titleID, p.EditionTypeID, int(info.DurationSeconds+0.5))
	if err != nil {
		return fmt.Errorf("save edition: %w", err)
	}
	if err := s.Store.UpsertVersion(ctx, editionID, NewVersion{Path: path, SizeBytes: size, Fingerprint: fp, Info: info}); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

// addMusicVideoFile records a music video: one title per artist and song,
// with live, acoustic and other versions as its editions.
func (s Scanner) addMusicVideoFile(ctx context.Context, lib catalog.Library, root string, types []catalog.EditionType, defaultEdition, path string, size int64) error {
	p := ParseMusicVideo(root, path, types, defaultEdition)
	if p.Title == "" {
		return errors.New("could not work out the song from the file name; name it like \"Artist - Song.mp4\"")
	}
	info, err := s.Prober.Probe(ctx, path)
	if err != nil {
		return err
	}
	fp, err := Fingerprint(path)
	if err != nil {
		return fmt.Errorf("fingerprint: %w", describeErr(err))
	}
	titleID, err := s.Store.FindOrCreateTitle(ctx, lib.ID, lib.MediaType, p)
	if err != nil {
		return fmt.Errorf("save title: %w", err)
	}
	editionID, err := s.Store.FindOrCreateEdition(ctx, titleID, p.EditionTypeID, int(info.DurationSeconds+0.5))
	if err != nil {
		return fmt.Errorf("save edition: %w", err)
	}
	if err := s.Store.UpsertVersion(ctx, editionID, NewVersion{Path: path, SizeBytes: size, Fingerprint: fp, Info: info}); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

// addEpisodeFile records a TV episode file. All of a show's files go in its
// default edition for now; per-episode editions come later.
func (s Scanner) addEpisodeFile(ctx context.Context, lib catalog.Library, root string, _ []catalog.EditionType, defaultEdition, path string, size int64) error {
	p, ok := ParseEpisode(root, path)
	if !ok {
		return errors.New("no season and episode number in the file name; name it like \"Show - S01E02 - Title.mkv\"")
	}
	info, err := s.Prober.Probe(ctx, path)
	if err != nil {
		return err
	}
	fp, err := Fingerprint(path)
	if err != nil {
		return fmt.Errorf("fingerprint: %w", describeErr(err))
	}
	titleID, err := s.Store.FindOrCreateTitle(ctx, lib.ID, lib.MediaType, p.Show)
	if err != nil {
		return fmt.Errorf("save show: %w", err)
	}
	editionID, err := s.Store.FindOrCreateEdition(ctx, titleID, defaultEdition, 0)
	if err != nil {
		return fmt.Errorf("save edition: %w", err)
	}
	episodeID, err := s.Store.FindOrCreateEpisode(ctx, titleID, p.Season, p.Episode, p.Name)
	if err != nil {
		return fmt.Errorf("save episode: %w", err)
	}
	v := NewVersion{Path: path, SizeBytes: size, Fingerprint: fp, Info: info, EpisodeID: episodeID, LastEpisode: p.Last}
	if err := s.Store.UpsertVersion(ctx, editionID, v); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

func isSample(name string) bool {
	n := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	return n == "sample" || strings.HasSuffix(n, "-sample") || strings.HasSuffix(n, ".sample")
}

func describe(err error) string { return describeErr(err).Error() }

func describeErr(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return errors.New("folder or file not found")
	case errors.Is(err, fs.ErrPermission):
		return errors.New("permission denied")
	}
	return err
}

// fingerprintChunk is how much of each end of a file is hashed.
const fingerprintChunk = 64 << 10

// Fingerprint identifies a file by its size and a hash of its first and last
// 64 KiB, so a file is recognized after a rename or a move to another drive
// without reading the whole thing.
func Fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := st.Size()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, fingerprintChunk)); err != nil {
		return "", err
	}
	if size > fingerprintChunk {
		off := size - fingerprintChunk
		if off < fingerprintChunk {
			off = fingerprintChunk
		}
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return "", err
		}
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("v1:%d:%s", size, hex.EncodeToString(h.Sum(nil))[:32]), nil
}
