package stream

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// ConvertToWebVTT converts a subtitle file to WebVTT format.
// Supports: SRT, ASS/SSA, VTT (passthrough).
func ConvertToWebVTT(filePath, format string) (string, error) {
	switch strings.ToLower(format) {
	case "webvtt", "vtt":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "subrip", "srt":
		return convertSRTToWebVTT(filePath)
	case "ass", "ssa":
		return convertASSToWebVTT(filePath)
	default:
		// Fallback: use ffmpeg to convert
		return convertViaFFmpeg(filePath)
	}
}

// ExtractEmbeddedSubtitle extracts an embedded subtitle stream to WebVTT using FFmpeg.
func ExtractEmbeddedSubtitle(ffmpegPath, mediaFilePath string, streamIndex int) (string, error) {
	args := []string{
		"-hide_banner", "-v", "error",
		"-i", mediaFilePath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-f", "webvtt",
		"pipe:",
	}

	cmd := exec.Command(ffmpegPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg subtitle extraction failed: %w", err)
	}
	return string(output), nil
}

// convertSRTToWebVTT converts SRT subtitle format to WebVTT.
func convertSRTToWebVTT(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	scanner := bufio.NewScanner(f)
	// Increase buffer size for long lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	timeRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}),(\d{3})`)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip BOM
		line = strings.TrimPrefix(line, "\xef\xbb\xbf")

		// Convert timestamps: replace comma with period
		if m := timeRegex.FindStringSubmatch(line); m != nil {
			sb.WriteString(fmt.Sprintf("%s.%s --> %s.%s\n", m[1], m[2], m[3], m[4]))
			continue
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String(), scanner.Err()
}

// convertASSToWebVTT converts ASS/SSA subtitle format to WebVTT.
func convertASSToWebVTT(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inEvents := false
	formatFields := []string{}
	textIdx := -1
	startIdx := -1
	endIdx := -1

	// Tag stripping regex for ASS override codes: {\...}
	tagRegex := regexp.MustCompile(`\{[^}]*\}`)
	cueNum := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[Events]") {
			inEvents = true
			continue
		}
		if strings.HasPrefix(line, "[") && inEvents {
			break // New section, done with events
		}

		if !inEvents {
			continue
		}

		if strings.HasPrefix(line, "Format:") {
			fields := strings.Split(strings.TrimPrefix(line, "Format:"), ",")
			for i, f := range fields {
				switch strings.TrimSpace(f) {
				case "Text":
					textIdx = i
				case "Start":
					startIdx = i
				case "End":
					endIdx = i
				}
			}
			formatFields = fields
			continue
		}

		if !strings.HasPrefix(line, "Dialogue:") || len(formatFields) == 0 {
			continue
		}

		parts := strings.SplitN(strings.TrimPrefix(line, "Dialogue:"), ",", len(formatFields))
		if textIdx < 0 || startIdx < 0 || endIdx < 0 || len(parts) <= textIdx {
			continue
		}

		start := convertASSTime(strings.TrimSpace(parts[startIdx]))
		end := convertASSTime(strings.TrimSpace(parts[endIdx]))
		text := strings.TrimSpace(parts[textIdx])

		// Strip ASS formatting tags
		text = tagRegex.ReplaceAllString(text, "")
		// Convert \N to newline
		text = strings.ReplaceAll(text, `\N`, "\n")
		text = strings.ReplaceAll(text, `\n`, "\n")

		if text == "" {
			continue
		}

		sb.WriteString(fmt.Sprintf("%d\n", cueNum))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", start, end))
		sb.WriteString(text)
		sb.WriteString("\n\n")
		cueNum++
	}

	return sb.String(), scanner.Err()
}

// convertASSTime converts ASS time format (H:MM:SS.CC) to WebVTT (HH:MM:SS.mmm).
func convertASSTime(t string) string {
	parts := strings.Split(t, ":")
	if len(parts) != 3 {
		return t
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])

	secParts := strings.Split(parts[2], ".")
	sec, _ := strconv.Atoi(secParts[0])
	cs := 0
	if len(secParts) > 1 {
		cs, _ = strconv.Atoi(secParts[1])
	}

	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, sec, cs*10)
}

// convertViaFFmpeg uses ffmpeg as a fallback converter for other subtitle formats.
func convertViaFFmpeg(filePath string) (string, error) {
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-v", "error",
		"-i", filePath,
		"-f", "webvtt",
		"pipe:",
	)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg subtitle conversion failed: %w", err)
	}
	return string(output), nil
}

// SubtitleBurnInFilter returns the FFmpeg filter string for burning in subtitles.
// For text-based subtitles (SRT, ASS, SSA), uses the subtitles filter.
// For image-based subtitles (PGS, VobSub), uses overlay filter via filter_complex.
func SubtitleBurnInFilter(filePath string, streamIndex int, format string) (string, bool) {
	lower := strings.ToLower(format)
	switch lower {
	case "subrip", "srt", "ass", "ssa", "webvtt", "vtt":
		// Text subtitles: use -vf subtitles filter with stream index
		// Escape special characters in file path for FFmpeg filter
		escaped := strings.ReplaceAll(filePath, "'", "'\\''")
		escaped = strings.ReplaceAll(escaped, ":", "\\:")
		escaped = strings.ReplaceAll(escaped, "[", "\\[")
		escaped = strings.ReplaceAll(escaped, "]", "\\]")
		return fmt.Sprintf("subtitles='%s':si=%d", escaped, streamIndex), false
	case "hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle", "pgssub", "vobsub":
		// Image subtitles: need filter_complex with overlay
		return fmt.Sprintf("[0:v][0:%d]overlay", streamIndex), true
	default:
		// Default: try text subtitles filter
		escaped := strings.ReplaceAll(filePath, "'", "'\\''")
		return fmt.Sprintf("subtitles='%s':si=%d", escaped, streamIndex), false
	}
}

// HDRToSDRFilter returns the FFmpeg tonemap filter chain for HDR-to-SDR conversion.
// Supports hardware-accelerated tone mapping when available.
func HDRToSDRFilter(hwAccel string) string {
	switch {
	case strings.Contains(hwAccel, "nvenc") || strings.Contains(hwAccel, "cuda"):
		// NVIDIA CUDA/OpenCL tone mapping
		return "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"
	case strings.Contains(hwAccel, "qsv"):
		// Intel QSV tone mapping (via VPP)
		return "vpp_qsv=tonemap=1"
	case strings.Contains(hwAccel, "vaapi"):
		// VAAPI tone mapping
		return "tonemap_vaapi=format=nv12:t=bt709:m=bt709:p=bt709"
	default:
		// Software tone mapping (zscale + tonemap)
		return "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"
	}
}

// BuildAudioTranscodeArgs returns FFmpeg args for audio transcoding.
// When the source audio codec is not browser-compatible, transcodes to AAC stereo
// or AC3 5.1 based on channel count. When gainDB is non-zero, a volume filter is
// applied and audio is forced to transcode (cannot apply filters with -c:a copy).
func BuildAudioTranscodeArgs(sourceCodec string, channels int, gainDB float64) []string {
	needsTranscode := NeedsAudioTranscode(sourceCodec)
	hasGain := gainDB != 0

	// If no gain and codec is compatible, copy audio as-is
	if !needsTranscode && !hasGain {
		return []string{"-c:a", "copy"}
	}

	var args []string

	// Apply volume filter when gain is non-zero
	if hasGain {
		args = append(args, "-af", fmt.Sprintf("volume=%.2fdB", gainDB))
	}

	if channels > 2 {
		args = append(args, "-c:a", "ac3", "-ac", "6", "-b:a", "384k")
	} else {
		args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
	}

	return args
}

// SelectAudioStream returns FFmpeg map args for selecting a specific audio stream.
func SelectAudioStream(streamIndex int) []string {
	return []string{"-map", fmt.Sprintf("0:%d", streamIndex)}
}

// ServeSubtitleFile reads a subtitle file and writes WebVTT content to the writer.
func ServeSubtitleFile(w io.Writer, filePath, format string) error {
	vtt, err := ConvertToWebVTT(filePath, format)
	if err != nil {
		return fmt.Errorf("convert to webvtt: %w", err)
	}
	_, err = io.WriteString(w, vtt)
	return err
}
