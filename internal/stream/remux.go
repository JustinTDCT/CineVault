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
	if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".m4v") {
		return false
	}
	return true
}

// NeedsAudioTranscode checks if the audio codec needs transcoding to AAC
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
// Video is copied as-is, audio is converted to AAC if needed.
func ServeRemuxed(w http.ResponseWriter, r *http.Request, ffmpegPath, filePath, audioCodec string, startSeconds float64) error {
	args := []string{"-nostdin"}

	// Seek position before input for fast seeking
	if startSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", startSeconds))
	}

	args = append(args,
		"-i", filePath,
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", "copy",
	)

	// Audio handling
	if NeedsAudioTranscode(audioCodec) {
		args = append(args,
			"-c:a", "aac",
			"-ac", "2",
			"-b:a", "192k",
		)
	} else {
		args = append(args, "-c:a", "copy")
	}

	args = append(args,
		"-vsync", "passthrough",     // Don't alter video timestamps
		"-async", "1",               // Sync audio to video by inserting silence/dropping samples
		"-copyts",                   // Preserve original timestamps
		"-start_at_zero",            // But normalize to start at 0
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-max_muxing_queue_size", "4096",
		"-f", "mp4",
		"pipe:1",
	)

	cmd := exec.Command(ffmpegPath, args...)
	cmd.Stderr = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	audioAction := "copy"
	if NeedsAudioTranscode(audioCodec) {
		audioAction = "aac"
	}
	log.Printf("Remux starting: %s (audio: %s -> %s, seek: %.1fs)", filePath, audioCodec, audioAction, startSeconds)

	if err := cmd.Start(); err != nil {
		return err
	}

	buf := make([]byte, 256*1024)
	flusher, canFlush := w.(http.Flusher)

	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				cmd.Process.Kill()
				cmd.Wait()
				log.Printf("Remux: client disconnected")
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
		log.Printf("Remux FFmpeg exited: %v", err)
	}

	return nil
}
