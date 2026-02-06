package stream

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Quality struct {
	Name       string
	Width      int
	Height     int
	VideoBitrate string
	AudioBitrate string
}

var Qualities = map[string]Quality{
	"360p":  {Name: "360p", Width: 640, Height: 360, VideoBitrate: "800k", AudioBitrate: "96k"},
	"480p":  {Name: "480p", Width: 854, Height: 480, VideoBitrate: "1400k", AudioBitrate: "128k"},
	"720p":  {Name: "720p", Width: 1280, Height: 720, VideoBitrate: "2800k", AudioBitrate: "128k"},
	"1080p": {Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: "5000k", AudioBitrate: "192k"},
	"4K":    {Name: "4K", Width: 3840, Height: 2160, VideoBitrate: "14000k", AudioBitrate: "192k"},
}

type Transcoder struct {
	ffmpegPath string
	outputBase string
	mu         sync.Mutex
	sessions   map[string]*Session
}

type Session struct {
	ID            string
	MediaItemID   string
	UserID        string
	Quality       string
	OutputDir     string
	Cmd           *exec.Cmd
	SegmentsReady int
	StartedAt     time.Time
	LastAccess    time.Time
}

func NewTranscoder(ffmpegPath, outputBase string) *Transcoder {
	return &Transcoder{
		ffmpegPath: ffmpegPath,
		outputBase: outputBase,
		sessions:   make(map[string]*Session),
	}
}

func (t *Transcoder) DetectHWAccel() string {
	// Try hardware encoders in order
	for _, accel := range []string{"h264_nvenc", "h264_qsv", "h264_vaapi"} {
		cmd := exec.Command(t.ffmpegPath, "-hide_banner", "-encoders")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), accel) {
			log.Printf("Detected HW encoder: %s", accel)
			return accel
		}
	}
	return "libx264"
}

func (t *Transcoder) StartTranscode(mediaItemID, userID, filePath, quality string) (*Session, error) {
	q, ok := Qualities[quality]
	if !ok {
		q = Qualities["720p"]
		quality = "720p"
	}

	// Use composite key so we can find existing sessions
	sessionKey := fmt.Sprintf("%s-%s", mediaItemID, quality)

	// Return existing session if already running
	t.mu.Lock()
	if existing, ok := t.sessions[sessionKey]; ok {
		existing.LastAccess = time.Now()
		t.mu.Unlock()
		return existing, nil
	}
	t.mu.Unlock()

	outputID := uuid.New().String()
	outputDir := filepath.Join(t.outputBase, outputID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	encoder := t.DetectHWAccel()
	playlistPath := filepath.Join(outputDir, "stream.m3u8")

	args := []string{
		"-nostdin",
		"-i", filePath,
		"-c:v", encoder,
		"-vf", fmt.Sprintf("scale=%d:%d", q.Width, q.Height),
		"-b:v", q.VideoBitrate,
		"-c:a", "aac",
		"-b:a", q.AudioBitrate,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outputDir, "segment_%05d.ts"),
		"-hls_flags", "independent_segments",
		"-y",
		playlistPath,
	}

	cmd := exec.Command(t.ffmpegPath, args...)
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	session := &Session{
		ID:          sessionKey,
		MediaItemID: mediaItemID,
		UserID:      userID,
		Quality:     quality,
		OutputDir:   outputDir,
		Cmd:         cmd,
		StartedAt:   time.Now(),
		LastAccess:  time.Now(),
	}

	t.mu.Lock()
	t.sessions[sessionKey] = session
	t.mu.Unlock()

	log.Printf("Transcode session started: %s (%s)", sessionKey, quality)

	// Wait for FFmpeg in background
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("FFmpeg transcode ended: %v", err)
		}
	}()

	return session, nil
}

// Note: HLS-based remux (StartRemux) has been removed.
// Non-native formats are now handled via piped on-the-fly transcoding
// in ServeTranscoded (remux.go), following StashApp's approach.
// This gives instant start and instant seeking (restart FFmpeg with -ss).

func (t *Transcoder) GetSession(sessionID string) *Session {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.sessions[sessionID]
	if s != nil {
		s.LastAccess = time.Now()
	}
	return s
}

func (t *Transcoder) StopSession(sessionID string) {
	t.mu.Lock()
	s := t.sessions[sessionID]
	delete(t.sessions, sessionID)
	t.mu.Unlock()

	if s != nil && s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
	}
	if s != nil {
		os.RemoveAll(s.OutputDir)
	}
}

func (t *Transcoder) GenerateMasterPlaylist(mediaItemID, filePath string, availableQualities []string) string {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")

	for _, qName := range availableQualities {
		q := Qualities[qName]
		sb.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%dx%d,NAME=\"%s\"\n",
			strings.TrimSuffix(q.VideoBitrate, "k")+"000", q.Width, q.Height, q.Name))
		sb.WriteString(fmt.Sprintf("/api/v1/stream/%s/%s/stream.m3u8\n", mediaItemID, qName))
	}

	return sb.String()
}

func (t *Transcoder) CleanupExpired(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for id, s := range t.sessions {
		if now.Sub(s.LastAccess) > maxAge {
			if s.Cmd != nil && s.Cmd.Process != nil {
				s.Cmd.Process.Kill()
			}
			os.RemoveAll(s.OutputDir)
			delete(t.sessions, id)
			log.Printf("Cleaned up expired session: %s", id)
		}
	}
}
