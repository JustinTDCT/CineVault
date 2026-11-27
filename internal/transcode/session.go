package transcode

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by the session manager.
var (
	ErrNoSession = errors.New("playback session not found")
	ErrBusy      = errors.New("too many videos are being converted right now")
	ErrSegment   = errors.New("no such segment")
)

// Source is the file a session streams.
type Source struct {
	Path            string
	VideoCodec      string
	Height          int
	DurationSeconds float64
}

// Manager runs HLS sessions. Each session converts one file with FFmpeg,
// producing segments on demand: a request far from what's being produced
// restarts FFmpeg at that segment.
type Manager struct {
	FFmpeg      string        // ffmpeg binary
	WorkDir     string        // parent folder for session folders
	MaxSessions int           // concurrent sessions; 0 means 4
	IdleTimeout time.Duration // sessions with no requests for this long are stopped; 0 means 2 minutes
	// SegmentTimeout bounds how long a request waits for a segment; 0 means 60s.
	SegmentTimeout time.Duration
	Log            *slog.Logger

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	id     string
	src    Source
	plan   Plan
	dir    string
	segs   int
	mu     sync.Mutex
	proc   *proc
	last   time.Time
	closed bool
}

// proc is one running FFmpeg. err and stderr are only read after done closes.
type proc struct {
	cmd    *exec.Cmd
	start  int // first segment it produces
	done   chan struct{}
	err    error
	stderr bytes.Buffer
	killed atomic.Bool // stopped on purpose (seek, stop); not a failure
}

// lookahead is how far beyond the next missing segment a request may be and
// still wait for the running FFmpeg rather than restart it.
const lookahead = 3

// Start begins a session for src. FFmpeg starts on the first segment request.
func (m *Manager) Start(src Source, plan Plan) (string, error) {
	if src.DurationSeconds <= 0 {
		return "", errors.New("the file's length is unknown, so it can't be streamed; rescan the library")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions == nil {
		m.sessions = map[string]*session{}
	}
	limit := m.MaxSessions
	if limit == 0 {
		limit = 4
	}
	if len(m.sessions) >= limit {
		return "", ErrBusy
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(m.WorkDir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create session folder: %w", err)
	}
	m.sessions[id] = &session{id: id, src: src, plan: plan, dir: dir, segs: SegmentCount(src.DurationSeconds), last: time.Now()}
	return id, nil
}

// Playlist returns the session's VOD playlist.
func (m *Manager) Playlist(id string) (string, error) {
	s, err := m.get(id)
	if err != nil {
		return "", err
	}
	return Playlist(s.src.DurationSeconds), nil
}

// Segment returns the path of segment n, waiting for FFmpeg to produce it and
// starting or restarting FFmpeg as needed.
func (m *Manager) Segment(ctx context.Context, id string, n int) (string, error) {
	s, err := m.get(id)
	if err != nil {
		return "", err
	}
	if n < 0 || n >= s.segs {
		return "", ErrSegment
	}
	path := filepath.Join(s.dir, fmt.Sprintf("seg%d.ts", n))
	if exists(path) {
		return path, nil
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", ErrNoSession
	}
	p := s.proc
	if p == nil || !p.running() || n < p.start || n > s.nextMissing(p.start)+lookahead {
		if p != nil {
			p.stop()
		}
		p, err = m.spawn(s, n)
		if err != nil {
			s.mu.Unlock()
			return "", err
		}
		s.proc = p
	}
	s.mu.Unlock()

	timeout := m.SegmentTimeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if exists(path) {
			return path, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("segment %d wasn't ready after %s", n, timeout)
		case <-p.done:
			if exists(path) {
				return path, nil
			}
			if p.err != nil && !p.stopped() {
				return "", fmt.Errorf("conversion failed: %s", firstLine(p.stderr.String(), p.err))
			}
			if p.stopped() {
				// Replaced by a newer FFmpeg (another request seeked); ask again.
				return m.Segment(ctx, id, n)
			}
			return "", fmt.Errorf("segment %d wasn't produced", n)
		case <-tick.C:
		}
	}
}

// Stop ends a session and deletes its files.
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.closed = true
	if s.proc != nil {
		s.proc.stop()
	}
	s.mu.Unlock()
	_ = os.RemoveAll(s.dir)
}

// StopAll ends every session, for shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}

// Count is the number of open sessions.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// ReapIdle stops sessions nobody has requested anything from recently.
func (m *Manager) ReapIdle() {
	idle := m.IdleTimeout
	if idle == 0 {
		idle = 2 * time.Minute
	}
	m.mu.Lock()
	var stale []string
	for id, s := range m.sessions {
		s.mu.Lock()
		if time.Since(s.last) > idle {
			stale = append(stale, id)
		}
		s.mu.Unlock()
	}
	m.mu.Unlock()
	for _, id := range stale {
		m.logger().Info("stopping idle playback session", "session", id)
		m.Stop(id)
	}
}

// RunReaper calls ReapIdle every 30 seconds until ctx ends, then stops all sessions.
func (m *Manager) RunReaper(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.StopAll()
			return
		case <-t.C:
			m.ReapIdle()
		}
	}
}

func (m *Manager) get(id string) (*session, error) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return nil, ErrNoSession
	}
	s.mu.Lock()
	s.last = time.Now()
	s.mu.Unlock()
	return s, nil
}

func (m *Manager) spawn(s *session, start int) (*proc, error) {
	args := Args(Job{
		Input: s.src.Path, OutDir: s.dir, StartSegment: start, Encoder: s.plan.Encoder,
		VideoCodec: s.src.VideoCodec, SourceHeight: s.src.Height, MaxHeight: s.plan.MaxHeight,
	})
	p := &proc{start: start, done: make(chan struct{})}
	p.cmd = exec.Command(m.FFmpeg, args...)
	p.cmd.Stderr = &p.stderr
	if err := p.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	m.logger().Info("transcode started", "session", s.id, "segment", start,
		"encoder", s.plan.Encoder.Name, "device", s.plan.Encoder.DeviceName)
	go func() {
		p.err = p.cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// nextMissing is the first segment at or after start that doesn't exist yet.
func (s *session) nextMissing(start int) int {
	n := start
	for n < s.segs && exists(filepath.Join(s.dir, fmt.Sprintf("seg%d.ts", n))) {
		n++
	}
	return n
}

func (p *proc) running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (p *proc) stop() {
	if p.running() && p.cmd.Process != nil {
		p.killed.Store(true)
		_ = p.cmd.Process.Kill()
		<-p.done
	}
}

func (p *proc) stopped() bool { return p.killed.Load() }

func (m *Manager) logger() *slog.Logger {
	if m.Log != nil {
		return m.Log
	}
	return slog.Default()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstLine(stderr string, err error) string {
	for _, l := range strings.Split(stderr, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return err.Error()
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
