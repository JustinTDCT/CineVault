package preview

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

	ctx, cancel := context.WithTimeout(context.Background(), ffmpegTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, g.ffmpegPath,
		"-ss", fmt.Sprintf("%d", seekTo),
		"-i", filePath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Thumbnail generation timed out after %v: %s", ffmpegTimeout, filePath)
			return "", fmt.Errorf("thumbnail: timed out")
		}
		log.Printf("Thumbnail generation failed: %s", string(output))
		return "", fmt.Errorf("thumbnail: %w", err)
	}
	return outPath, nil
}

// GenerateSprite creates a sprite sheet for scrubber hover preview
func (g *Generator) GenerateSprite(mediaItemID, filePath string, durationSec int) (string, error) {
	outDir := filepath.Join(g.outputBase, mediaItemID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(outDir, "sprite.jpg")
	// Extract one frame every 10 seconds, tile into 10 columns
	interval := 10
	if durationSec > 600 {
		interval = durationSec / 60
	}

	ctx, cancel := context.WithTimeout(context.Background(), ffmpegTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, g.ffmpegPath,
		"-i", filePath,
		"-vf", fmt.Sprintf("fps=1/%d,scale=160:-1,tile=10x10", interval),
		"-q:v", "5",
		"-y",
		outPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Sprite generation timed out after %v: %s", ffmpegTimeout, filePath)
			return "", fmt.Errorf("sprite: timed out")
		}
		log.Printf("Sprite generation failed: %s", string(output))
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

	// Detect hardware encoder for faster preview generation
	encoder := ffmpeg.DetectH264Encoder(g.ffmpegPath)
	hwCfg := ffmpeg.PreviewEncodeConfig(encoder)

	// Build ffmpeg args: hw init (if needed), then one -ss/-t/-i per segment
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
		"-y", outPath,
	)

	ctx, cancel := context.WithTimeout(context.Background(), ffmpegTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, g.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Animated preview timed out after %v: %s", ffmpegTimeout, filePath)
			return "", fmt.Errorf("animated preview: timed out")
		}
		log.Printf("Animated preview failed: %s", string(output))
		return "", fmt.Errorf("animated preview: %w", err)
	}
	return outPath, nil
}
