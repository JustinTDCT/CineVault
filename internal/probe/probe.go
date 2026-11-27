// Package probe detects what the host can do for transcoding: the FFmpeg build,
// its hardware acceleration methods, which GPU encoders actually work, and
// which NVIDIA GPUs are present.
//
// Encoders are tested with a real two-frame encode, not just looked up in
// FFmpeg's encoder list. FFmpeg lists encoders it was built with, which can
// include ones the hardware lacks (an NVIDIA A2 lists av1_nvenc but cannot
// encode AV1), and a listed encoder also fails when the driver's encode
// library is missing.
package probe

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// Runner runs a command and returns its stdout and stderr.
type Runner func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)

// ExecRunner runs commands with os/exec.
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// Encoder is one hardware video encoder and whether it worked in a test encode.
type Encoder struct {
	Name   string `json:"name"`
	Codec  string `json:"codec"`
	Usable bool   `json:"usable"`
	Error  string `json:"error,omitempty"`
}

// FFmpegInfo describes the FFmpeg build and the hardware encoders that work with it.
type FFmpegInfo struct {
	Found    bool      `json:"found"`
	Path     string    `json:"path"`
	Version  string    `json:"version,omitempty"`
	HWAccels []string  `json:"hwaccels"`
	Encoders []Encoder `json:"encoders"`
	Error    string    `json:"error,omitempty"`
}

// GPU is one NVIDIA GPU reported by nvidia-smi.
type GPU struct {
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	MemoryMiB int    `json:"memory_mib"`
}

// candidateEncoders are the hardware encoders CineVault knows how to use.
var candidateEncoders = []Encoder{
	{Name: "h264_nvenc", Codec: "h264"},
	{Name: "hevc_nvenc", Codec: "hevc"},
	{Name: "av1_nvenc", Codec: "av1"},
}

// FFmpeg probes the FFmpeg binary at path.
func FFmpeg(ctx context.Context, run Runner, path string) FFmpegInfo {
	info := FFmpegInfo{Path: path, HWAccels: []string{}, Encoders: []Encoder{}}

	out, _, err := run(ctx, path, "-hide_banner", "-version")
	if err != nil {
		var notFound *exec.Error
		if errors.As(err, &notFound) {
			info.Error = "ffmpeg not found at " + path
		} else {
			info.Error = "ffmpeg -version failed: " + err.Error()
		}
		return info
	}
	info.Found = true
	info.Version = parseVersion(out)

	if out, _, err := run(ctx, path, "-hide_banner", "-hwaccels"); err == nil {
		info.HWAccels = parseHWAccels(out)
	}

	listed := map[string]bool{}
	if out, _, err := run(ctx, path, "-hide_banner", "-encoders"); err == nil {
		listed = parseEncoderNames(out)
	}

	for _, c := range candidateEncoders {
		if !listed[c.Name] {
			continue
		}
		e := c
		_, stderr, err := run(ctx, path,
			"-hide_banner", "-loglevel", "error",
			"-f", "lavfi", "-i", "color=c=black:s=256x256:r=24",
			"-frames:v", "2", "-c:v", e.Name, "-f", "null", "-")
		if err == nil {
			e.Usable = true
		} else {
			e.Error = firstError(stderr)
			if e.Error == "" {
				e.Error = err.Error()
			}
		}
		info.Encoders = append(info.Encoders, e)
	}
	return info
}

// NvidiaGPUs lists NVIDIA GPUs using nvidia-smi. It returns an empty list if
// nvidia-smi is missing or fails.
func NvidiaGPUs(ctx context.Context, run Runner) []GPU {
	out, _, err := run(ctx, "nvidia-smi",
		"--query-gpu=name,driver_version,memory.total",
		"--format=csv,noheader,nounits")
	if err != nil {
		return []GPU{}
	}
	return parseGPUs(out)
}

func parseVersion(out []byte) string {
	// First line: "ffmpeg version 8.0.1-3ubuntu2 Copyright (c) 2000-2025 ..."
	line, _, _ := strings.Cut(string(out), "\n")
	f := strings.Fields(line)
	if len(f) >= 3 && f[0] == "ffmpeg" && f[1] == "version" {
		return f[2]
	}
	return ""
}

func parseHWAccels(out []byte) []string {
	accels := []string{}
	started := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Hardware acceleration methods") {
			started = true
			continue
		}
		if started && line != "" {
			accels = append(accels, line)
		}
	}
	return accels
}

func parseEncoderNames(out []byte) map[string]bool {
	// Lines look like: " V....D hevc_nvenc           NVIDIA NVENC hevc encoder (codec hevc)"
	names := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && len(f[0]) == 6 {
			names[f[1]] = true
		}
	}
	return names
}

func parseGPUs(out []byte) []GPU {
	gpus := []GPU{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			continue
		}
		mem, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
		gpus = append(gpus, GPU{
			Name:      strings.TrimSpace(parts[0]),
			Driver:    strings.TrimSpace(parts[1]),
			MemoryMiB: mem,
		})
	}
	return gpus
}

// firstError returns the first line of FFmpeg's error output without its
// "[encoder @ 0x...]" prefix. The first line names the cause (for example
// "Cannot load libnvidia-encode.so.1"); later lines are generic follow-ups.
func firstError(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if _, rest, ok := strings.Cut(line, "] "); ok {
				line = rest
			}
		}
		return line
	}
	return ""
}
