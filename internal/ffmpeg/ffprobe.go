package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type FFprobe struct{ Path string }
type ProbeResult struct {
	Format   FormatInfo   `json:"format"`
	Streams  []StreamInfo `json:"streams"`
	Chapters []ChapterInfo `json:"chapters"`
}
type FormatInfo struct {
	Filename string `json:"filename"`
	Duration string `json:"duration"`
	Size     string `json:"size"`
	Bitrate  string `json:"bit_rate"`
}
type StreamInfo struct {
	Index          int            `json:"index"`
	CodecType      string         `json:"codec_type"`
	CodecName      string         `json:"codec_name"`
	CodecLongName  string         `json:"codec_long_name"`
	Width          int            `json:"width"`
	Height         int            `json:"height"`
	Channels       int            `json:"channels"`
	ChannelLayout  string         `json:"channel_layout"`
	SampleRate     string         `json:"sample_rate"`
	BitRate        string         `json:"bit_rate"`
	ColorTransfer  string         `json:"color_transfer"`
	ColorPrimaries string         `json:"color_primaries"`
	ColorSpace     string         `json:"color_space"`
	PixFmt         string         `json:"pix_fmt"`
	Profile        string         `json:"profile"`
	SideDataList   []SideDataItem `json:"side_data_list"`
	Tags           map[string]string `json:"tags"`
	Disposition    Disposition    `json:"disposition"`
}

// SideDataItem represents a side_data entry from ffprobe (used for Dolby Vision RPU detection).
type SideDataItem struct {
	SideDataType string `json:"side_data_type"`
}

// Disposition flags from ffprobe stream disposition.
type Disposition struct {
	Default    int `json:"default"`
	Forced     int `json:"forced"`
	Comment    int `json:"comment"`
	HearingImpaired int `json:"hearing_impaired"`
}

// ChapterInfo represents a chapter entry from ffprobe.
type ChapterInfo struct {
	ID        int               `json:"id"`
	TimeBase  string            `json:"time_base"`
	Start     int64             `json:"start"`
	StartTime string            `json:"start_time"`
	End       int64             `json:"end"`
	EndTime   string            `json:"end_time"`
	Tags      map[string]string `json:"tags"`
}

func NewFFprobe(path string) *FFprobe { return &FFprobe{Path: path} }

func (f *FFprobe) Probe(filePath string) (*ProbeResult, error) {
	cmd := exec.Command(f.Path, "-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", "-show_chapters", filePath)
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

// GetAudioFormat returns an enhanced audio format string that detects spatial audio
// features like Dolby Atmos and DTS:X on top of the base codec.
// Possible return values include: "TrueHD Atmos", "EAC3 Atmos", "DTS-HD MA DTS:X",
// "TrueHD", "DTS-HD MA", "FLAC", "AAC", etc.
func (r *ProbeResult) GetAudioFormat() string {
	for _, s := range r.Streams {
		if s.CodecType != "audio" {
			continue
		}
		codec := strings.ToUpper(s.CodecName)
		profile := strings.ToLower(s.Profile)

		// Normalize codec display names
		displayCodec := s.CodecName
		switch codec {
		case "TRUEHD":
			displayCodec = "TrueHD"
		case "EAC3":
			displayCodec = "EAC3"
		case "AC3":
			displayCodec = "AC3"
		case "DTS":
			// Check profile for DTS-HD variants
			if strings.Contains(profile, "dts-hd ma") || strings.Contains(profile, "ma") {
				displayCodec = "DTS-HD MA"
			} else if strings.Contains(profile, "dts-hd hra") || strings.Contains(profile, "hra") {
				displayCodec = "DTS-HD HRA"
			} else {
				displayCodec = "DTS"
			}
		case "FLAC":
			displayCodec = "FLAC"
		case "AAC":
			displayCodec = "AAC"
		case "OPUS":
			displayCodec = "Opus"
		case "VORBIS":
			displayCodec = "Vorbis"
		case "PCM_S16LE", "PCM_S24LE", "PCM_S32LE":
			displayCodec = "PCM"
		}

		// Detect Dolby Atmos: present as side_data (JOC) in TrueHD, or as profile in EAC3
		isAtmos := false
		for _, sd := range s.SideDataList {
			sdType := strings.ToLower(sd.SideDataType)
			if strings.Contains(sdType, "atmos") || strings.Contains(sdType, "joint object coding") {
				isAtmos = true
				break
			}
		}
		// EAC3 Atmos is indicated by the "atmos" profile or channel layout with objects
		if codec == "EAC3" && (strings.Contains(profile, "atmos") || s.Channels > 8) {
			isAtmos = true
		}
		// TrueHD with >8 channels is typically Atmos
		if codec == "TRUEHD" && s.Channels > 8 {
			isAtmos = true
		}
		if isAtmos {
			return displayCodec + " Atmos"
		}

		// Detect DTS:X: indicated by profile or side_data
		if codec == "DTS" {
			if strings.Contains(profile, "dts:x") || strings.Contains(profile, "dtsx") {
				return displayCodec + " DTS:X"
			}
			for _, sd := range s.SideDataList {
				if strings.Contains(strings.ToLower(sd.SideDataType), "dts:x") {
					return displayCodec + " DTS:X"
				}
			}
		}

		return displayCodec
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

// GetHDRFormat returns a detailed HDR format string based on color transfer, primaries,
// and side data. Returns empty string for SDR content.
// Possible values: "Dolby Vision", "HDR10+", "HDR10", "HLG", "PQ" (generic PQ without HDR10 primaries).
func (r *ProbeResult) GetHDRFormat() string {
	for _, s := range r.Streams {
		if s.CodecType != "video" {
			continue
		}
		// Check for Dolby Vision via side_data_list (RPU or configuration record)
		for _, sd := range s.SideDataList {
			if sd.SideDataType == "DOVI configuration record" || sd.SideDataType == "Dolby Vision RPU Data" {
				return "Dolby Vision"
			}
		}
		// Check color transfer for PQ (HDR10/HDR10+) or HLG
		switch s.ColorTransfer {
		case "smpte2084": // PQ transfer
			// HDR10 requires BT.2020 primaries
			if s.ColorPrimaries == "bt2020" {
				return "HDR10"
			}
			return "PQ"
		case "arib-std-b67": // HLG transfer
			return "HLG"
		}
		// Check for 10-bit pixel formats that might indicate HDR
		// (secondary heuristic — PQ/HLG above is authoritative)
	}
	return ""
}

// GetDynamicRange returns a simplified dynamic range classification.
// Returns "HDR" for any HDR format, "SDR" otherwise.
func (r *ProbeResult) GetDynamicRange() string {
	if r.GetHDRFormat() != "" {
		return "HDR"
	}
	return "SDR"
}

// ── Subtitle Track Extraction ──

// SubtitleTrack represents a detected embedded subtitle stream.
type SubtitleTrack struct {
	StreamIndex int
	CodecName   string
	Language    string
	Title       string
	IsDefault   bool
	IsForced    bool
	IsSDH       bool
}

// GetSubtitleTracks returns all subtitle streams found in the file.
func (r *ProbeResult) GetSubtitleTracks() []SubtitleTrack {
	var tracks []SubtitleTrack
	for _, s := range r.Streams {
		if s.CodecType != "subtitle" {
			continue
		}
		track := SubtitleTrack{
			StreamIndex: s.Index,
			CodecName:   s.CodecName,
			IsDefault:   s.Disposition.Default == 1,
			IsForced:    s.Disposition.Forced == 1,
			IsSDH:       s.Disposition.HearingImpaired == 1,
		}
		if lang, ok := s.Tags["language"]; ok {
			track.Language = lang
		}
		if title, ok := s.Tags["title"]; ok {
			track.Title = title
			// Detect SDH from title if not in disposition
			lower := strings.ToLower(title)
			if strings.Contains(lower, "sdh") || strings.Contains(lower, "hearing impaired") {
				track.IsSDH = true
			}
			// Detect forced from title
			if strings.Contains(lower, "forced") {
				track.IsForced = true
			}
		}
		tracks = append(tracks, track)
	}
	return tracks
}

// ── Audio Track Extraction ──

// AudioTrack represents a detected audio stream.
type AudioTrack struct {
	StreamIndex   int
	CodecName     string
	Channels      int
	ChannelLayout string
	SampleRate    int
	BitRate       int64
	Language      string
	Title         string
	IsDefault     bool
	IsCommentary  bool
}

// GetAudioTracks returns all audio streams found in the file.
func (r *ProbeResult) GetAudioTracks() []AudioTrack {
	var tracks []AudioTrack
	for _, s := range r.Streams {
		if s.CodecType != "audio" {
			continue
		}
		track := AudioTrack{
			StreamIndex:   s.Index,
			CodecName:     s.CodecName,
			Channels:      s.Channels,
			ChannelLayout: s.ChannelLayout,
			IsDefault:     s.Disposition.Default == 1,
			IsCommentary:  s.Disposition.Comment == 1,
		}
		if sr, err := strconv.Atoi(s.SampleRate); err == nil {
			track.SampleRate = sr
		}
		if br, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil {
			track.BitRate = br
		}
		if lang, ok := s.Tags["language"]; ok {
			track.Language = lang
		}
		if title, ok := s.Tags["title"]; ok {
			track.Title = title
			// Detect commentary from title
			lower := strings.ToLower(title)
			if strings.Contains(lower, "commentary") || strings.Contains(lower, "comment") {
				track.IsCommentary = true
			}
		}
		tracks = append(tracks, track)
	}
	return tracks
}

// ── Chapter Extraction ──

// Chapter represents a chapter marker from the container.
type Chapter struct {
	Title        string
	StartSeconds float64
	EndSeconds   float64
	SortOrder    int
}

// GetChapters returns all chapters found in the file.
func (r *ProbeResult) GetChapters() []Chapter {
	var chapters []Chapter
	for i, c := range r.Chapters {
		ch := Chapter{
			SortOrder: i,
		}
		if start, err := strconv.ParseFloat(c.StartTime, 64); err == nil {
			ch.StartSeconds = start
		}
		if end, err := strconv.ParseFloat(c.EndTime, 64); err == nil {
			ch.EndSeconds = end
		}
		if title, ok := c.Tags["title"]; ok {
			ch.Title = title
		}
		chapters = append(chapters, ch)
	}
	return chapters
}

func (r *ProbeResult) GetFileSize() int64 {
	size, _ := strconv.ParseInt(r.Format.Size, 10, 64)
	return size
}

func (r *ProbeResult) GetBitrate() int64 {
	br, _ := strconv.ParseInt(r.Format.Bitrate, 10, 64)
	return br
}
