package transcode

import (
	"slices"
	"strings"
	"testing"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/probe"
)

var (
	a2     = probe.Encoder{Name: "h264_nvenc", Codec: "h264", Kind: probe.KindNVENC, GPU: 0, DeviceName: "NVIDIA A2", Usable: true}
	t4     = probe.Encoder{Name: "h264_nvenc", Codec: "h264", Kind: probe.KindNVENC, GPU: 1, DeviceName: "NVIDIA T4", Usable: true}
	x264   = probe.Encoder{Name: "libx264", Codec: "h264", Kind: probe.KindCPU, GPU: -1, DeviceName: "CPU", Usable: true}
	hevcA2 = probe.Encoder{Name: "hevc_nvenc", Codec: "hevc", Kind: probe.KindNVENC, GPU: 0, Usable: true}
)

func TestSelectH264(t *testing.T) {
	all := []probe.Encoder{hevcA2, a2, t4, x264}
	cases := []struct {
		name    string
		encs    []probe.Encoder
		pref    Preference
		want    probe.Encoder
		ok      bool
		noteHas string
	}{
		{"auto prefers the first GPU", all, Preference{"auto", -1}, a2, true, ""},
		{"auto honours a chosen GPU", all, Preference{"auto", 1}, t4, true, ""},
		{"nvenc on a chosen GPU", all, Preference{"nvenc", 1}, t4, true, ""},
		{"cpu when asked", all, Preference{"cpu", -1}, x264, true, ""},
		{"auto falls back to CPU", []probe.Encoder{x264}, Preference{"auto", -1}, x264, true, ""},
		{"nvenc missing falls back with a note", []probe.Encoder{x264}, Preference{"nvenc", -1}, x264, true, "no working NVIDIA GPU"},
		{"missing GPU number", all, Preference{"nvenc", 5}, x264, true, "GPU 5"},
		{"nothing works", []probe.Encoder{{Name: "h264_nvenc", Codec: "h264", Kind: probe.KindNVENC}}, Preference{"auto", -1}, probe.Encoder{}, false, "no working"},
	}
	for _, c := range cases {
		got, note, ok := SelectH264(c.encs, c.pref)
		if got != c.want || ok != c.ok || (c.noteHas != "" && !strings.Contains(note, c.noteHas)) {
			t.Errorf("%s: got %s gpu%d ok=%v note=%q", c.name, got.Name, got.GPU, ok, note)
		}
	}
}

func TestParsePreference(t *testing.T) {
	if p, err := ParsePreference("", ""); err != nil || p.Mode != "auto" || p.GPU != -1 {
		t.Errorf("defaults: %+v %v", p, err)
	}
	if p, err := ParsePreference("NVENC", "1"); err != nil || p.Mode != "nvenc" || p.GPU != 1 {
		t.Errorf("nvenc 1: %+v %v", p, err)
	}
	for _, bad := range [][2]string{{"qsv", ""}, {"auto", "x"}, {"auto", "-1"}} {
		if _, err := ParsePreference(bad[0], bad[1]); err == nil {
			t.Errorf("%v: expected error", bad)
		}
	}
}

var browser = ClientCaps{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "vp9", "av1"}, Audio: []string{"aac", "mp3", "opus"}}

func TestDecide(t *testing.T) {
	alienDC := catalog.Version{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac", Height: 816}
	p := Decide(alienDC, browser, a2, 1080)
	if p.Mode != "hls" || len(p.Reasons) != 2 || p.Encoder != a2 {
		t.Errorf("Alien DC: %+v", p)
	}
	mp4 := catalog.Version{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Height: 1080}
	if p := Decide(mp4, browser, a2, 1080); p.Mode != "direct" {
		t.Errorf("H.264 MP4 should play directly: %+v", p)
	}
	if p := Decide(mp4, browser, a2, 720); p.Mode != "hls" || !strings.Contains(p.Reasons[0], "720p") {
		t.Errorf("height limit should convert: %+v", p)
	}
	truehd := catalog.Version{Container: "mp4", VideoCodec: "h264", AudioCodec: "truehd", Height: 1080}
	if p := Decide(truehd, browser, a2, 0); p.Mode != "hls" || !strings.Contains(p.Reasons[0], "TrueHD") {
		t.Errorf("TrueHD audio should convert: %+v", p)
	}
}

func TestPlaylist(t *testing.T) {
	pl := Playlist(6997.824) // Alien Director's Cut
	if SegmentCount(6997.824) != 1167 || !strings.HasSuffix(pl, "seg1166.ts\n#EXT-X-ENDLIST\n") {
		t.Errorf("segments: %d", SegmentCount(6997.824))
	}
	if !strings.Contains(pl, "#EXTINF:6.000,\nseg0.ts\n") || !strings.Contains(pl, "#EXTINF:1.824,\nseg1166.ts") {
		t.Errorf("durations wrong:\n%s", pl[:200])
	}
	if !strings.Contains(pl, "#EXT-X-PLAYLIST-TYPE:VOD") {
		t.Error("must be a VOD playlist")
	}
}

func has(args []string, seq ...string) bool {
	for i := range args {
		if i+len(seq) <= len(args) && slices.Equal(args[i:i+len(seq)], seq) {
			return true
		}
	}
	return false
}

func TestArgsNVENCFullGPU(t *testing.T) {
	a := Args(Job{Input: "/m/Alien.mkv", OutDir: "/tmp/s", StartSegment: 10, Encoder: a2, VideoCodec: "hevc", SourceHeight: 816, MaxHeight: 1080})
	for _, seq := range [][]string{
		{"-ss", "60"}, {"-hwaccel", "cuda", "-hwaccel_output_format", "cuda", "-hwaccel_device", "0"},
		{"-vf", "scale_cuda=format=yuv420p"}, {"-c:v", "h264_nvenc", "-gpu", "0"},
		{"-force_key_frames", "expr:gte(t,n_forced*6)"}, {"-start_number", "10"}, {"-output_ts_offset", "60"},
		{"-hls_segment_filename", "/tmp/s/seg%d.ts"},
	} {
		if !has(a, seq...) {
			t.Errorf("missing %v in %v", seq, a)
		}
	}
	// -ss must come before -i for fast seeking.
	if slices.Index(a, "-ss") > slices.Index(a, "-i") {
		t.Error("-ss must precede -i")
	}
}

func TestArgsNVENCScalesOnGPU(t *testing.T) {
	a := Args(Job{Input: "x", OutDir: "o", Encoder: a2, VideoCodec: "hevc", SourceHeight: 2160, MaxHeight: 1080})
	if !has(a, "-vf", "scale_cuda=w=-2:h=1080:format=yuv420p") || has(a, "-ss", "0") {
		t.Errorf("args = %v", a)
	}
}

func TestArgsNVENCSoftwareDecodeForOldCodecs(t *testing.T) {
	a := Args(Job{Input: "x", OutDir: "o", Encoder: a2, VideoCodec: "mpeg4", SourceHeight: 480, MaxHeight: 1080})
	if slices.Contains(a, "-hwaccel") || !has(a, "-vf", "format=yuv420p") || !has(a, "-c:v", "h264_nvenc", "-gpu", "0") {
		t.Errorf("args = %v", a)
	}
}

func TestArgsCPU(t *testing.T) {
	a := Args(Job{Input: "x", OutDir: "o", Encoder: x264, VideoCodec: "hevc", SourceHeight: 2160, MaxHeight: 1080})
	if slices.Contains(a, "-hwaccel") || !has(a, "-vf", "scale=-2:1080,format=yuv420p") || !has(a, "-c:v", "libx264") || !has(a, "-sc_threshold", "0") {
		t.Errorf("args = %v", a)
	}
}
