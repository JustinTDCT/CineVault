package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/probe"
	"github.com/JustinTDCT/CineVault/internal/transcode"
)

type fakeVersions map[int64]catalog.Version

func (f fakeVersions) Version(_ context.Context, id int64) (catalog.Version, error) {
	v, ok := f[id]
	if !ok {
		return v, catalog.ErrNotFound
	}
	return v, nil
}

type fakeStreamer struct {
	started []transcode.Source
	stopped []string
	segPath string
	busy    bool
}

func (f *fakeStreamer) Start(src transcode.Source, _ transcode.Plan) (string, error) {
	if f.busy {
		return "", transcode.ErrBusy
	}
	f.started = append(f.started, src)
	return "sess1", nil
}
func (f *fakeStreamer) Playlist(id string) (string, error) {
	if id != "sess1" {
		return "", transcode.ErrNoSession
	}
	return transcode.Playlist(12), nil
}
func (f *fakeStreamer) Segment(_ context.Context, id string, n int) (string, error) {
	if id != "sess1" {
		return "", transcode.ErrNoSession
	}
	if n > 1 {
		return "", transcode.ErrSegment
	}
	return f.segPath, nil
}
func (f *fakeStreamer) Stop(id string) { f.stopped = append(f.stopped, id) }

var browserCaps = `{"containers":["mp4","webm"],"video":["h264","vp9"],"audio":["aac","opus"]}`

func playServer(t *testing.T, encoderOK bool) (*Server, *fakeStreamer) {
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "Movie (2001).mp4")
	os.WriteFile(mp4, []byte("0123456789abcdef"), 0o644)
	seg := filepath.Join(dir, "seg0.ts")
	os.WriteFile(seg, []byte("TS-DATA"), 0o644)
	st := &fakeStreamer{segPath: seg}
	versions := fakeVersions{
		1: {ID: 1, Path: mp4, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Height: 1080, DurationSeconds: 12},
		2: {ID: 2, Path: "/m/Alien {edition-Director's Cut} (1979).mkv", Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac", Height: 816, DurationSeconds: 6997.8},
		3: {ID: 3, Path: "/gone.mkv", Container: "mkv", Missing: true},
	}
	enc := probe.Encoder{Name: "h264_nvenc", Codec: "h264", Kind: probe.KindNVENC, GPU: 0, DeviceName: "NVIDIA A2", Usable: true}
	return New(Options{
		Playback: &Playback{Versions: versions, Streamer: st, Encoder: enc, EncoderOK: encoderOK, MaxHeight: 1080},
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}), st
}

func play(t *testing.T, s *Server, body string) (int, playResponse, string) {
	t.Helper()
	code, raw := send(t, s, "POST", "/api/playback", body)
	var r playResponse
	_ = json.Unmarshal([]byte(raw), &r)
	return code, r, raw
}

func TestDirectPlay(t *testing.T) {
	s, st := playServer(t, true)
	code, r, raw := play(t, s, `{"version_id":1,"caps":`+browserCaps+`}`)
	if code != http.StatusOK || r.Mode != "direct" || r.URL != "/api/versions/1/file" || len(st.started) != 0 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", r.URL, nil)
	req.Header.Set("Range", "bytes=4-7")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "4567" || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Errorf("range: code=%d body=%q type=%q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
}

func TestHLSPlayback(t *testing.T) {
	s, st := playServer(t, true)
	code, r, raw := play(t, s, `{"version_id":2,"caps":`+browserCaps+`}`)
	if code != http.StatusOK || r.Mode != "hls" || r.URL != "/api/playback/sess1/index.m3u8" || r.Device != "NVIDIA A2" || len(r.Reasons) != 2 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	if len(st.started) != 1 || st.started[0].VideoCodec != "hevc" || st.started[0].DurationSeconds != 6997.8 {
		t.Errorf("started = %+v", st.started)
	}

	code, pl := send(t, s, "GET", r.URL, "")
	if code != http.StatusOK || !strings.Contains(pl, "seg1.ts") {
		t.Errorf("playlist: code=%d %s", code, pl)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/playback/sess1/seg0.ts", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "TS-DATA" || rec.Header().Get("Content-Type") != "video/mp2t" {
		t.Errorf("segment: code=%d body=%q type=%q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	for target, want := range map[string]int{
		"/api/playback/sess1/seg9.ts":    http.StatusNotFound,
		"/api/playback/other/seg0.ts":    http.StatusNotFound,
		"/api/playback/other/index.m3u8": http.StatusNotFound,
		"/api/playback/sess1/evil.txt":   http.StatusNotFound,
	} {
		if code, _ := send(t, s, "GET", target, ""); code != want {
			t.Errorf("%s: %d, want %d", target, code, want)
		}
	}
	if code, _ := send(t, s, "DELETE", "/api/playback/sess1", ""); code != http.StatusNoContent || len(st.stopped) != 1 {
		t.Errorf("stop: code=%d stopped=%v", code, st.stopped)
	}
}

func TestPlaybackErrors(t *testing.T) {
	s, st := playServer(t, true)
	cases := map[string]struct {
		code int
		has  string
	}{
		`{"version_id":3,"caps":` + browserCaps + `}`:  {http.StatusConflict, "missing from disk"},
		`{"version_id":99,"caps":` + browserCaps + `}`: {http.StatusNotFound, "not found"},
		`{"caps":{}}`: {http.StatusBadRequest, "version_id"},
		`nope`:        {http.StatusBadRequest, "version_id"},
	}
	for body, want := range cases {
		if code, _, raw := play(t, s, body); code != want.code || !strings.Contains(raw, want.has) {
			t.Errorf("%s: code=%d body=%s", body, code, raw)
		}
	}
	st.busy = true
	if code, _, raw := play(t, s, `{"version_id":2,"caps":`+browserCaps+`}`); code != http.StatusServiceUnavailable || !strings.Contains(raw, "too many") {
		t.Errorf("busy: code=%d body=%s", code, raw)
	}

	noEnc, _ := playServer(t, false)
	if code, _, raw := play(t, noEnc, `{"version_id":2,"caps":`+browserCaps+`}`); code != http.StatusServiceUnavailable || !strings.Contains(raw, "no working video encoder") {
		t.Errorf("no encoder: code=%d body=%s", code, raw)
	}
	// Direct play still works without an encoder.
	if code, r, _ := play(t, noEnc, `{"version_id":1,"caps":`+browserCaps+`}`); code != http.StatusOK || r.Mode != "direct" {
		t.Errorf("direct without encoder: code=%d", code)
	}
}
