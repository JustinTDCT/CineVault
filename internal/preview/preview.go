package preview

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
)

const ffmpegTimeout = 2 * time.Minute

type Generator struct {
	ffmpegPath  string
	ffprobePath string
	outputBase  string
}

func NewGenerator(ffmpegPath, ffprobePath, outputBase string) *Generator {
	return &Generator{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		outputBase:  outputBase,
	}
}

// runFFmpegWithTimeout starts an FFmpeg command in its own process group and
// kills the entire group if it exceeds the timeout. This avoids the known
// issue where exec.CommandContext + CombinedOutput blocks on pipe drain even
// after the process is signaled.
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
		// Kill the entire process group so no orphans survive
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		<-done // wait for Wait() to return after kill
		return buf.Bytes(), fmt.Errorf("timed out after %v", timeout)
	}
}

// GenerateThumbnail extracts a poster frame at ~10% into the video
func (g *Generator) GenerateThumbnail(mediaItemID, filePath string, durationSec int) (string, error) {
	outDir := filepath.Join(g.outputBase, mediaItemID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(outDir, "thumbnail.jpg")
	seekTo := durationSec / 10
	if seekTo < 1 {
		seekTo = 1
	}

	cmd := exec.Command(g.ffmpegPath,
		"-ss", fmt.Sprintf("%d", seekTo),
		"-i", filePath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outPath,
	)
	output, err := runFFmpegWithTimeout(cmd, ffmpegTimeout)
	if err != nil {
		log.Printf("Thumbnail generation failed for %s: %v\n%s", filePath, err, string(output))
		return "", fmt.Errorf("thumbnail: %w", err)
	}
	return outPath, nil
}

// GenerateSprite creates a sprite sheet for scrubber hover preview.
// Uses keyframe-only decoding for ~100x faster extraction.
func (g *Generator) GenerateSprite(mediaItemID, filePath string, durationSec int) (string, error) {
	outDir := filepath.Join(g.outputBase, mediaItemID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(outDir, "sprite.jpg")
	interval := 10
	if durationSec > 600 {
		interval = durationSec / 60
	}

	cmd := exec.Command(g.ffmpegPath,
		"-skip_frame", "nokey",
		"-i", filePath,
		"-an", "-sn",
		"-vf", fmt.Sprintf("fps=1/%d,scale=160:-1,tile=10x10", interval),
		"-q:v", "5",
		"-vsync", "passthrough",
		"-threads", "4",
		"-y",
		outPath,
	)
	output, err := runFFmpegWithTimeout(cmd, ffmpegTimeout)
	if err != nil {
		log.Printf("Sprite generation failed for %s: %v\n%s", filePath, err, string(output))
		return "", fmt.Errorf("sprite: %w", err)
	}
	return outPath, nil
}

// GenerateAnimatedPreview creates a short looping MP4 video preview (StashApp-style).
// Extracts multiple short clips from evenly-spaced positions, concatenates them into
// a single MP4 encoded with H.264 at low resolution for fast card-hover playback.
func (g *Generator) GenerateAnimatedPreview(mediaItemID, filePath string, durationSec int) (string, error) {
	outDir := filepath.Join(g.outputBase, mediaItemID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(outDir, "preview.mp4")

	numSegments := 8
	clipDuration := 1.5
	if durationSec < 60 {
		numSegments = 4
	}

	startOffset := float64(durationSec) * 0.05
	usable := float64(durationSec) * 0.90
	interval := usable / float64(numSegments)

	encoder := ffmpeg.DetectH264Encoder(g.ffmpegPath)
	hwCfg := ffmpeg.PreviewEncodeConfig(encoder)

	args := make([]string, 0, numSegments*6+20)
	args = append(args, hwCfg.PreInputArgs...)
	for i := 0; i < numSegments; i++ {
		ss := startOffset + float64(i)*interval
		args = append(args,
			"-ss", fmt.Sprintf("%.2f", ss),
			"-t", fmt.Sprintf("%.2f", clipDuration),
			"-i", filePath,
		)
	}

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
		"-y", outPath,
	)

	cmd := exec.Command(g.ffmpegPath, args...)
	output, err := runFFmpegWithTimeout(cmd, ffmpegTimeout)
	if err != nil {
		log.Printf("Animated preview failed for %s: %v\n%s", filePath, err, string(output))
		return "", fmt.Errorf("animated preview: %w", err)
	}
	return outPath, nil
}
