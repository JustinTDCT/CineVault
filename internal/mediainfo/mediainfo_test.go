package mediainfo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A 4K Dolby Vision file with TrueHD Atmos, a commentary track and cover art.
const uhdJSON = `{
  "streams": [
    {"codec_type": "video", "codec_name": "hevc", "width": 3840, "height": 1600,
     "color_transfer": "smpte2084",
     "side_data_list": [{"side_data_type": "DOVI configuration record"}],
     "disposition": {"default": 1, "attached_pic": 0}},
    {"codec_type": "audio", "codec_name": "ac3", "channels": 2, "disposition": {"default": 0}},
    {"codec_type": "audio", "codec_name": "truehd", "channels": 8, "profile": "Dolby TrueHD + Dolby Atmos",
     "disposition": {"default": 1}},
    {"codec_type": "video", "codec_name": "mjpeg", "width": 600, "height": 900, "disposition": {"attached_pic": 1}}
  ],
  "format": {"duration": "9240.0451", "bit_rate": "58000000"}
}`

// A 1080p scope film (1920×800) with DTS-HD MA.
const scopeJSON = `{
  "streams": [
    {"codec_type": "video", "codec_name": "h264", "width": 1920, "height": 800, "color_transfer": "bt709"},
    {"codec_type": "audio", "codec_name": "dts", "channels": 6, "profile": "DTS-HD MA"}
  ],
  "format": {"duration": "7020.032", "bit_rate": "16000000"}
}`

func TestParseUHD(t *testing.T) {
	i, err := Parse([]byte(uhdJSON))
	if err != nil {
		t.Fatal(err)
	}
	if i.VideoCodec != "hevc" || i.Width != 3840 || i.HDR != "dolby_vision" || i.AudioCodec != "truehd" || i.AudioChannels != 8 {
		t.Errorf("got %+v", i)
	}
	if i.DurationSeconds != 9240.045 || i.Bitrate != 58000000 {
		t.Errorf("duration=%v bitrate=%v", i.DurationSeconds, i.Bitrate)
	}
	if got, want := i.Label(), "4K · Dolby Vision · HEVC · TrueHD 7.1 Atmos"; got != want {
		t.Errorf("Label = %q, want %q", got, want)
	}
}

func TestParseScope1080p(t *testing.T) {
	i, err := Parse([]byte(scopeJSON))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := i.Label(), "1080p · H.264 · DTS-HD MA 5.1"; got != want {
		t.Errorf("Label = %q, want %q", got, want)
	}
}

func TestResolution(t *testing.T) {
	cases := map[[2]int]string{
		{3840, 2160}: "4K", {3840, 1608}: "4K", {1920, 1080}: "1080p", {1920, 800}: "1080p",
		{1280, 720}: "720p", {1280, 536}: "720p", {720, 480}: "SD", {0, 0}: "",
	}
	for wh, want := range cases {
		if got := (Info{Width: wh[0], Height: wh[1]}).Resolution(); got != want {
			t.Errorf("%dx%d = %q, want %q", wh[0], wh[1], got, want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte("not json")); err == nil {
		t.Error("expected error for bad JSON")
	}
	if _, err := Parse([]byte(`{"streams": [{"codec_type": "subtitle"}], "format": {}}`)); err == nil {
		t.Error("expected error for a file with no audio or video")
	}
}

func TestFFprobeRunsAndSetsContainer(t *testing.T) {
	var gotArgs []string
	f := FFprobe{Run: func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		gotArgs = append([]string{name}, args...)
		return []byte(scopeJSON), nil, nil
	}}
	i, err := f.Probe(context.Background(), "/m/Alien (1979).MKV")
	if err != nil {
		t.Fatal(err)
	}
	if i.Container != "mkv" || gotArgs[0] != "ffprobe" || gotArgs[len(gotArgs)-1] != "/m/Alien (1979).MKV" {
		t.Errorf("container=%q args=%v", i.Container, gotArgs)
	}

	bad := FFprobe{Run: func(context.Context, string, ...string) ([]byte, []byte, error) {
		return nil, []byte("/m/x.mkv: Invalid data found when processing input\n"), errors.New("exit status 1")
	}}
	if _, err := bad.Probe(context.Background(), "/m/x.mkv"); err == nil || !strings.Contains(err.Error(), "Invalid data") {
		t.Errorf("err = %v", err)
	}
}
