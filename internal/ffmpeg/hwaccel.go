package ffmpeg

import (
	"log"
	"os/exec"
	"strings"
	"sync"
)

// HWPreviewConfig holds the FFmpeg arguments needed to use a hardware encoder
// in preview/thumbnail generation pipelines.
type HWPreviewConfig struct {
	Encoder      string   // Encoder name (e.g. "h264_nvenc", "libx264")
	PreInputArgs []string // Args before -i (hw device init for vaapi)
	QualityArgs  []string // Quality control args (replaces -crf/-preset)
	FilterSuffix string   // Appended to end of filter chain (e.g. ",format=nv12,hwupload" for vaapi)
}

var (
	hwMu      sync.Mutex
	hwCached  string
	hwProbed  bool
)

// DetectH264Encoder probes for hardware H.264 encoders and verifies each
// by encoding a test frame. Falls back to libx264. Cached after first call.
func DetectH264Encoder(ffmpegPath string) string {
	hwMu.Lock()
	defer hwMu.Unlock()
	if hwProbed {
		return hwCached
	}
	hwProbed = true

	cmd := exec.Command(ffmpegPath, "-hide_banner", "-encoders")
	output, _ := cmd.Output()
	encoderList := string(output)

	for _, enc := range []string{"h264_nvenc", "h264_qsv", "h264_vaapi"} {
		if !strings.Contains(encoderList, enc) {
			continue
		}
		if testEncoder(ffmpegPath, enc) {
			log.Printf("[hwaccel] detected H.264 encoder: %s", enc)
			hwCached = enc
			return enc
		}
		log.Printf("[hwaccel] encoder %s compiled in but hardware test failed", enc)
	}

	log.Printf("[hwaccel] no hardware encoder available, using libx264")
	hwCached = "libx264"
	return "libx264"
}

// testEncoder verifies a hardware encoder works by encoding a single test frame.
func testEncoder(ffmpegPath, encoder string) bool {
	args := []string{"-hide_banner", "-v", "error"}

	switch {
	case strings.Contains(encoder, "qsv"):
		args = append(args, "-init_hw_device", "qsv=hw:/dev/dri/renderD128")
	case strings.Contains(encoder, "vaapi"):
		args = append(args, "-init_hw_device", "vaapi=/dev/dri/renderD128")
	}

	args = append(args,
		"-f", "lavfi", "-i", "color=black:s=64x64:d=0.1:r=1",
		"-frames:v", "1", "-an",
	)

	if strings.Contains(encoder, "vaapi") {
		args = append(args, "-vf", "format=nv12,hwupload")
	}

	args = append(args, "-c:v", encoder, "-f", "null", "-")

	cmd := exec.Command(ffmpegPath, args...)
	if err := cmd.Run(); err != nil {
		log.Printf("[hwaccel] encoder test failed for %s: %v", encoder, err)
		return false
	}
	return true
}

// PreviewEncodeConfig returns the full HWPreviewConfig for a detected encoder.
// Use in preview/clip generation to replace hardcoded libx264 args.
func PreviewEncodeConfig(encoder string) HWPreviewConfig {
	switch {
	case strings.Contains(encoder, "nvenc"):
		return HWPreviewConfig{
			Encoder:     encoder,
			QualityArgs: []string{"-preset", "p1", "-cq", "30", "-b:v", "0"},
		}
	case strings.Contains(encoder, "qsv"):
		return HWPreviewConfig{
			Encoder:     encoder,
			QualityArgs: []string{"-preset", "veryfast", "-global_quality", "30"},
		}
	case strings.Contains(encoder, "vaapi"):
		return HWPreviewConfig{
			Encoder:      encoder,
			PreInputArgs: []string{"-init_hw_device", "vaapi=/dev/dri/renderD128", "-filter_hw_device", "vaapi"},
			QualityArgs:  []string{"-qp", "30"},
			FilterSuffix: ",format=nv12,hwupload",
		}
	default:
		return HWPreviewConfig{
			Encoder:     "libx264",
			QualityArgs: []string{"-preset", "veryfast", "-crf", "28"},
		}
	}
}
