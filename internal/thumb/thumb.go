// Package thumb makes still images from video files with FFmpeg, for titles
// that have no artwork from a metadata source (music videos, unmatched movies).
package thumb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Maker creates and caches thumbnails. The zero value isn't usable; set
// FFmpeg and Dir.
type Maker struct {
	FFmpeg string // path to ffmpeg
	Dir    string // cache folder; created when needed
	sem    chan struct{}
}

// New returns a Maker that runs at most two FFmpeg processes at a time.
func New(ffmpeg, dir string) *Maker {
	return &Maker{FFmpeg: ffmpeg, Dir: dir, sem: make(chan struct{}, 2)}
}

// Source is the video a thumbnail is taken from.
type Source struct {
	VersionID       int64
	Path            string
	SizeBytes       int64
	DurationSeconds float64
}

// ErrFailed means FFmpeg couldn't produce an image from the file.
var ErrFailed = errors.New("could not make a thumbnail from this file")

// Get returns the path of a 640-pixel-wide JPEG taken a third of the way
// into the video, making it on first use. The cache name includes the file
// size, so a replaced file gets a new image.
func (m *Maker) Get(ctx context.Context, src Source) (string, error) {
	out := filepath.Join(m.Dir, fmt.Sprintf("v%d-%d.jpg", src.VersionID, src.SizeBytes))
	if st, err := os.Stat(out); err == nil && st.Size() > 0 {
		return out, nil
	}
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if st, err := os.Stat(out); err == nil && st.Size() > 0 { // made while we waited
		return out, nil
	}
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return "", err
	}
	at := src.DurationSeconds / 3
	if at <= 0 {
		at = 5
	}
	tmp := out + ".tmp.jpg"
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.FFmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-ss", strconv.FormatFloat(at, 'f', 2, 64), "-i", src.Path,
		"-frames:v", "1", "-vf", "scale=640:-2", "-q:v", "4", tmp)
	if msg, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("%w: %v %s", ErrFailed, err, msg)
	}
	if st, err := os.Stat(tmp); err != nil || st.Size() == 0 {
		os.Remove(tmp)
		return "", ErrFailed
	}
	return out, os.Rename(tmp, out)
}

// coverNames are the image files that hold an album's cover, in order of preference.
var coverNames = []string{"cover", "folder", "front", "album", "albumart"}

// Cover returns an album cover for a music file: an image next to it
// (cover.jpg, folder.jpg, front.png...) or, failing that, the art embedded in
// the file, extracted once and cached.
func (m *Maker) Cover(ctx context.Context, src Source) (string, error) {
	if p := FolderCover(filepath.Dir(src.Path)); p != "" {
		return p, nil
	}
	out := filepath.Join(m.Dir, fmt.Sprintf("c%d-%d.jpg", src.VersionID, src.SizeBytes))
	if st, err := os.Stat(out); err == nil && st.Size() > 0 {
		return out, nil
	}
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return "", err
	}
	tmp := out + ".tmp.jpg"
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.FFmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", src.Path, "-an", "-map", "0:v:0", "-frames:v", "1",
		"-vf", "scale='min(600,iw)':-2", "-q:v", "3", tmp)
	if msg, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("%w: %v %s", ErrFailed, err, msg)
	}
	if st, err := os.Stat(tmp); err != nil || st.Size() == 0 {
		os.Remove(tmp)
		return "", ErrFailed
	}
	return out, os.Rename(tmp, out)
}

// FolderCover finds a cover image in dir, matching names without regard to case.
func FolderCover(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	found := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		ext := filepath.Ext(name)
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		if _, ok := found[base]; !ok {
			found[base] = filepath.Join(dir, e.Name())
		}
	}
	for _, n := range coverNames {
		if p, ok := found[n]; ok {
			return p
		}
	}
	return ""
}
