package fingerprint

import (
	"crypto/md5"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Fingerprinter struct {
	ffmpegPath string
	tempDir    string
}

func NewFingerprinter(ffmpegPath, tempDir string) *Fingerprinter {
	return &Fingerprinter{ffmpegPath: ffmpegPath, tempDir: tempDir}
}

// ComputePHash generates a perceptual hash from a video keyframe
func (f *Fingerprinter) ComputePHash(filePath string, seekSeconds int) (string, error) {
	tmpDir, err := os.MkdirTemp(f.tempDir, "phash-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	framePath := filepath.Join(tmpDir, "frame.jpg")

	// Extract a frame
	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%d", seekSeconds),
		"-i", filePath,
		"-vframes", "1",
		"-vf", "scale=32:32",
		"-y",
		framePath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Frame extraction failed: %s", string(output))
		return "", fmt.Errorf("extract frame: %w", err)
	}

	// Open and convert to grayscale
	file, err := os.Open(framePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	// Convert to 32x32 grayscale values
	bounds := img.Bounds()
	pixels := make([]float64, 32*32)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			gray := float64(color.GrayModel.Convert(color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}).(color.Gray).Y)
			pixels[y*32+x] = gray
		}
	}

	// Compute average
	var sum float64
	for _, v := range pixels {
		sum += v
	}
	avg := sum / float64(len(pixels))

	// Generate hash: 1 if pixel > avg, else 0
	var hashBits []byte
	for _, v := range pixels {
		if v > avg {
			hashBits = append(hashBits, '1')
		} else {
			hashBits = append(hashBits, '0')
		}
	}

	// Convert to hex for compact storage
	hash := md5.Sum(hashBits)
	return fmt.Sprintf("%x", hash), nil
}

// ComputeAudioFingerprint uses fpcalc (Chromaprint) if available, falls back to FFmpeg spectral
func (f *Fingerprinter) ComputeAudioFingerprint(filePath string) (string, error) {
	// Try fpcalc first
	fpcalcPath, err := exec.LookPath("fpcalc")
	if err == nil {
		cmd := exec.Command(fpcalcPath, "-raw", filePath)
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "FINGERPRINT=") {
					return strings.TrimPrefix(line, "FINGERPRINT="), nil
				}
			}
		}
	}

	// Fallback: extract audio stats via FFmpeg
	cmd := exec.Command(f.ffmpegPath,
		"-i", filePath,
		"-t", "60",
		"-af", "astats=metadata=1:reset=1",
		"-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("audio analysis: %w", err)
	}

	hash := md5.Sum(output)
	return fmt.Sprintf("audio:%x", hash), nil
}

// HammingDistance computes the number of differing bits between two hex hashes
func HammingDistance(hash1, hash2 string) int {
	if len(hash1) != len(hash2) {
		return -1
	}

	distance := 0
	for i := 0; i < len(hash1); i++ {
		v1, _ := strconv.ParseUint(string(hash1[i]), 16, 8)
		v2, _ := strconv.ParseUint(string(hash2[i]), 16, 8)
		xor := v1 ^ v2
		for xor > 0 {
			distance += int(xor & 1)
			xor >>= 1
		}
	}
	return distance
}

// Similarity returns a 0-1 score (1 = identical)
func Similarity(hash1, hash2 string) float64 {
	dist := HammingDistance(hash1, hash2)
	if dist < 0 {
		return 0
	}
	maxBits := len(hash1) * 4 // 4 bits per hex char
	return 1.0 - float64(dist)/float64(maxBits)
}

// IsDuplicate returns true if similarity is above threshold (default 0.97)
func IsDuplicate(hash1, hash2 string, threshold float64) bool {
	if threshold <= 0 {
		threshold = 0.97
	}
	return Similarity(hash1, hash2) >= threshold
}

