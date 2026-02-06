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
			if s.Height >= 2160 { return "4K" } 
			if s.Height >= 1080 { return "1080p" }
			if s.Height >= 720 { return "720p" }
			return "SD"
		}
	}
	return ""
}
