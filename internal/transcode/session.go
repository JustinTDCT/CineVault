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

	// Throttling: FFmpeg pauses when it's PauseAhead past the last segment the
	// player asked for, and resumes when the lead falls to ResumeAhead.
	// Segments more than KeepBehind before that point are deleted. Zero values
	// mean 3 minutes, 90 seconds and 3 minutes.
	PauseAhead  time.Duration
	ResumeAhead time.Duration
	KeepBehind  time.Duration

	Log *slog.Logger

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
	// lastReq is the highest segment the player asked for since the last seek.
	lastReq int
	// cleaned is the first segment not yet considered for deletion.
	cleaned int
}

// proc is one running FFmpeg. err and stderr are only read after done closes.
type proc struct {
	cmd    *exec.Cmd
	start  int // first segment it produces
	done   chan struct{}
	err    error
	stderr bytes.Buffer
	killed atomic.Bool // stopped on purpose (seek, stop); not a failure
	paused bool        // suspended by the throttle; guarded by the session lock
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
	m.sessions[id] = &session{id: id, src: src, plan: plan, dir: dir, segs: SegmentCount(src.DurationSeconds), last: time.Now(), lastReq: -1}
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
	s.mu.Lock()
	s.noteRequest(n)
	if s.proc != nil && s.proc.paused && s.lead(m.segAhead()) < m.resumeSegs() {
		s.proc.resume()
	}
	s.mu.Unlock()
	if exists(path) {
		return path, nil
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", ErrNoSession
	}
	p := s.proc
	if p == nil || !p.running() || n < p.start || n > s.nextMissing(max(p.start, s.cleaned))+lookahead {
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

// RunReaper throttles sessions every 2 seconds and stops idle ones every 30
// seconds, until ctx ends; then it stops all sessions.
func (m *Manager) RunReaper(ctx context.Context) {
	throttle := time.NewTicker(2 * time.Second)
	defer throttle.Stop()
	reap := time.NewTicker(30 * time.Second)
	defer reap.Stop()
	for {
		select {
		case <-ctx.Done():
			m.StopAll()
			return
		case <-throttle.C:
			m.Throttle()
		case <-reap.C:
			m.ReapIdle()
		}
	}
}

// Throttle pauses FFmpeg for sessions far enough ahead of the player, resumes
// those that have fallen behind, and deletes segments the player has passed.
func (m *Manager) Throttle() {
	m.mu.Lock()
	list := make([]*session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	m.mu.Unlock()

	for _, s := range list {
		s.mu.Lock()
		if p := s.proc; p != nil && p.running() && s.lastReq >= 0 {
			lead := s.lead(m.segAhead())
			switch {
			case !p.paused && lead >= m.pauseSegs():
				if p.pause() {
					m.logger().Debug("transcode paused", "session", s.id, "lead_segments", lead)
				}
			case p.paused && lead <= m.resumeSegs():
				p.resume()
				m.logger().Debug("transcode resumed", "session", s.id, "lead_segments", lead)
			}
		}
		s.cleanup(s.lastReq - m.keepSegs())
		s.mu.Unlock()
	}
}

// noteRequest records the player's position; a jump backwards is a seek.
func (s *session) noteRequest(n int) {
	if n > s.lastReq || n < s.lastReq-lookahead {
		s.lastReq = n
	}
}

// lead is how many finished segments exist past the player's last request,
// counting at most limit+1 (enough to compare with the thresholds).
func (s *session) lead(limit int) int {
	n := 0
	for i := s.lastReq + 1; i < s.segs && n <= limit; i++ {
		if !exists(filepath.Join(s.dir, fmt.Sprintf("seg%d.ts", i))) {
			break
		}
		n++
	}
	return n
}

// cleanup deletes segments before index before.
func (s *session) cleanup(before int) {
	for ; s.cleaned < before; s.cleaned++ {
		_ = os.Remove(filepath.Join(s.dir, fmt.Sprintf("seg%d.ts", s.cleaned)))
	}
	// A seek backwards re-creates earlier segments; allow them to be cleaned again.
	if before < s.cleaned {
		s.cleaned = max(before, 0)
	}
}

func segs(d, def time.Duration) int {
	if d == 0 {
		d = def
	}
	return int(d / (SegmentSeconds * time.Second))
}

func (m *Manager) pauseSegs() int  { return segs(m.PauseAhead, 3*time.Minute) }
func (m *Manager) resumeSegs() int { return segs(m.ResumeAhead, 90*time.Second) }
func (m *Manager) keepSegs() int   { return segs(m.KeepBehind, 3*time.Minute) }
func (m *Manager) segAhead() int   { return m.pauseSegs() }

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
		_ = p.cmd.Process.Kill() // SIGKILL also ends a paused process
		<-p.done
	}
}

func (p *proc) pause() bool {
	if p.paused || !p.running() || !suspend(p.cmd.Process) {
		return false
	}
	p.paused = true
	return true
}

func (p *proc) resume() {
	if p.paused {
		resumeProcess(p.cmd.Process)
		p.paused = false
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
