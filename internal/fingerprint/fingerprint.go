package fingerprint

import (
	"crypto/md5"
	"encoding/hex"
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

// hashSize is the width/height of the scaled frame for perceptual hashing.
// 8x8 = 64 bits per frame, 7 frames = 448 bits = 56 bytes = 112 hex chars.
const hashSize = 8

// ComputePHash generates a composite perceptual hash from multiple keyframes
// sampled at percentage-based positions throughout the video.
// Each frame is scaled to 8x8 grayscale and produces a 64-bit average hash.
// The per-frame hashes are concatenated into a single hex string that supports
// meaningful Hamming distance comparison for duplicate detection.
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

	var allHashBytes []byte
	extracted := 0

	for i, pct := range samplePoints {
		seekSec := int(float64(durationSec) * pct)
		if seekSec <= 0 {
			seekSec = 1
		}
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
			"-vf", fmt.Sprintf("scale=%d:%d", hashSize, hashSize),
			"-y",
			framePath,
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Frame extraction at %d%% (%ds) failed for %s: %s", int(pct*100), seekSec, filepath.Base(filePath), string(output))
			continue
		}

		frameBytes, err := hashFrame(framePath)
		if err != nil {
			log.Printf("Hash frame at %d%% failed for %s: %v", int(pct*100), filepath.Base(filePath), err)
			continue
		}

		allHashBytes = append(allHashBytes, frameBytes...)
		extracted++
	}

	if extracted == 0 {
		return "", fmt.Errorf("no frames could be extracted from %s", filepath.Base(filePath))
	}

	return hex.EncodeToString(allHashBytes), nil
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
		"-vf", fmt.Sprintf("scale=%d:%d", hashSize, hashSize),
		"-y",
		framePath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Frame extraction failed: %s", string(output))
		return "", fmt.Errorf("extract frame: %w", err)
	}

	frameBytes, err := hashFrame(framePath)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(frameBytes), nil
}

// hashFrame opens a JPEG frame and returns the packed perceptual hash bytes.
// It computes an average hash (aHash): each pixel above the mean is 1, below is 0.
// For an 8x8 frame, this produces 64 bits = 8 bytes.
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
	totalPixels := hashSize * hashSize
	pixels := make([]float64, totalPixels)
	for y := 0; y < hashSize; y++ {
		for x := 0; x < hashSize; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			gray := float64(color.GrayModel.Convert(color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}).(color.Gray).Y)
			pixels[y*hashSize+x] = gray
		}
	}

	var sum float64
	for _, v := range pixels {
		sum += v
	}
	avg := sum / float64(len(pixels))

	// Pack bits into bytes: 8 pixels per byte, MSB first
	numBytes := (totalPixels + 7) / 8
	hashBytes := make([]byte, numBytes)
	for i, v := range pixels {
		if v > avg {
			hashBytes[i/8] |= 1 << (7 - uint(i%8))
		}
	}

	return hashBytes, nil
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
