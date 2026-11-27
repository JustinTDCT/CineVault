// Package transcode decides how a file reaches a player, builds FFmpeg
// commands for HLS streaming, and runs streaming sessions.
package transcode

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/catalog"
	"github.com/JustinTDCT/CineVault/internal/probe"
)

// SegmentSeconds is the length of every HLS segment. Keyframes are forced on
// these boundaries, so a restarted transcode produces identical segments.
const SegmentSeconds = 6

// Preference is the transcoding device the admin asked for.
type Preference struct {
	// Mode is "auto" (best working device), "nvenc" or "cpu".
	Mode string
	// GPU is the NVIDIA GPU index to use, or -1 for any.
	GPU int
}

// ParsePreference reads CINEVAULT_TRANSCODE and CINEVAULT_TRANSCODE_GPU values.
func ParsePreference(mode, gpu string) (Preference, error) {
	p := Preference{Mode: strings.ToLower(strings.TrimSpace(mode)), GPU: -1}
	if p.Mode == "" {
		p.Mode = "auto"
	}
	if p.Mode != "auto" && p.Mode != "nvenc" && p.Mode != "cpu" {
		return p, fmt.Errorf("CINEVAULT_TRANSCODE must be auto, nvenc or cpu, not %q", mode)
	}
	if strings.TrimSpace(gpu) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(gpu))
		if err != nil || n < 0 {
			return p, fmt.Errorf("CINEVAULT_TRANSCODE_GPU must be a GPU number like 0, not %q", gpu)
		}
		p.GPU = n
	}
	return p, nil
}

// SelectH264 picks the H.264 encoder to use from the probed encoders. Auto
// prefers a working NVIDIA GPU and falls back to the CPU. An explicit choice
// that doesn't work also falls back to the CPU, and note explains why.
func SelectH264(encs []probe.Encoder, pref Preference) (enc probe.Encoder, note string, ok bool) {
	var nvenc, cpu *probe.Encoder
	for i := range encs {
		e := &encs[i]
		if e.Codec != "h264" || !e.Usable {
			continue
		}
		switch e.Kind {
		case probe.KindNVENC:
			if nvenc == nil && (pref.GPU < 0 || e.GPU == pref.GPU) {
				nvenc = e
			}
		case probe.KindCPU:
			if cpu == nil {
				cpu = e
			}
		}
	}
	switch pref.Mode {
	case "cpu":
		if cpu != nil {
			return *cpu, "", true
		}
		return probe.Encoder{}, "CPU encoding (libx264) isn't available in this FFmpeg", false
	case "nvenc":
		if nvenc != nil {
			return *nvenc, "", true
		}
		note = "NVIDIA encoding was requested but no working NVIDIA GPU was found"
		if pref.GPU >= 0 {
			note = fmt.Sprintf("NVIDIA GPU %d was requested but it isn't working", pref.GPU)
		}
		if cpu != nil {
			return *cpu, note + "; using the CPU", true
		}
		return probe.Encoder{}, note, false
	default:
		if nvenc != nil {
			return *nvenc, "", true
		}
		if cpu != nil {
			return *cpu, "", true
		}
		return probe.Encoder{}, "no working H.264 encoder was found", false
	}
}

// ClientCaps is what the player says it can play natively.
type ClientCaps struct {
	Containers []string `json:"containers"` // mp4, webm
	Video      []string `json:"video"`      // h264, hevc, av1, vp9
	Audio      []string `json:"audio"`      // aac, mp3, opus, ac3, eac3, flac
}

// Plan is how one file will be played.
type Plan struct {
	// Mode is "direct" (send the file as-is) or "hls" (stream it converted).
	Mode    string   `json:"mode"`
	Reasons []string `json:"reasons"`
	// Fields below apply to "hls".
	MaxHeight int           `json:"max_height,omitempty"`
	Encoder   probe.Encoder `json:"encoder"`
}

// directContainers maps file extensions to the container name browsers use.
var directContainers = map[string]string{"mp4": "mp4", "m4v": "mp4", "mov": "mp4", "webm": "webm"}

// Decide picks direct play when the player handles the file's container and
// codecs, and HLS conversion to H.264 + AAC otherwise.
func Decide(v catalog.Version, caps ClientCaps, enc probe.Encoder, maxHeight int) Plan {
	var reasons []string
	container, ok := directContainers[v.Container]
	if !ok || !slices.Contains(caps.Containers, container) {
		reasons = append(reasons, fmt.Sprintf("the %s container isn't supported by this player", strings.ToUpper(v.Container)))
	}
	if v.VideoCodec != "" && !slices.Contains(caps.Video, v.VideoCodec) {
		reasons = append(reasons, fmt.Sprintf("%s video isn't supported by this player", codecName(v.VideoCodec)))
	}
	if v.AudioCodec != "" && !slices.Contains(caps.Audio, v.AudioCodec) {
		reasons = append(reasons, fmt.Sprintf("%s audio isn't supported by this player", codecName(v.AudioCodec)))
	}
	if maxHeight > 0 && v.Height > maxHeight {
		reasons = append(reasons, fmt.Sprintf("the picture is taller than the %dp limit", maxHeight))
	}
	if len(reasons) == 0 {
		return Plan{Mode: "direct", Reasons: []string{}}
	}
	return Plan{Mode: "hls", Reasons: reasons, MaxHeight: maxHeight, Encoder: enc}
}

func codecName(c string) string {
	switch c {
	case "h264":
		return "H.264"
	case "hevc":
		return "HEVC"
	case "av1":
		return "AV1"
	case "eac3":
		return "E-AC-3"
	case "ac3":
		return "AC-3"
	case "truehd":
		return "TrueHD"
	case "dts":
		return "DTS"
	}
	return strings.ToUpper(c)
}

// SegmentCount is how many segments cover a file of the given length.
func SegmentCount(durationSeconds float64) int {
	if durationSeconds <= 0 {
		return 0
	}
	return int(math.Ceil(durationSeconds / SegmentSeconds))
}

// Playlist is the full VOD playlist for a file, so players know the length
// and can seek anywhere; segments are produced on demand.
func Playlist(durationSeconds float64) string {
	n := SegmentCount(durationSeconds)
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n", SegmentSeconds)
	for i := 0; i < n; i++ {
		d := float64(SegmentSeconds)
		if i == n-1 {
			d = durationSeconds - float64(SegmentSeconds*(n-1))
		}
		fmt.Fprintf(&b, "#EXTINF:%.3f,\nseg%d.ts\n", d, i)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// Job is one FFmpeg run producing segments from StartSegment onwards.
type Job struct {
	Input        string
	OutDir       string
	StartSegment int
	Encoder      probe.Encoder
	VideoCodec   string // source codec, to decide whether the GPU can decode it
	SourceHeight int
	MaxHeight    int
}

// cudaDecodable are source codecs NVIDIA GPUs decode (NVDEC).
var cudaDecodable = map[string]bool{"h264": true, "hevc": true, "av1": true, "vp9": true, "mpeg2video": true, "vc1": true}

// Args builds the FFmpeg arguments for a job. On NVIDIA, decoding, scaling and
// encoding all stay on the GPU when it can decode the source.
func Args(j Job) []string {
	start := j.StartSegment * SegmentSeconds
	scale := j.MaxHeight > 0 && j.SourceHeight > j.MaxHeight
	gpuDecode := j.Encoder.Kind == probe.KindNVENC && cudaDecodable[j.VideoCodec]

	a := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}
	if start > 0 {
		a = append(a, "-ss", strconv.Itoa(start))
	}
	if gpuDecode {
		a = append(a, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
		if j.Encoder.GPU >= 0 {
			a = append(a, "-hwaccel_device", strconv.Itoa(j.Encoder.GPU))
		}
	}
	a = append(a, "-i", j.Input, "-map", "0:v:0", "-map", "0:a:0?", "-sn", "-dn")

	keyframes := fmt.Sprintf("expr:gte(t,n_forced*%d)", SegmentSeconds)
	switch {
	case gpuDecode:
		vf := "scale_cuda=format=yuv420p"
		if scale {
			vf = fmt.Sprintf("scale_cuda=w=-2:h=%d:format=yuv420p", j.MaxHeight)
		}
		a = append(a, "-vf", vf)
	default:
		vf := "format=yuv420p"
		if scale {
			vf = fmt.Sprintf("scale=-2:%d,format=yuv420p", j.MaxHeight)
		}
		a = append(a, "-vf", vf)
	}

	switch j.Encoder.Kind {
	case probe.KindNVENC:
		a = append(a, "-c:v", j.Encoder.Name)
		if j.Encoder.GPU >= 0 {
			a = append(a, "-gpu", strconv.Itoa(j.Encoder.GPU))
		}
		a = append(a, "-preset", "p4", "-rc", "vbr", "-b:v", "8M", "-maxrate", "12M", "-bufsize", "16M",
			"-profile:v", "high", "-forced-idr", "1", "-no-scenecut", "1", "-force_key_frames", keyframes)
	default:
		a = append(a, "-c:v", "libx264", "-preset", "veryfast", "-crf", "21", "-maxrate", "10M", "-bufsize", "16M",
			"-profile:v", "high", "-sc_threshold", "0", "-force_key_frames", keyframes)
	}

	a = append(a, "-c:a", "aac", "-ac", "2", "-b:a", "192k",
		"-f", "hls", "-hls_time", strconv.Itoa(SegmentSeconds), "-hls_list_size", "0",
		"-hls_segment_type", "mpegts", "-hls_flags", "temp_file",
		"-start_number", strconv.Itoa(j.StartSegment),
		"-muxdelay", "0", "-output_ts_offset", strconv.Itoa(start),
		"-hls_segment_filename", j.OutDir+"/seg%d.ts",
		j.OutDir+"/ffmpeg.m3u8")
	return a
}
