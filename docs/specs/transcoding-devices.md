# Feature spec: Transcoding devices

Status: Draft 1 · 26 Nov 2026 · Implemented in part (see Scope)

## Goal

Let the admin choose what converts video for players that can't play a file directly: a specific GPU, the CPU's built-in graphics, or the CPU itself. The default picks the best working device automatically.

Example: a server with an 11th-gen Core i5 and an NVIDIA card could offer:

| Option | Uses |
|---|---|
| **Automatic** (default) | The best device that passed its startup test, falling back to the CPU |
| NVIDIA A2 (NVENC) | That NVIDIA card. Each NVIDIA card is listed separately. |
| Intel UHD Graphics 750 (Quick Sync) | The CPU's integrated graphics |
| CPU only | Software encoding (libx264): slow, but works everywhere |

## Rules

- **Only working devices are offered.** At startup each encoder is tested with a real two-frame encode on each device. Devices that fail are listed as unavailable with the reason (for example "Cannot load libnvidia-encode.so.1"), never silently used.
- **Choices fall back safely.** If the chosen device stops working, CineVault uses the CPU and shows why in the health check and the settings page.
- **Decoding and encoding are separate.** NVIDIA keeps decode, scaling and encode on the GPU when it can decode the source (H.264, HEVC, AV1, VP9, MPEG-2, VC-1), and decodes on the CPU otherwise. Later backends follow the same split.
- **Hardware limits are respected.** For example, the A2 lists an AV1 encoder in FFmpeg but can't encode AV1, so AV1 output is never offered on it.

## How playback uses it

- A file plays **directly** when the browser supports its container and codecs and it's within the height limit (1080p for now).
- Otherwise it's streamed as **HLS**: H.264 video plus AAC stereo, in 6-second segments with keyframes forced on segment boundaries, so a seek restarts FFmpeg at exactly the right segment.

## Scope

- **Done (26 Nov 2026):**
  - NVIDIA NVENC, tested per card
  - CPU (libx264)
  - automatic selection
  - the settings `CINEVAULT_TRANSCODE=auto|nvenc|cpu` and `CINEVAULT_TRANSCODE_GPU=<number>`
- **Next:**
  - a Settings page with the device dropdown, showing each device's test result
- **Later:**
  - Intel Quick Sync (QSV)
  - VA-API (Intel and AMD on Linux)
  - AMD AMF
  - Apple VideoToolbox
  - HEVC output for players that support it
  - HDR-to-SDR tone mapping
  - a per-device limit on simultaneous conversions
