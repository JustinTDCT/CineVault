package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 64*1024)
	total := 0
	for total < 1024*1024 {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			total += n
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func VideoPHash(ffmpegPath, filePath string) (string, error) {
	cmd := exec.Command(ffmpegPath,
		"-i", filePath,
		"-vf", "thumbnail=300,scale=8:8,format=gray",
		"-frames:v", "1",
		"-f", "rawvideo",
		"-")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("phash extract: %w", err)
	}

	if len(out) < 64 {
		return "", fmt.Errorf("phash: insufficient pixel data")
	}

	var sum int
	for _, b := range out[:64] {
		sum += int(b)
	}
	avg := sum / 64

	var hash uint64
	for i, b := range out[:64] {
		if int(b) > avg {
			hash |= 1 << uint(63-i)
		}
	}
	return fmt.Sprintf("%016x", hash), nil
}

func ChromaprintFingerprint(filePath string) (string, int, error) {
	cmd := exec.Command("fpcalc", "-json", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", 0, fmt.Errorf("fpcalc: %w", err)
	}

	result := string(out)
	var fingerprint string
	var duration int

	for _, line := range strings.Split(result, "\n") {
		if strings.HasPrefix(line, "FINGERPRINT=") {
			fingerprint = strings.TrimPrefix(line, "FINGERPRINT=")
		}
		if strings.HasPrefix(line, "DURATION=") {
			fmt.Sscanf(strings.TrimPrefix(line, "DURATION="), "%d", &duration)
		}
	}

	return fingerprint, duration, nil
}
