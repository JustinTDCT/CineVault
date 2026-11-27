// Package mediainfo reads a media file's technical details with ffprobe.
package mediainfo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/probe"
)

// Info is the technical description of one file, stored on its version.
type Info struct {
	Container       string  `json:"container"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	HDR             string  `json:"hdr,omitempty"` // hdr10, hlg, dolby_vision
	AudioCodec      string  `json:"audio_codec,omitempty"`
	AudioChannels   int     `json:"audio_channels,omitempty"`
	AudioProfile    string  `json:"audio_profile,omitempty"` // e.g. "DTS-HD MA", "Dolby TrueHD + Dolby Atmos"
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Bitrate         int64   `json:"bitrate,omitempty"`
}

// Prober reads a file's Info.
type Prober interface {
	Probe(ctx context.Context, path string) (Info, error)
}

// FFprobe reads files with the ffprobe binary.
type FFprobe struct {
	Path string // ffprobe binary, default "ffprobe"
	Run  probe.Runner
}

// Probe runs ffprobe on path.
func (f FFprobe) Probe(ctx context.Context, path string) (Info, error) {
	bin := f.Path
	if bin == "" {
		bin = "ffprobe"
	}
	run := f.Run
	if run == nil {
		run = probe.ExecRunner
	}
	out, stderr, err := run(ctx, bin, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path)
	if err != nil {
		msg := strings.TrimSpace(string(stderr))
		if msg == "" {
			msg = err.Error()
		}
		return Info{}, fmt.Errorf("ffprobe: %s", msg)
	}
	info, err := Parse(out)
	if err != nil {
		return Info{}, err
	}
	info.Container = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	return info, nil
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType     string `json:"codec_type"`
		CodecName     string `json:"codec_name"`
		Profile       string `json:"profile"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		Channels      int    `json:"channels"`
		ColorTransfer string `json:"color_transfer"`
		SideData      []struct {
			Type string `json:"side_data_type"`
		} `json:"side_data_list"`
		Disposition struct {
			Default     int `json:"default"`
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

// Parse reads ffprobe's JSON output (-print_format json -show_format -show_streams).
func Parse(data []byte) (Info, error) {
	var out ffprobeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return Info{}, fmt.Errorf("ffprobe output: %w", err)
	}
	var info Info
	info.DurationSeconds, _ = strconv.ParseFloat(out.Format.Duration, 64)
	info.DurationSeconds = math.Round(info.DurationSeconds*1000) / 1000
	info.Bitrate, _ = strconv.ParseInt(out.Format.BitRate, 10, 64)

	videoDone, audioIdx := false, -1
	for i, s := range out.Streams {
		switch s.CodecType {
		case "video":
			if videoDone || s.Disposition.AttachedPic == 1 {
				continue // cover art is a "video" stream too
			}
			videoDone = true
			info.VideoCodec, info.Width, info.Height = s.CodecName, s.Width, s.Height
			info.HDR = hdrKind(s.ColorTransfer)
			for _, sd := range s.SideData {
				if strings.Contains(sd.Type, "DOVI") {
					info.HDR = "dolby_vision"
				}
			}
		case "audio":
			// The default audio track, or the first one.
			if audioIdx == -1 || (s.Disposition.Default == 1 && out.Streams[audioIdx].Disposition.Default != 1) {
				audioIdx = i
			}
		}
	}
	if audioIdx >= 0 {
		a := out.Streams[audioIdx]
		info.AudioCodec, info.AudioChannels, info.AudioProfile = a.CodecName, a.Channels, a.Profile
	}
	if !videoDone && audioIdx < 0 {
		return info, fmt.Errorf("no audio or video streams")
	}
	return info, nil
}

func hdrKind(transfer string) string {
	switch transfer {
	case "smpte2084":
		return "hdr10"
	case "arib-std-b67":
		return "hlg"
	}
	return ""
}

// Resolution returns "4K", "1080p", "720p", "SD", or "" for audio-only files.
// Width counts as much as height, so a 1920×800 scope film is still 1080p.
func (i Info) Resolution() string {
	switch {
	case i.Width == 0 && i.Height == 0:
		return ""
	case i.Width >= 3200 || i.Height >= 1800:
		return "4K"
	case i.Width >= 1800 || i.Height >= 1000:
		return "1080p"
	case i.Width >= 1200 || i.Height >= 700:
		return "720p"
	}
	return "SD"
}

// Label builds a short description such as "4K · HDR10 · HEVC · TrueHD 7.1 Atmos".
func (i Info) Label() string {
	var parts []string
	if r := i.Resolution(); r != "" {
		parts = append(parts, r)
	}
	switch i.HDR {
	case "hdr10":
		parts = append(parts, "HDR10")
	case "hlg":
		parts = append(parts, "HLG")
	case "dolby_vision":
		parts = append(parts, "Dolby Vision")
	}
	if v := videoName(i.VideoCodec); v != "" {
		parts = append(parts, v)
	}
	if a := i.audioLabel(); a != "" {
		parts = append(parts, a)
	}
	return strings.Join(parts, " · ")
}

func videoName(codec string) string {
	switch codec {
	case "":
		return ""
	case "hevc":
		return "HEVC"
	case "h264":
		return "H.264"
	case "av1":
		return "AV1"
	case "vp9":
		return "VP9"
	case "mpeg2video":
		return "MPEG-2"
	}
	return strings.ToUpper(codec)
}

func (i Info) audioLabel() string {
	if i.AudioCodec == "" {
		return ""
	}
	name := strings.ToUpper(i.AudioCodec)
	switch i.AudioCodec {
	case "truehd":
		name = "TrueHD"
	case "dts":
		name = "DTS"
		if strings.Contains(i.AudioProfile, "DTS-HD MA") {
			name = "DTS-HD MA"
		} else if strings.Contains(i.AudioProfile, "DTS:X") {
			name = "DTS:X"
		}
	case "eac3":
		name = "E-AC-3"
	case "ac3":
		name = "AC-3"
	case "aac":
		name = "AAC"
	case "flac":
		name = "FLAC"
	case "opus":
		name = "Opus"
	}
	switch i.AudioChannels {
	case 1:
		name += " 1.0"
	case 2:
		name += " 2.0"
	case 6:
		name += " 5.1"
	case 8:
		name += " 7.1"
	}
	if strings.Contains(i.AudioProfile, "Atmos") {
		name += " Atmos"
	}
	return name
}
