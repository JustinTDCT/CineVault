package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type FFprobe struct{ Path string }
type ProbeResult struct {
	Format  FormatInfo   `json:"format"`
	Streams []StreamInfo `json:"streams"`
}
type FormatInfo struct {
	Filename string `json:"filename"`
	Duration string `json:"duration"`
	Size     string `json:"size"`
	Bitrate  string `json:"bit_rate"`
}
type StreamInfo struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

func NewFFprobe(path string) *FFprobe { return &FFprobe{Path: path} }

func (f *FFprobe) Probe(filePath string) (*ProbeResult, error) {
	cmd := exec.Command(f.Path, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}
	var result ProbeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}
	return &result, nil
}

func (r *ProbeResult) GetDurationSeconds() int {
	duration, _ := strconv.ParseFloat(r.Format.Duration, 64)
	return int(duration)
}

func (r *ProbeResult) GetResolution() string {
	for _, s := range r.Streams {
		if s.CodecType == "video" {
			// Use both width and height for accurate resolution classification
			// Some content is slightly letterboxed (e.g. 1920x1036 is still 1080p)
			if s.Height >= 2160 || s.Width >= 3840 { return "4K" }
			if s.Height >= 900 || s.Width >= 1800 { return "1080p" }
			if s.Height >= 600 || s.Width >= 1200 { return "720p" }
			if s.Height >= 400 { return "480p" }
			return "SD"
		}
	}
	return ""
}

func (r *ProbeResult) GetVideoCodec() string {
	for _, s := range r.Streams {
		if s.CodecType == "video" {
			return s.CodecName
		}
	}
	return ""
}

func (r *ProbeResult) GetAudioCodec() string {
	for _, s := range r.Streams {
		if s.CodecType == "audio" {
			return s.CodecName
		}
	}
	return ""
}

func (r *ProbeResult) GetWidth() int {
	for _, s := range r.Streams {
		if s.CodecType == "video" {
			return s.Width
		}
	}
	return 0
}

func (r *ProbeResult) GetHeight() int {
	for _, s := range r.Streams {
		if s.CodecType == "video" {
			return s.Height
		}
	}
	return 0
}

func (r *ProbeResult) GetFileSize() int64 {
	size, _ := strconv.ParseInt(r.Format.Size, 10, 64)
	return size
}

func (r *ProbeResult) GetBitrate() int64 {
	br, _ := strconv.ParseInt(r.Format.Bitrate, 10, 64)
	return br
}
