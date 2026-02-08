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

// samplePoints defines the percentage offsets into the video where frames are extracted.
// Using multiple sample points increases accuracy by comparing content at different positions.
var samplePoints = []float64{0.05, 0.15, 0.30, 0.50, 0.70, 0.85, 0.95}

// ComputePHash generates a composite perceptual hash from multiple keyframes
// sampled at percentage-based positions throughout the video.
// The durationSec parameter is the total duration of the video in seconds.
func (f *Fingerprinter) ComputePHash(filePath string, durationSec int) (string, error) {
	if durationSec <= 0 {
		// Fallback: single frame at 1 second
		return f.computeSingleFrameHash(filePath, 1)
	}

	tmpDir, err := os.MkdirTemp(f.tempDir, "phash-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	var allHashBits []byte
	extracted := 0

	for i, pct := range samplePoints {
		seekSec := int(float64(durationSec) * pct)
		if seekSec <= 0 {
			seekSec = 1
		}
		// Don't seek past the end
		if seekSec >= durationSec {
			seekSec = durationSec - 1
		}
		if seekSec <= 0 {
			seekSec = 1
		}

		framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%d.jpg", i))

		cmd := exec.Command(f.ffmpegPath,
			"-ss", fmt.Sprintf("%d", seekSec),
			"-i", filePath,
			"-vframes", "1",
			"-vf", "scale=32:32",
			"-y",
			framePath,
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Frame extraction at %d%% (%ds) failed for %s: %s", int(pct*100), seekSec, filepath.Base(filePath), string(output))
			continue
		}

		bits, err := hashFrame(framePath)
		if err != nil {
			log.Printf("Hash frame at %d%% failed for %s: %v", int(pct*100), filepath.Base(filePath), err)
			continue
		}

		allHashBits = append(allHashBits, bits...)
		extracted++
	}

	if extracted == 0 {
		return "", fmt.Errorf("no frames could be extracted from %s", filepath.Base(filePath))
	}

	// Produce a single composite hash from all sampled frame bits
	hash := md5.Sum(allHashBits)
	return fmt.Sprintf("%x", hash), nil
}

// computeSingleFrameHash is a fallback for very short videos or when duration is unknown.
func (f *Fingerprinter) computeSingleFrameHash(filePath string, seekSec int) (string, error) {
	tmpDir, err := os.MkdirTemp(f.tempDir, "phash-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	framePath := filepath.Join(tmpDir, "frame.jpg")

	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%d", seekSec),
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

	bits, err := hashFrame(framePath)
	if err != nil {
		return "", err
	}

	hash := md5.Sum(bits)
	return fmt.Sprintf("%x", hash), nil
}

// hashFrame opens a JPEG frame and returns the binary hash bits (1/0 per pixel vs average).
func hashFrame(framePath string) ([]byte, error) {
	file, err := os.Open(framePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	pixels := make([]float64, 32*32)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			gray := float64(color.GrayModel.Convert(color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}).(color.Gray).Y)
			pixels[y*32+x] = gray
		}
	}

	var sum float64
	for _, v := range pixels {
		sum += v
	}
	avg := sum / float64(len(pixels))

	var hashBits []byte
	for _, v := range pixels {
		if v > avg {
			hashBits = append(hashBits, '1')
		} else {
			hashBits = append(hashBits, '0')
		}
	}

	return hashBits, nil
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

// IsDuplicate returns true if similarity is above threshold (default 0.90)
func IsDuplicate(hash1, hash2 string, threshold float64) bool {
	if threshold <= 0 {
		threshold = 0.90
	}
	return Similarity(hash1, hash2) >= threshold
}
