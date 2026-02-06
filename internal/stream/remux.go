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

// ServeTranscoded streams a video file transcoded to fragmented MP4 on-the-fly.
// This follows StashApp's proven approach:
//   - Video is RE-ENCODED to H264 (not copied) — this guarantees A/V timestamp
//     alignment when converting from MKV/AVI containers. StashApp only uses
//     -c:v copy for files already in MP4 container.
//   - Audio is transcoded to AAC stereo.
//   - Output is fragmented MP4 piped directly to the HTTP response.
//   - Uses exec.CommandContext tied to the HTTP request for automatic cleanup.
//   - Uses io.Copy for clean stdout→response piping.
//
// Reference: https://github.com/stashapp/stash — pkg/ffmpeg/stream_transcode.go
func ServeTranscoded(ctx context.Context, w http.ResponseWriter, ffmpegPath, filePath string, startSeconds float64) error {
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
		// Video: re-encode to H264 (guarantees timestamp sync)
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-preset", "veryfast",
		"-crf", "22",
		"-sc_threshold", "0",
		// Audio: AAC stereo
		"-c:a", "aac",
		"-ac", "2",
		"-b:a", "192k",
		// Output: fragmented MP4 for streaming
		"-movflags", "frag_keyframe+empty_moov",
		"-f", "mp4",
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

	log.Printf("Transcode stream: %s (seek: %.1fs)", filePath, startSeconds)
	log.Printf("Transcode cmd: %s %s", ffmpegPath, strings.Join(args, " "))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	// Must consume stderr in a goroutine to prevent deadlock (StashApp pattern)
	go func() {
		stderrBytes, _ := io.ReadAll(stderr)
		if err := cmd.Wait(); err != nil {
			// Only log if NOT a context cancellation (client disconnect)
			if ctx.Err() == nil {
				errStr := string(stderrBytes)
				if len(errStr) > 500 {
					errStr = errStr[len(errStr)-500:]
				}
				log.Printf("FFmpeg transcode error: %v | stderr: %s", err, errStr)
			}
		}
	}()

	// Set headers (StashApp pattern: no Content-Length, chunked transfer)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	// Pipe FFmpeg stdout directly to HTTP response
	if _, err := io.Copy(w, stdout); err != nil {
		// Ignore write errors from client disconnect (EPIPE, connection reset)
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("Transcode stream write error: %v", err)
	}

	// Flush if possible
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}
