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
	ffmpegPath    string
	outputBase    string
	mu            sync.Mutex
	sessions      map[string]*Session
	hwMu          sync.Mutex
	cachedH264Enc string
	cachedHEVCEnc string
	h264Probed    bool
	hevcProbed    bool
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
	ErrorLog      string
}

// TranscodeOptions holds optional parameters for transcoding.
type TranscodeOptions struct {
	AudioStreamIndex int     // Specific audio stream index (-1 for default)
	AudioCodec       string  // Source audio codec
	AudioChannels    int     // Source audio channel count
	SubtitleIndex    int     // Subtitle stream index for burn-in (-1 for none)
	SubtitleFormat   string  // Subtitle codec name for burn-in filter selection
	BurnSubtitles    bool    // Whether to burn in subtitles
	HDRToSDR         bool    // Whether to convert HDR to SDR
	StartSeconds     float64 // Seek position (0 for beginning)
	Codec            string  // Output codec: "h264" (default), "hevc"
	GainDB           float64 // Audio normalization gain in dB (0 = no gain)
}

func NewTranscoder(ffmpegPath, outputBase string) *Transcoder {
	return &Transcoder{
		ffmpegPath: ffmpegPath,
		outputBase: outputBase,
		sessions:   make(map[string]*Session),
	}
}

func (t *Transcoder) DetectHWAccel() string {
	t.hwMu.Lock()
	defer t.hwMu.Unlock()
	if t.h264Probed {
		return t.cachedH264Enc
	}
	t.h264Probed = true

	// Get the encoder list once
	cmd := exec.Command(t.ffmpegPath, "-hide_banner", "-encoders")
	output, _ := cmd.Output()
	encoderList := string(output)

	for _, accel := range []string{"h264_nvenc", "h264_qsv", "h264_vaapi"} {
		if !strings.Contains(encoderList, accel) {
			continue
		}
		if t.testEncoderAvailable(accel) {
			log.Printf("Detected HW encoder: %s", accel)
			t.cachedH264Enc = accel
			return accel
		}
		log.Printf("Encoder %s compiled in but hardware test failed, skipping", accel)
	}
	log.Printf("No hardware encoder available, using libx264")
	t.cachedH264Enc = "libx264"
	return "libx264"
}

func (t *Transcoder) StartTranscode(mediaItemID, userID, filePath, quality string, opts ...TranscodeOptions) (*Session, error) {
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

	// Resolve options
	var opt TranscodeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Determine encoder
	encoder := t.DetectHWAccel()
	if opt.Codec == "hevc" {
		encoder = t.detectHEVCEncoder()
	}

	// Build hwaccel input args for hardware decode
	hwAccelArgs := t.buildHWAccelInputArgs(encoder)

	playlistPath := filepath.Join(outputDir, "stream.m3u8")

	args := []string{"-nostdin"}

	// Hardware decode (before -i)
	args = append(args, hwAccelArgs...)

	// Seek support (before -i for fast keyframe seek)
	if opt.StartSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", opt.StartSeconds))
	}

	args = append(args, "-i", filePath)

	// Video stream mapping
	args = append(args, "-map", "0:v:0")

	// Audio stream mapping
	if opt.AudioStreamIndex >= 0 {
		args = append(args, SelectAudioStream(opt.AudioStreamIndex)...)
	} else {
		args = append(args, "-map", "0:a:0")
	}

	// Build video filter chain
	var videoFilters []string

	// QSV hwaccel keeps decoded frames in GPU memory; download to system
	// memory so standard software filters (scale, subs, HDR tonemap) work.
	// The QSV encoder re-uploads to GPU internally.
	if strings.Contains(encoder, "qsv") {
		videoFilters = append(videoFilters, "hwdownload", "format=nv12")
	}

	videoFilters = append(videoFilters, fmt.Sprintf("scale=%d:%d", q.Width, q.Height))

	// HDR-to-SDR tone mapping
	if opt.HDRToSDR {
		// QSV frames are downloaded to system memory above,
		// so use software tone mapping instead of vpp_qsv.
		filterHint := encoder
		if strings.Contains(encoder, "qsv") {
			filterHint = "software"
		}
		toneMap := HDRToSDRFilter(filterHint)
		videoFilters = append(videoFilters, toneMap)
	}

	// Subtitle burn-in
	if opt.BurnSubtitles && opt.SubtitleIndex >= 0 {
		subFilter, isComplex := SubtitleBurnInFilter(filePath, opt.SubtitleIndex, opt.SubtitleFormat)
		if isComplex {
			// Image-based subtitles need filter_complex instead of -vf
			args = append(args, "-filter_complex", subFilter)
		} else {
			videoFilters = append(videoFilters, subFilter)
		}
	}

	args = append(args,
		"-c:v", encoder,
		"-vf", strings.Join(videoFilters, ","),
		"-b:v", q.VideoBitrate,
	)

	// Audio transcoding with smart codec/channel handling
	channels := 2
	audioCodec := "unknown"
	if opt.AudioChannels > 0 {
		channels = opt.AudioChannels
	}
	if opt.AudioCodec != "" {
		audioCodec = opt.AudioCodec
	}
	args = append(args, BuildAudioTranscodeArgs(audioCodec, channels, opt.GainDB)...)

	// HLS output — use fmp4 for HEVC (required by spec), mpegts for H.264
	segExt := "ts"
	hlsSegType := ""
	if opt.Codec == "hevc" {
		segExt = "mp4"
		hlsSegType = "-hls_segment_type fmp4"
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outputDir, fmt.Sprintf("segment_%%05d.%s", segExt)),
		"-hls_flags", "independent_segments",
	)
	if hlsSegType != "" {
		args = append(args, "-hls_segment_type", "fmp4")
	}
	args = append(args, "-y", playlistPath)

	cmd := exec.Command(t.ffmpegPath, args...)

	// Capture stderr for error logging
	stderrBuf := &strings.Builder{}
	cmd.Stderr = stderrBuf

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

	log.Printf("Transcode session started: %s (%s, encoder=%s)", sessionKey, quality, encoder)

	// Wait for FFmpeg in background, capture errors and track progress
	go func() {
		if err := cmd.Wait(); err != nil {
			errStr := stderrBuf.String()
			if len(errStr) > 1000 {
				errStr = errStr[len(errStr)-1000:]
			}
			log.Printf("FFmpeg transcode ended: %v | stderr: %s", err, errStr)
			t.mu.Lock()
			if s, ok := t.sessions[sessionKey]; ok {
				s.ErrorLog = errStr
			}
			t.mu.Unlock()
		}
		// Count segments produced
		t.mu.Lock()
		if s, ok := t.sessions[sessionKey]; ok {
			entries, _ := os.ReadDir(s.OutputDir)
			count := 0
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".ts") || strings.HasSuffix(e.Name(), ".mp4") {
					count++
				}
			}
			s.SegmentsReady = count
		}
		t.mu.Unlock()
	}()

	return session, nil
}

// detectHEVCEncoder probes for HEVC hardware encoders, falls back to libx265.
func (t *Transcoder) detectHEVCEncoder() string {
	t.hwMu.Lock()
	defer t.hwMu.Unlock()
	if t.hevcProbed {
		return t.cachedHEVCEnc
	}
	t.hevcProbed = true

	cmd := exec.Command(t.ffmpegPath, "-hide_banner", "-encoders")
	output, _ := cmd.Output()
	encoderList := string(output)

	for _, encoder := range []string{"hevc_nvenc", "hevc_qsv", "hevc_vaapi"} {
		if !strings.Contains(encoderList, encoder) {
			continue
		}
		if t.testEncoderAvailable(encoder) {
			log.Printf("Detected HEVC HW encoder: %s", encoder)
			t.cachedHEVCEnc = encoder
			return encoder
		}
		log.Printf("HEVC encoder %s compiled in but hardware test failed, skipping", encoder)
	}
	log.Printf("No HEVC hardware encoder available, using libx265")
	t.cachedHEVCEnc = "libx265"
	return "libx265"
}

// testEncoderAvailable verifies a hardware encoder actually works by encoding
// a single test frame.  This catches cases where the encoder is compiled into
// FFmpeg but the underlying hardware / driver is missing at runtime.
func (t *Transcoder) testEncoderAvailable(encoder string) bool {
	args := []string{"-hide_banner", "-v", "error"}

	// Initialise the appropriate hardware device so FFmpeg can reach the GPU.
	switch {
	case strings.Contains(encoder, "qsv"):
		args = append(args, "-init_hw_device", "qsv=hw:/dev/dri/renderD128")
	case strings.Contains(encoder, "vaapi"):
		args = append(args, "-init_hw_device", "vaapi=/dev/dri/renderD128")
	}

	// Minimal synthetic source – one frame, tiny resolution.
	args = append(args,
		"-f", "lavfi", "-i", "color=black:s=64x64:d=0.1:r=1",
		"-frames:v", "1", "-an",
	)

	// VAAPI encoders require frames on a hw surface; upload explicitly.
	if strings.Contains(encoder, "vaapi") {
		args = append(args, "-vf", "format=nv12,hwupload")
	}

	args = append(args, "-c:v", encoder, "-f", "null", "-")

	cmd := exec.Command(t.ffmpegPath, args...)
	if err := cmd.Run(); err != nil {
		log.Printf("Hardware encoder test failed for %s: %v", encoder, err)
		return false
	}
	return true
}

// buildHWAccelInputArgs returns FFmpeg args for hardware-accelerated decoding.
func (t *Transcoder) buildHWAccelInputArgs(encoder string) []string {
	switch {
	case strings.Contains(encoder, "nvenc") || strings.Contains(encoder, "cuda"):
		return []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
	case strings.Contains(encoder, "qsv"):
		// QSV on Linux uses VAAPI as backend; -qsv_device tells FFmpeg which
		// render node to use.  Decoded frames stay in QSV GPU memory and are
		// brought to system memory by the hwdownload filter prepended to -vf
		// in StartTranscode, so all software filters remain compatible.
		return []string{"-hwaccel", "qsv", "-qsv_device", "/dev/dri/renderD128", "-hwaccel_output_format", "qsv"}
	case strings.Contains(encoder, "vaapi"):
		return []string{"-hwaccel", "vaapi", "-hwaccel_output_format", "vaapi", "-vaapi_device", "/dev/dri/renderD128"}
	default:
		return nil
	}
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

	if s == nil {
		return
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
		s.Cmd.Wait()
	}
	if err := os.RemoveAll(s.OutputDir); err != nil {
		log.Printf("Transcode cleanup: failed to remove %s: %v", s.OutputDir, err)
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

// ActiveSessionCount returns the number of active transcode sessions.
func (t *Transcoder) ActiveSessionCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.sessions)
}

// HWAccel returns the detected hardware accelerator name.
func (t *Transcoder) HWAccel() string {
	return t.DetectHWAccel()
}

func (t *Transcoder) CleanupExpired(maxAge time.Duration) {
	t.mu.Lock()
	var expired []*Session
	var expiredIDs []string
	now := time.Now()
	for id, s := range t.sessions {
		if now.Sub(s.LastAccess) > maxAge {
			expired = append(expired, s)
			expiredIDs = append(expiredIDs, id)
			delete(t.sessions, id)
		}
	}
	t.mu.Unlock()

	for i, s := range expired {
		if s.Cmd != nil && s.Cmd.Process != nil {
			s.Cmd.Process.Kill()
			s.Cmd.Wait()
		}
		os.RemoveAll(s.OutputDir)
		log.Printf("Cleaned up expired session: %s", expiredIDs[i])
	}
}

// RunCleanupLoop starts a background goroutine that periodically cleans up
// expired transcode sessions. Stop it by cancelling the context.
func (t *Transcoder) RunCleanupLoop(done <-chan struct{}, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				t.CleanupExpired(maxAge)
			}
		}
	}()
}
