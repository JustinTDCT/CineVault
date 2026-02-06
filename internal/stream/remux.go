package stream

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

// NeedsRemux returns true if the container isn't browser-native (MP4/WebM)
func NeedsRemux(filePath string) bool {
	lower := strings.ToLower(filePath)
	// Browsers can play MP4/WebM/M4V natively
	if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".m4v") {
		return false
	}
	return true
}

// NeedsAudioTranscode checks if the audio codec needs transcoding to AAC
// Browsers support: AAC, MP3, Opus, Vorbis, FLAC
// Browsers do NOT support: DTS, AC3, EAC3, TrueHD
func NeedsAudioTranscode(audioCodec string) bool {
	lower := strings.ToLower(audioCodec)
	switch lower {
	case "aac", "mp3", "opus", "vorbis", "flac":
		return false
	default:
		return true
	}
}

// ServeRemuxed streams a video file remuxed to fragmented MP4 on-the-fly.
// Video is copied as-is (no transcoding), audio is converted to AAC if needed.
// startSeconds allows seeking - FFmpeg starts from that position.
// This is how Plex/Jellyfin handle "direct stream" for MKV files.
func ServeRemuxed(w http.ResponseWriter, r *http.Request, ffmpegPath, filePath, audioCodec string, startSeconds float64) error {
	// Determine audio handling
	audioArgs := []string{"-c:a", "copy"}
	if NeedsAudioTranscode(audioCodec) {
		// Transcode audio to AAC with sync correction
		audioArgs = []string{
			"-c:a", "aac",
			"-b:a", "192k",
			"-af", "aresample=async=1:first_pts=0", // Force audio sync to video timestamps
		}
	}

	// Build FFmpeg command for on-the-fly remux
	args := []string{
		"-fflags", "+genpts+discardcorrupt", // Regenerate timestamps, discard corrupt frames
	}

	// Add seek position before input for fast seeking
	if startSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", startSeconds))
	}

	args = append(args,
		"-i", filePath,
		"-c:v", "copy", // Copy video stream as-is (no re-encoding)
	)
	args = append(args, audioArgs...)
	args = append(args,
		"-avoid_negative_ts", "make_zero",   // Normalize timestamps to start at 0
		"-start_at_zero",                     // Ensure both streams start at timestamp 0
		"-movflags", "frag_keyframe+empty_moov+default_base_moof", // Fragmented MP4 for streaming
		"-f", "mp4",      // Output as MP4 container
		"-map", "0:v:0",  // Map first video stream
		"-map", "0:a:0",  // Map first audio stream (avoids subtitle/data issues)
		"pipe:1",         // Output to stdout
	)

	cmd := exec.Command(ffmpegPath, args...)

	// Pipe FFmpeg stdout directly to HTTP response
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// Set headers before starting
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	log.Printf("Remux starting: %s (audio: %s -> %s)", filePath, audioCodec, audioArgs[1])

	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream the output - use a generous buffer for smooth playback
	buf := make([]byte, 256*1024) // 256KB buffer
	flusher, canFlush := w.(http.Flusher)

	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				// Client disconnected, kill FFmpeg
				cmd.Process.Kill()
				cmd.Wait()
				log.Printf("Remux: client disconnected, killed FFmpeg")
				return nil
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		// FFmpeg may exit with error if client disconnects early - that's OK
		log.Printf("Remux FFmpeg exited: %v", err)
	}

	return nil
}
