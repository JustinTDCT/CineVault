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
