package preview

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

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

	cmd := exec.Command(g.ffmpegPath,
		"-ss", fmt.Sprintf("%d", seekTo),
		"-i", filePath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
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

	cmd := exec.Command(g.ffmpegPath,
		"-i", filePath,
		"-vf", fmt.Sprintf("fps=1/%d,scale=160:-1,tile=10x10", interval),
		"-q:v", "5",
		"-y",
		outPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Sprite generation failed: %s", string(output))
		return "", fmt.Errorf("sprite: %w", err)
	}
	return outPath, nil
}

// GenerateAnimatedPreview creates a short animated WebP/GIF preview
func (g *Generator) GenerateAnimatedPreview(mediaItemID, filePath string, durationSec int) (string, error) {
	outDir := filepath.Join(g.outputBase, mediaItemID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(outDir, "preview.webp")
	// Take 3-second clips from 3 different points
	start1 := durationSec / 4
	start2 := durationSec / 2
	start3 := durationSec * 3 / 4
	if start1 < 1 {
		start1 = 1
	}

	// Use complex filter to concatenate segments
	cmd := exec.Command(g.ffmpegPath,
		"-ss", fmt.Sprintf("%d", start1), "-t", "2", "-i", filePath,
		"-ss", fmt.Sprintf("%d", start2), "-t", "2", "-i", filePath,
		"-ss", fmt.Sprintf("%d", start3), "-t", "2", "-i", filePath,
		"-filter_complex", "[0:v]scale=320:-1[v0];[1:v]scale=320:-1[v1];[2:v]scale=320:-1[v2];[v0][v1][v2]concat=n=3:v=1[out]",
		"-map", "[out]",
		"-loop", "0",
		"-y",
		outPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Animated preview failed: %s", string(output))
		return "", fmt.Errorf("animated preview: %w", err)
	}
	return outPath, nil
}
