package thumb

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGet(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mp4")
	if out, err := exec.Command(ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=10:duration=3",
		"-pix_fmt", "yuv420p", src).CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v %s", err, out)
	}
	m := New(ff, filepath.Join(dir, "cache"))
	p, err := m.Get(context.Background(), Source{VersionID: 7, Path: src, SizeBytes: 123, DurationSeconds: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if len(b) < 100 || b[0] != 0xFF || b[1] != 0xD8 {
		t.Fatalf("not a JPEG: %d bytes", len(b))
	}
	// Cached: the source can go and the image is still returned.
	os.Remove(src)
	if p2, err := m.Get(context.Background(), Source{VersionID: 7, Path: src, SizeBytes: 123}); err != nil || p2 != p {
		t.Errorf("cached: %s %v", p2, err)
	}
	if _, err := m.Get(context.Background(), Source{VersionID: 8, Path: src, SizeBytes: 1}); !errors.Is(err, ErrFailed) {
		t.Errorf("missing source: %v", err)
	}
}

func TestCover(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	art := filepath.Join(dir, "art.png")
	song := filepath.Join(dir, "01 Song.flac")
	run := func(args ...string) {
		if out, err := exec.Command(ff, append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("-f", "lavfi", "-i", "color=c=red:s=300x300", "-frames:v", "1", art)
	run("-f", "lavfi", "-i", "sine=d=1", "-i", art, "-map", "0:a", "-map", "1:v", "-c:v", "png", "-disposition:v", "attached_pic", song)
	os.Remove(art)

	m := New(ff, filepath.Join(dir, "cache"))
	p, err := m.Cover(context.Background(), Source{VersionID: 3, Path: song, SizeBytes: 10})
	if err != nil {
		t.Fatalf("embedded: %v", err)
	}
	if b, _ := os.ReadFile(p); len(b) < 100 || b[0] != 0xFF {
		t.Error("embedded cover isn't a JPEG")
	}
	// A cover file next to the songs wins over embedded art.
	folder := filepath.Join(dir, "Folder.JPG")
	os.WriteFile(folder, []byte("jpeg"), 0o644)
	if p, err := m.Cover(context.Background(), Source{VersionID: 3, Path: song, SizeBytes: 10}); err != nil || p != folder {
		t.Errorf("folder cover: %s %v", p, err)
	}
}
