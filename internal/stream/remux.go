package stream

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

// NeedsRemux returns true if the container isn't browser-native (MP4/WebM)
func NeedsRemux(filePath string) bool {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".m4v") {
		return false
	}
	return true
}

// NeedsAudioTranscode checks if the audio codec needs transcoding to AAC.
// Browser-compatible codecs (AAC, MP3, Opus, Vorbis, FLAC) can be copied;
// everything else (DTS, AC3, EAC3, TrueHD, etc.) must be transcoded.
func NeedsAudioTranscode(audioCodec string) bool {
	lower := strings.ToLower(audioCodec)
	switch lower {
	case "aac", "mp3", "opus", "vorbis", "flac":
		return false
	default:
		return true
	}
}

// ServeRemuxedMPEGTS streams a video file remuxed to MPEG-TS on-the-fly.
// This follows Plex's "Direct Stream" approach:
//   - Video is COPIED as-is (no re-encoding) — preserves quality, minimal CPU
//   - Audio is transcoded to AAC only if the source codec isn't browser-compatible
//   - Output is MPEG Transport Stream, which handles timestamp alignment natively
//     (unlike fragmented MP4 which has timestamp shift issues with -c:v copy from MKV)
//   - Frontend uses mpegts.js (MSE-based MPEG-TS player) for playback
//   - Seeking is handled by restarting FFmpeg with -ss at the seek position
//
// Why MPEGTS instead of fragmented MP4:
//   MPEGTS embeds PCR (Program Clock Reference) and per-packet PTS/DTS in PES headers,
//   guaranteeing A/V sync. Fragmented MP4's tfdt/trun atoms can cause timestamp shifts
//   of 4-5 seconds when copying video packets from MKV containers.
func ServeRemuxedMPEGTS(ctx context.Context, w http.ResponseWriter, ffmpegPath, filePath, audioCodec string, startSeconds float64) error {
	args := []string{
		"-hide_banner",
		"-v", "error",
	}

	// Input seeking (before -i for fast keyframe seek)
	if startSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", startSeconds))
	}

	args = append(args,
		"-i", filePath,
		"-map", "0:v:0", // First video stream only
		"-map", "0:a:0", // First audio stream only
		"-c:v", "copy",  // Copy video as-is (no re-encoding!)
	)

	// Audio: transcode incompatible codecs to AAC, copy compatible ones
	if NeedsAudioTranscode(audioCodec) {
		args = append(args,
			"-c:a", "aac",
			"-ac", "2",
			"-b:a", "192k",
		)
	} else {
		args = append(args, "-c:a", "copy")
	}

	// Output: MPEG Transport Stream for proper timestamp handling
	args = append(args,
		"-f", "mpegts",
		"pipe:",
	)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	audioAction := "copy"
	if NeedsAudioTranscode(audioCodec) {
		audioAction = "aac"
	}
	log.Printf("MPEGTS remux: %s (audio: %s->%s, seek: %.1fs)", filePath, audioCodec, audioAction, startSeconds)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	// Must consume stderr in goroutine to prevent deadlock
	go func() {
		stderrBytes, _ := io.ReadAll(stderr)
		if err := cmd.Wait(); err != nil {
			if ctx.Err() == nil {
				errStr := string(stderrBytes)
				if len(errStr) > 500 {
					errStr = errStr[len(errStr)-500:]
				}
				log.Printf("FFmpeg MPEGTS error: %v | stderr: %s", err, errStr)
			}
		}
	}()

	// Set headers — MPEG-TS content type, no caching, chunked transfer
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	// Pipe FFmpeg stdout directly to HTTP response
	if _, err := io.Copy(w, stdout); err != nil {
		if ctx.Err() != nil {
			return nil // Client disconnected, normal
		}
		log.Printf("MPEGTS stream write error: %v", err)
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}
