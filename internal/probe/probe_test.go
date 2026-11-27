package probe

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// Output captured from the test server (Ubuntu 26.04, FFmpeg 8.0.1, NVIDIA A2).
const (
	versionOut  = "ffmpeg version 8.0.1-3ubuntu2 Copyright (c) 2000-2025 the FFmpeg developers\nbuilt with gcc 15\n"
	hwaccelsOut = "Hardware acceleration methods:\nvdpau\ncuda\nvaapi\nqsv\ndrm\nopencl\nvulkan\n\n"
	encodersOut = `Encoders:
 V..... = Video
 ------
 V....D av1_nvenc            NVIDIA NVENC av1 encoder (codec av1)
 V....D h264_nvenc           NVIDIA NVENC H.264 encoder (codec h264)
 V....D hevc_nvenc           NVIDIA NVENC hevc encoder (codec hevc)
 V....D libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
`
	smiOut = "0, NVIDIA A2, 580.178.04, 16380\n"
)

// fakeRunner simulates the A2: H.264 and HEVC encode work, AV1 encode fails,
// and the CPU encoder works.
func fakeRunner(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case name == "nvidia-smi":
		return []byte(smiOut), nil, nil
	case strings.Contains(joined, "-version"):
		return []byte(versionOut), nil, nil
	case strings.Contains(joined, "-hwaccels"):
		return []byte(hwaccelsOut), nil, nil
	case strings.Contains(joined, "-encoders"):
		return []byte(encodersOut), nil, nil
	case strings.Contains(joined, "-c:v av1_nvenc"):
		return nil, []byte("[av1_nvenc @ 0x1] Codec not supported\nError while opening encoder\n"), errors.New("exit status 187")
	case strings.Contains(joined, "-c:v"):
		return nil, nil, nil
	}
	return nil, nil, errors.New("unexpected command: " + name + " " + joined)
}

var a2 = []GPU{{Index: 0, Name: "NVIDIA A2"}}

func TestFFmpegProbe(t *testing.T) {
	info := FFmpeg(context.Background(), fakeRunner, "ffmpeg", a2)
	if !info.Found || info.Version != "8.0.1-3ubuntu2" {
		t.Fatalf("Found=%v Version=%q", info.Found, info.Version)
	}
	if strings.Join(info.HWAccels, ",") != "vdpau,cuda,vaapi,qsv,drm,opencl,vulkan" {
		t.Errorf("HWAccels = %v", info.HWAccels)
	}
	got := map[string]Encoder{}
	for _, e := range info.Encoders {
		got[e.Name] = e
	}
	if len(got) != 4 {
		t.Fatalf("want 3 NVENC encoders and libx264, got %v", info.Encoders)
	}
	if e := got["h264_nvenc"]; e.Kind != KindNVENC || e.GPU != 0 || e.DeviceName != "NVIDIA A2" {
		t.Errorf("h264_nvenc device = %+v", e)
	}
	if e := got["libx264"]; !e.Usable || e.Kind != KindCPU || e.GPU != -1 {
		t.Errorf("libx264 = %+v", e)
	}
	if !got["h264_nvenc"].Usable || !got["hevc_nvenc"].Usable {
		t.Errorf("h264/hevc should be usable: %+v", info.Encoders)
	}
	if got["av1_nvenc"].Usable || got["av1_nvenc"].Error != "Codec not supported" {
		t.Errorf("av1_nvenc should be listed but unusable: %+v", got["av1_nvenc"])
	}
}

func TestFFmpegNotFound(t *testing.T) {
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	info := FFmpeg(context.Background(), run, "/nope/ffmpeg", nil)
	if info.Found || !strings.Contains(info.Error, "not found") {
		t.Errorf("got %+v", info)
	}
	if info.HWAccels == nil || info.Encoders == nil {
		t.Error("lists should be empty, not nil, so JSON shows []")
	}
}

func TestFirstError(t *testing.T) {
	in := "\n[hevc_nvenc @ 0x5990d5c3a040] Cannot load libnvidia-encode.so.1\n[hevc_nvenc @ 0x5990d5c3a040] The minimum required Nvidia driver for nvenc is 530.41.03 or newer\n"
	if got := firstError([]byte(in)); got != "Cannot load libnvidia-encode.so.1" {
		t.Errorf("got %q", got)
	}
	if got := firstError(nil); got != "" {
		t.Errorf("empty input: got %q", got)
	}
}

func TestNvidiaGPUs(t *testing.T) {
	gpus := NvidiaGPUs(context.Background(), fakeRunner)
	if len(gpus) != 1 || gpus[0] != (GPU{Index: 0, Name: "NVIDIA A2", Driver: "580.178.04", MemoryMiB: 16380}) {
		t.Errorf("got %+v", gpus)
	}
	none := NvidiaGPUs(context.Background(), func(context.Context, string, ...string) ([]byte, []byte, error) {
		return nil, nil, errors.New("missing")
	})
	if none == nil || len(none) != 0 {
		t.Errorf("want empty list, got %v", none)
	}
}

func TestNVENCTestedPerGPU(t *testing.T) {
	var gpuArgs []string
	run := func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-c:v h264_nvenc") {
			for i, a := range args {
				if a == "-gpu" {
					gpuArgs = append(gpuArgs, args[i+1])
				}
			}
		}
		return fakeRunner(ctx, name, args...)
	}
	two := []GPU{{Index: 0, Name: "NVIDIA A2"}, {Index: 1, Name: "NVIDIA T4"}}
	info := FFmpeg(context.Background(), run, "ffmpeg", two)
	if strings.Join(gpuArgs, ",") != "0,1" {
		t.Errorf("h264_nvenc tested with -gpu %v, want 0,1", gpuArgs)
	}
	n := 0
	for _, e := range info.Encoders {
		if e.Name == "h264_nvenc" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("want h264_nvenc listed once per GPU, got %d", n)
	}
}
