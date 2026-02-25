package scanner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type ProbeResult struct {
	VideoCodec string
	AudioCodec string
	Resolution string
	Width      int
	Height     int
	Duration   float64
	Bitrate    int
	AudioRate  int
	Channels   int
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	SampleRate string `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
	BitRate  string `json:"bit_rate"`
}

func Probe(ffprobePath, filePath string) (*ProbeResult, error) {
	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		filePath)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	var data ffprobeOutput
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("parse ffprobe: %w", err)
	}

	result := &ProbeResult{}

	for _, s := range data.Streams {
		switch s.CodecType {
		case "video":
			if result.VideoCodec == "" {
				result.VideoCodec = s.CodecName
				result.Width = s.Width
				result.Height = s.Height
				result.Resolution = formatResolution(s.Height)
			}
		case "audio":
			if result.AudioCodec == "" {
				result.AudioCodec = s.CodecName
				result.Channels = s.Channels
				if rate, err := strconv.Atoi(s.SampleRate); err == nil {
					result.AudioRate = rate
				}
			}
		}
	}

	if data.Format.Duration != "" {
		result.Duration, _ = strconv.ParseFloat(data.Format.Duration, 64)
	}
	if data.Format.BitRate != "" {
		result.Bitrate, _ = strconv.Atoi(data.Format.BitRate)
	}

	return result, nil
}

func formatResolution(height int) string {
	switch {
	case height >= 2160:
		return "4K"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 480:
		return "480p"
	default:
		return strings.ToLower(fmt.Sprintf("%dp", height))
	}
}
