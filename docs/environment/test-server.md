# Test server: cinevault (GPU transcoding)

Set up 26 Nov 2026. A headless Ubuntu VM on Hyper-V, with an NVIDIA A2 passed through for hardware transcoding.

## Summary

| Item | Value |
|---|---|
| Hostname | `cinevault` |
| Hypervisor | Hyper-V on Windows Server, Generation 2 VM named `CineVault` |
| OS | Ubuntu 26.04 LTS Server ("resolute"), headless |
| GPU | NVIDIA A2, 16 GB (GA107GL, PCI ID `10DE:25B6`), passed through with Discrete Device Assignment |
| GPU address in guest | `3244:00:00.0` (a Hyper-V synthetic PCI domain, which is normal) |
| Driver | `nvidia-driver-580-server-open`, 580.178.04, CUDA 13.0 |
| Video libraries | `libnvidia-encode-580-server`, `libnvidia-decode-580-server` |
| FFmpeg | 8.0.1 (Ubuntu package), with `h264_nvenc`, `hevc_nvenc` and CUDA hardware decoding |
| Persistence mode | Enabled, survives reboots |

## A2 capabilities relevant to CineVault

- **Decode (NVDEC):** H.264, HEVC, AV1, VP9 and others.
- **Encode (NVENC):** H.264 and HEVC only. FFmpeg lists `av1_nvenc`, but the A2's hardware can't encode AV1, so CineVault must not offer AV1 output on this card.
- **Concurrent sessions:** no consumer-style NVENC session limit, because it's a data-center card.

## Baseline benchmark

Run 26 Nov 2026 on a synthetic 4K test pattern encoded to HEVC:

```bash
ffmpeg -hide_banner -f lavfi -i testsrc2=size=3840x2160:rate=24 -t 30 \
  -c:v hevc_nvenc -preset p4 -b:v 20M -f null -
```

Result: **90 fps, 3.73× real time** (720 frames in 8.0 s).

This is a floor. The CPU generates the test frames and uploads them to the GPU. A full GPU pipeline (NVDEC → `scale_cuda` → NVENC), and the more common 4K → 1080p job, should both run faster. Re-measure with real media once CineVault's transcoder exists.

## Setup steps (for rebuilding)

### 1. Hyper-V host (PowerShell as Administrator)

The A2 was previously assigned to another VM, which was deleted. The GPU stayed dismounted from the host.

```powershell
# Find the GPU, which is still dismounted from the host
Get-VMHostAssignableDevice | Format-List InstanceID, LocationPath
$loc = (Get-VMHostAssignableDevice | Where-Object InstanceID -like "*VEN_10DE*").LocationPath
# LocationPath on this host: PCIROOT(AE)#PCI(0000)#PCI(0000)

$vm = "CineVault"
Stop-VM -Name $vm
Set-VM -Name $vm -AutomaticStopAction TurnOff
Set-VMMemory -VMName $vm -DynamicMemoryEnabled $false
Set-VM -Name $vm -GuestControlledCacheTypes $true -LowMemoryMappedIoSpace 3GB -HighMemoryMappedIoSpace 33280MB
Add-VMAssignableDevice -VMName $vm -LocationPath $loc
Start-VM -Name $vm
```

If the GPU isn't listed by `Get-VMHostAssignableDevice` (for example after a host rebuild), dismount it first:

```powershell
$gpu = Get-PnpDevice -PresentOnly | Where-Object { $_.InstanceId -like "PCI\VEN_10DE*" }
$loc = ($gpu | Get-PnpDeviceProperty DEVPKEY_Device_LocationPaths).Data[0]
Disable-PnpDevice -InstanceId $gpu.InstanceId -Confirm:$false
Dismount-VMHostAssignableDevice -LocationPath $loc -Force
```

**VM constraints from DDA:**
- static memory only
- no checkpoints
- automatic stop action must be TurnOff
- Secure Boot can stay on, with the "Microsoft UEFI Certificate Authority" template, because Ubuntu's driver modules are signed

### 2. Ubuntu guest

```bash
lspci | grep -i nvidia           # expect: NVIDIA Corporation GA107GL [A2 / A16]

sudo apt update && sudo apt full-upgrade -y
sudo ubuntu-drivers install --gpgpu nvidia:580-server-open
sudo apt install -y nvidia-utils-580-server
sudo reboot

nvidia-smi                       # expect: NVIDIA A2, 16380MiB, driver 580.x
```

**Encode and decode libraries.** The `--gpgpu` install doesn't include them, and without them FFmpeg fails with "Cannot load libnvidia-encode.so.1":

```bash
sudo apt install -y libnvidia-encode-580-server libnvidia-decode-580-server
ldconfig -p | grep -E 'libnvidia-encode|libnvcuvid'
```

**Persistence mode.** Ubuntu's `nvidia-persistenced` unit is static and passes `--no-persistence-mode`, so it needs a boot link and an override:

```bash
sudo systemctl add-wants multi-user.target nvidia-persistenced.service
sudo mkdir -p /etc/systemd/system/nvidia-persistenced.service.d
printf '[Service]\nExecStart=\nExecStart=/usr/bin/nvidia-persistenced --user nvidia-persistenced --verbose\n' \
  | sudo tee /etc/systemd/system/nvidia-persistenced.service.d/override.conf
sudo systemctl daemon-reload
sudo systemctl restart nvidia-persistenced
nvidia-smi --query-gpu=persistence_mode --format=csv   # expect: Enabled
```

**FFmpeg:**

```bash
sudo apt install -y ffmpeg
ffmpeg -hide_banner -encoders | grep nvenc   # h264_nvenc, hevc_nvenc (ignore av1_nvenc)
ffmpeg -hide_banner -hwaccels                # includes cuda
```

## To do

- [ ] Install the NVIDIA Container Toolkit if CineVault runs in Docker.
- [ ] Benchmark a real full-GPU pipeline (4K HEVC source → 1080p H.264) and record concurrent stream capacity.
- [ ] Optional: `sudo apt autoremove` to clear the old 7.0.0-30 kernel packages.
