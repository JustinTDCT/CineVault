package scanner

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/JustinTDCT/CineVault/internal/ffmpeg"
	"github.com/JustinTDCT/CineVault/internal/models"
)

const ffmpegPreviewTimeout = 2 * time.Minute

func (s *Scanner) GenerateScreenshotPoster(item *models.MediaItem) {
	s.generateScreenshotPoster(item)
}

// generateScreenshotPoster is the internal implementation.
func (s *Scanner) generateScreenshotPoster(item *models.MediaItem) {
	if s.ffmpegPath == "" || s.posterDir == "" {
		return
	}

	// Determine seek position: 50% into the video (halfway mark)
	seekSec := 5 // default for very short/unknown duration
	if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
		seekSec = *item.DurationSeconds / 2 // 50% — halfway mark
		if seekSec < 1 {
			seekSec = 1
		}
	}

	// Ensure output directory exists
	posterDir := filepath.Join(s.posterDir, "posters")
	if err := os.MkdirAll(posterDir, 0755); err != nil {
		log.Printf("Screenshot: failed to create poster dir: %v", err)
		return
	}

	filename := item.ID.String() + ".jpg"
	outPath := filepath.Join(posterDir, filename)

	cmd := exec.Command(s.ffmpegPath,
		"-ss", fmt.Sprintf("%d", seekSec),
		"-i", item.FilePath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Screenshot: failed for %s: %s", item.FileName, string(output))
		return
	}

	// Verify the file was actually created
	if _, err := os.Stat(outPath); err != nil {
		return
	}

	webPath := "/previews/posters/" + filename
	if err := s.mediaRepo.UpdatePosterPath(item.ID, webPath); err != nil {
		log.Printf("Screenshot: failed to update poster path for %s: %v", item.FileName, err)
		return
	}
	_ = s.mediaRepo.SetGeneratedPoster(item.ID, true)
	item.PosterPath = &webPath
	item.GeneratedPoster = true
	log.Printf("Screenshot: generated poster for %s at %ds", item.FileName, seekSec)
}

// runFFmpegWithTimeout starts an FFmpeg command in its own process group and
// kills the entire group if it exceeds the timeout.
func runFFmpegWithTimeout(cmd *exec.Cmd, timeout time.Duration) ([]byte, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return buf.Bytes(), err
	case <-time.After(timeout):
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		<-done
		return buf.Bytes(), fmt.Errorf("timed out after %v", timeout)
	}
}

// GeneratePreviewClip creates a short looping MP4 video preview (StashApp-style).
// Extracts multiple short clips from evenly-spaced positions across the video,
// concatenates them into a single H.264 MP4 at 320px width for fast card-hover playback.
// Stores in /previews/previews/{id}.mp4. Updates media_items.preview_path.
func (s *Scanner) GeneratePreviewClip(item *models.MediaItem) {
	if s.ffmpegPath == "" || s.posterDir == "" || item.DurationSeconds == nil || *item.DurationSeconds < 30 {
		return
	}

	previewDir := filepath.Join(s.posterDir, "previews")
	os.MkdirAll(previewDir, 0755)

	duration := *item.DurationSeconds

	numSegments := 8
	clipDuration := 1.5
	if duration < 60 {
		numSegments = 4
	}

	startOffset := float64(duration) * 0.05
	usable := float64(duration) * 0.90
	interval := usable / float64(numSegments)

	outFile := filepath.Join(previewDir, item.ID.String()+".mp4")

	// Detect hardware encoder for faster preview generation
	encoder := ffmpeg.DetectH264Encoder(s.ffmpegPath)
	hwCfg := ffmpeg.PreviewEncodeConfig(encoder)

	// Build ffmpeg args: hw init (if needed), then one -ss/-t/-i per segment
	args := make([]string, 0, numSegments*6+20)
	args = append(args, hwCfg.PreInputArgs...)
	for i := 0; i < numSegments; i++ {
		ss := startOffset + float64(i)*interval
		args = append(args,
			"-ss", fmt.Sprintf("%.2f", ss),
			"-t", fmt.Sprintf("%.2f", clipDuration),
			"-i", item.FilePath,
		)
	}

	// Build filter_complex: scale each stream then concat
	var filterParts string
	var concatInputs string
	for i := 0; i < numSegments; i++ {
		filterParts += fmt.Sprintf("[%d:v]scale=320:-2,setpts=PTS-STARTPTS[v%d];", i, i)
		concatInputs += fmt.Sprintf("[v%d]", i)
	}
	filterComplex := fmt.Sprintf("%s%sconcat=n=%d:v=1:a=0%s[out]", filterParts, concatInputs, numSegments, hwCfg.FilterSuffix)

	args = append(args,
		"-filter_complex", filterComplex,
		"-map", "[out]",
		"-c:v", hwCfg.Encoder,
	)
	args = append(args, hwCfg.QualityArgs...)
	args = append(args,
		"-an",
		"-movflags", "+faststart",
		"-y", outFile,
	)

	cmd := exec.Command(s.ffmpegPath, args...)
	if output, err := runFFmpegWithTimeout(cmd, ffmpegPreviewTimeout); err != nil {
		log.Printf("Preview: MP4 clip failed for %s: %v (%s)", item.FileName, err, string(output))
		return
	}

	webPath := "/previews/previews/" + item.ID.String() + ".mp4"
	if err := s.mediaRepo.UpdatePreviewPath(item.ID, webPath); err != nil {
		log.Printf("Preview: failed to store path for %s: %v", item.FileName, err)
		return
	}
	log.Printf("Preview: generated MP4 clip for %s (%d segments x %.1fs)", item.FileName, numSegments, clipDuration)
}

// GenerateTimelineThumbnails creates a sprite sheet of thumbnails for scrubber hover preview.
// Uses keyframe-only decoding (-skip_frame nokey) for ~100x faster extraction, matching
// the approach used by Jellyfin's trickplay implementation. Timeline thumbnails don't
// need exact frame positioning so keyframe approximation is ideal.
// Stores in /thumbnails/sprites/{id}.jpg. Updates media_items.sprite_path.
func (s *Scanner) GenerateTimelineThumbnails(item *models.MediaItem) {
	if s.ffmpegPath == "" || s.posterDir == "" || item.DurationSeconds == nil || *item.DurationSeconds < 10 {
		return
	}

	spriteDir := filepath.Join(s.posterDir, "sprites")
	os.MkdirAll(spriteDir, 0755)

	duration := *item.DurationSeconds
	interval := 10
	if duration > 600 {
		interval = duration / 60 // ~60 frames total
	}

	outFile := filepath.Join(spriteDir, item.ID.String()+".jpg")

	args := make([]string, 0, 20)
	// Keyframe-only decoding: decoder skips all P/B-frames, only outputs I-frames
	args = append(args, "-skip_frame", "nokey")
	if s.hwaccel != "none" {
		args = append(args, "-hwaccel", s.hwaccel)
	}
	args = append(args,
		"-i", item.FilePath,
		"-an", "-sn",
		"-vf", fmt.Sprintf("fps=1/%d,scale=160:-1,tile=10x10", interval),
		"-q:v", "5",
		"-vsync", "passthrough",
		"-threads", "4",
		"-y", outFile,
	)

	cmd := exec.Command(s.ffmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Sprite: generation failed for %s: %v (%s)", item.FileName, err, string(output))
		return
	}

	webPath := "/thumbnails/sprites/" + item.ID.String() + ".jpg"
	if err := s.mediaRepo.UpdateSpritePath(item.ID, webPath); err != nil {
		log.Printf("Sprite: failed to store path for %s: %v", item.FileName, err)
		return
	}
	log.Printf("Sprite: generated timeline thumbnails for %s (interval=%ds)", item.FileName, interval)
}
