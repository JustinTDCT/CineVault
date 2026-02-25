package player

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
)

type TranscodeType string

const (
	TranscodeQSV  TranscodeType = "qsv"
	TranscodeCUDA TranscodeType = "cuda"
	TranscodeCPU  TranscodeType = "cpu"
)

type Transcoder struct {
	ffmpegPath   string
	accelType    TranscodeType
	maxSessions  int
	activeSessions int32
	dataDir      string
	mu           sync.Mutex
	sessions     map[string]*TranscodeSession
}

type TranscodeSession struct {
	ID       string
	FilePath string
	OutDir   string
	Cmd      *exec.Cmd
	Cancel   context.CancelFunc
}

func NewTranscoder(ffmpegPath, dataDir string, accelType string, maxSessions int) *Transcoder {
	return &Transcoder{
		ffmpegPath:  ffmpegPath,
		accelType:   TranscodeType(accelType),
		maxSessions: maxSessions,
		dataDir:     dataDir,
		sessions:    make(map[string]*TranscodeSession),
	}
}

func (t *Transcoder) CanStart() bool {
	return int(atomic.LoadInt32(&t.activeSessions)) < t.maxSessions
}

func (t *Transcoder) Start(sessionID, filePath, quality string) (*TranscodeSession, error) {
	if !t.CanStart() {
		return nil, fmt.Errorf("max concurrent transcodes reached (%d)", t.maxSessions)
	}

	outDir := filepath.Join(t.dataDir, "transcode", sessionID)
	os.MkdirAll(outDir, 0755)

	ctx, cancel := context.WithCancel(context.Background())
	args := t.buildArgs(filePath, outDir, quality)

	cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)
	cmd.Dir = outDir

	session := &TranscodeSession{
		ID:       sessionID,
		FilePath: filePath,
		OutDir:   outDir,
		Cmd:      cmd,
		Cancel:   cancel,
	}

	t.mu.Lock()
	t.sessions[sessionID] = session
	t.mu.Unlock()
	atomic.AddInt32(&t.activeSessions, 1)

	go func() {
		defer func() {
			atomic.AddInt32(&t.activeSessions, -1)
			t.mu.Lock()
			delete(t.sessions, sessionID)
			t.mu.Unlock()
		}()

		if err := cmd.Run(); err != nil {
			log.Printf("transcode %s error: %v", sessionID, err)
		}
	}()

	return session, nil
}

func (t *Transcoder) Stop(sessionID string) {
	t.mu.Lock()
	s, ok := t.sessions[sessionID]
	t.mu.Unlock()
	if ok {
		s.Cancel()
	}
}

func (t *Transcoder) buildArgs(input, outDir, quality string) []string {
	playlist := filepath.Join(outDir, "stream.m3u8")

	args := []string{"-hide_banner", "-y"}

	switch t.accelType {
	case TranscodeQSV:
		args = append(args, "-hwaccel", "qsv", "-hwaccel_output_format", "qsv")
	case TranscodeCUDA:
		args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
	}

	args = append(args, "-i", input)

	switch quality {
	case "1080p":
		args = append(args, "-vf", "scale=-2:1080", "-b:v", "5M")
	case "720p":
		args = append(args, "-vf", "scale=-2:720", "-b:v", "3M")
	case "480p":
		args = append(args, "-vf", "scale=-2:480", "-b:v", "1.5M")
	}

	switch t.accelType {
	case TranscodeQSV:
		args = append(args, "-c:v", "h264_qsv")
	case TranscodeCUDA:
		args = append(args, "-c:v", "h264_nvenc")
	default:
		args = append(args, "-c:v", "libx264", "-preset", "fast")
	}

	args = append(args,
		"-c:a", "aac", "-b:a", "192k",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outDir, "seg_%05d.ts"),
		playlist,
	)

	return args
}
