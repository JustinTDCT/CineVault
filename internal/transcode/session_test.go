package transcode

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JustinTDCT/CineVault/internal/probe"
)

// testSource makes a 60-second HEVC file (or H.264 if FFmpeg lacks x265).
// Tests using it are skipped when FFmpeg or libx264 isn't installed.
func testSource(t *testing.T) Source {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	out, _ := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	if !strings.Contains(string(out), "libx264") {
		t.Skip("ffmpeg has no libx264")
	}
	codec, enc := "hevc", []string{"-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=error"}
	if !strings.Contains(string(out), "libx265") {
		codec, enc = "h264", []string{"-c:v", "libx264", "-preset", "ultrafast"}
	}
	path := filepath.Join(t.TempDir(), "Alien (1979).mkv")
	args := append([]string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=320x136:r=24:d=60",
		"-f", "lavfi", "-i", "sine=d=60"}, enc...)
	args = append(args, "-c:a", "aac", path)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("make test file: %v %s", err, out)
	}
	return Source{Path: path, VideoCodec: codec, Height: 136, DurationSeconds: 60}
}

var cpuPlan = Plan{Mode: "hls", Encoder: probe.Encoder{Name: "libx264", Codec: "h264", Kind: probe.KindCPU, GPU: -1}, MaxHeight: 1080}

func newManager(t *testing.T) *Manager {
	return &Manager{FFmpeg: "ffmpeg", WorkDir: t.TempDir(), SegmentTimeout: 30 * time.Second}
}

func TestSessionStreamsAndSeeks(t *testing.T) {
	src := testSource(t)
	m := newManager(t)
	id, err := m.Start(src, cpuPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop(id)

	pl, err := m.Playlist(id)
	if err != nil || strings.Count(pl, ".ts\n") != 10 {
		t.Fatalf("playlist: %v\n%s", err, pl)
	}

	ctx := context.Background()
	for _, n := range []int{0, 1, 2} {
		if p, err := m.Segment(ctx, id, n); err != nil || !exists(p) {
			t.Fatalf("segment %d: %v", n, err)
		}
	}

	// Seeking far ahead restarts FFmpeg at that segment.
	s, _ := m.get(id)
	s.mu.Lock()
	first := s.proc
	s.mu.Unlock()
	if _, err := m.Segment(ctx, id, 8); err != nil {
		t.Fatalf("segment 8: %v", err)
	}
	s.mu.Lock()
	second := s.proc
	s.mu.Unlock()
	if second == first || second.start != 8 {
		t.Errorf("expected a restart at segment 8, got start=%d same=%v", second.start, second == first)
	}
	if _, err := m.Segment(ctx, id, 9); err != nil {
		t.Errorf("last segment: %v", err)
	}
	if _, err := m.Segment(ctx, id, 10); !errors.Is(err, ErrSegment) {
		t.Errorf("segment past the end: %v", err)
	}
}

func TestStopRemovesFiles(t *testing.T) {
	src := testSource(t)
	m := newManager(t)
	id, _ := m.Start(src, cpuPlan)
	if _, err := m.Segment(context.Background(), id, 0); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.WorkDir, id)
	m.Stop(id)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("session folder still exists: %v", err)
	}
	if _, err := m.Segment(context.Background(), id, 1); !errors.Is(err, ErrNoSession) {
		t.Errorf("after stop: %v", err)
	}
}

func TestLimitsAndIdle(t *testing.T) {
	m := &Manager{FFmpeg: "ffmpeg", WorkDir: t.TempDir(), MaxSessions: 1, IdleTimeout: time.Millisecond}
	src := Source{Path: "/nope.mkv", DurationSeconds: 60}
	id, err := m.Start(src, cpuPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(src, cpuPlan); !errors.Is(err, ErrBusy) {
		t.Errorf("second session: %v, want ErrBusy", err)
	}
	time.Sleep(5 * time.Millisecond)
	m.ReapIdle()
	if m.Count() != 0 {
		t.Errorf("idle session not stopped")
	}
	if _, err := m.Playlist(id); !errors.Is(err, ErrNoSession) {
		t.Errorf("reaped session still answers: %v", err)
	}
	if _, err := m.Start(Source{Path: "x"}, cpuPlan); err == nil {
		t.Error("unknown duration should be rejected")
	}
}

func TestConversionFailureIsReported(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	m := newManager(t)
	bad := filepath.Join(t.TempDir(), "broken.mkv")
	os.WriteFile(bad, []byte("not a video"), 0o644)
	id, _ := m.Start(Source{Path: bad, VideoCodec: "hevc", DurationSeconds: 60}, cpuPlan)
	defer m.Stop(id)
	if _, err := m.Segment(context.Background(), id, 0); err == nil || !strings.Contains(err.Error(), "conversion failed") {
		t.Errorf("err = %v", err)
	}
}
