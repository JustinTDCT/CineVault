# Test server: cinevault (GPU transcoding)

Set up 26 Nov 2026. A headless Ubuntu VM on Hyper-V, with an NVIDIA A2 passed through for hardware transcoding.

## Summary

| Item | Value |
|---|---|
| Hostname | `cinevault` |
| Address | `10.150.125.60` (static). Web interface: http://10.150.125.60:8456/ |
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

## Development tools (installed 26 Nov 2026)

| Tool | Version | Notes |
|---|---|---|
| Go | 1.27.4 | Official tarball in `/usr/local/go`; `PATH` set in `~/.profile` |
| Docker / Compose | 29.1.3 / 2.40.3 | Ubuntu packages `docker.io` and `docker-compose-v2`; user `jdube` is in the `docker` group |
| NVIDIA Container Toolkit | 1.20.2 | NVIDIA's apt repository, then `nvidia-ctk runtime configure --runtime=docker`. `docker run --rm --gpus all ubuntu nvidia-smi` shows the A2. |
| PostgreSQL | 18.7 | Container `cinevault-db`, volume `cinevault-pgdata`, bound to `127.0.0.1:5432` only |
| Node / npm | 22.22.1 / 9.2.0 | Ubuntu packages, for the web interface later |
| psql client | 18.6 | Ubuntu package `postgresql-client` |
| git | | Repo cloned to `~/cinevault`. Pushes use a repo deploy key with write access (`~/.ssh/id_ed25519`). |

**Database credentials** are in `~/.config/cinevault/db.env` (mode 600), with a random password generated at setup. `make run` reads this file. To recreate it and the container:

```bash
mkdir -p ~/.config/cinevault && chmod 700 ~/.config/cinevault
printf 'POSTGRES_USER=cinevault\nPOSTGRES_DB=cinevault\nPOSTGRES_PASSWORD=%s\n' "$(openssl rand -hex 24)" \
  > ~/.config/cinevault/db.env
chmod 600 ~/.config/cinevault/db.env

docker run -d --name cinevault-db --restart unless-stopped \
  --env-file ~/.config/cinevault/db.env \
  -v cinevault-pgdata:/var/lib/postgresql \
  -p 127.0.0.1:5432:5432 \
  postgres:18
```

**TMDb token** (for matching titles) is in `~/.config/cinevault/tmdb.env` (mode 600) as `CINEVAULT_TMDB_TOKEN=…`. `make run` loads it if the file exists. To create or replace it without the token entering shell history:

```bash
read -rsp 'TMDb read access token: ' T; echo
printf 'CINEVAULT_TMDB_TOKEN=%s\n' "$T" > ~/.config/cinevault/tmdb.env; unset T
chmod 600 ~/.config/cinevault/tmdb.env
```

**CineVault Cache** URL and subscription token are in `~/.config/cinevault/cache.env` (mode 600) as `CINEVAULT_CACHE_URL=…` and `CINEVAULT_CACHE_TOKEN=…`. `make run` loads it if the file exists. Without a token the server still shares its titles with the cache (Settings, "Help build the CineVault Cache").

**Backups:** Hyper-V checkpoints don't work while the GPU is assigned. Use an offline copy of the VM's disk instead: stop the VM, copy its `.vhdx`, then start it again.

## To do

- [x] Install the NVIDIA Container Toolkit (done 26 Nov 2026).
- [ ] Benchmark a real full-GPU pipeline (4K HEVC source → 1080p H.264) and record concurrent stream capacity.
- [ ] Optional: `sudo apt autoremove` to clear the old 7.0.0-30 kernel packages.
