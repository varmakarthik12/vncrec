# VNC Recorder

> Record VNC screens as video using FFmpeg — supports MP4 and HLS formats.

A lightweight Go application that connects to a VNC server and records the screen. Supports both **MP4** (with rotating files) and **HLS** (live streaming with auto-cleanup) output formats.

Built on top of [amitbet/vnc2video](https://github.com/amitbet/vnc2video).

## Features

- **MP4 Format** — Rotating files with configurable max duration (default: 30 min)
- **HLS Format** — Live streaming with `.m3u8` playlist and auto-cleanup
- **Daemon Mode** — Continuous recording with automatic reconnection
- **Low CPU Usage** — Uses `ultrafast` preset for minimal compute overhead
- **Configurable** — All settings via CLI flags or environment variables
- **Docker Ready** — Pre-built multi-arch image on GHCR
- **Tailscale Support** — Connect to any Tailscale device with just an auth key

---

## Installation

### Docker (recommended)

```bash
docker pull ghcr.io/varmakarthik12/vncrec:latest
```

### Using go install

```bash
go install github.com/varmakarthik12/vncrec@latest
```

### Building from Source

```bash
git clone https://github.com/varmakarthik12/vncrec.git
cd vncrec
go build -o vncrec .
```

---

## Quick Start

### Binary

```bash
# MP4 recording (default) - creates output-SUFFIX.mp4 files
vncrec --host 192.168.1.100 --password mypassword

# HLS recording - creates stream.m3u8 + .ts segments
vncrec --host 192.168.1.100 --password mypassword --format hls

# Daemon mode with automatic reconnection
vncrec daemon --host 192.168.1.100 --password mypassword
```

### Docker

```bash
# Basic MP4 recording — recordings saved to ./recordings on the host
docker run --rm \
  -e VR_VNC_HOST=192.168.1.100 \
  -e VR_VNC_PASSWORD=mypassword \
  -v $(pwd)/recordings:/recordings \
  ghcr.io/varmakarthik12/vncrec:latest

# Daemon mode (default CMD) with HLS output
docker run -d \
  -e VR_VNC_HOST=192.168.1.100 \
  -e VR_VNC_PASSWORD=mypassword \
  -e VR_FORMAT=hls \
  -v $(pwd)/recordings:/recordings \
  ghcr.io/varmakarthik12/vncrec:latest
```

---

## Output Structure

**MP4 Format (default):**
```
/recordings/
├── output-17384756.mp4   # Rotates every 30 min (configurable)
└── output-17385123.mp4
```

**HLS Format:**
```
/recordings/
├── stream.m3u8           # HLS playlist
├── segment_20260131_010530_00001.ts
└── segment_20260131_010530_00002.ts
```

---

## Command Line Options

```
OPTIONS:
   --host value                  VNC server hostname (default: "localhost") [$VR_VNC_HOST]
   --port value                  VNC server port (default: 5900) [$VR_VNC_PORT]
   --password value              VNC password (default: "secret") [$VR_VNC_PASSWORD]
   --output-path value           Output directory (default: ./recordings/) [$VR_OUTPUT_PATH]
   --format value                Output format: 'mp4' (default) or 'hls' [$VR_FORMAT]
   --mp4-max-duration value      Max duration per MP4 file in seconds (default: 1800 = 30 min) [$VR_MP4_MAX_DURATION]
   --hls-segment-duration value  HLS segment duration in seconds, max 30 (default: 30) [$VR_HLS_SEGMENT_DURATION]
   --hls-max-duration value      Max HLS recording to retain in seconds (default: 172800 = 2 days) [$VR_HLS_MAX_DURATION]
   --retry-delay value           Initial retry delay in seconds for daemon mode (default: 5) [$VR_RETRY_DELAY]
   --max-retry-delay value       Maximum retry delay cap in seconds for daemon exponential backoff (default: 5) [$VR_MAX_RETRY_DELAY]
   --framerate value             Recording framerate (default: 30) [$VR_FRAMERATE]
   --crf value                   Quality setting, lower = better (default: 35) [$VR_CRF]
   --ffmpeg value                Path to ffmpeg binary (default: "ffmpeg") [$VR_FFMPEG_BIN]
   --help, -h                    Show help
   --version, -v                 Print version

COMMANDS:
   daemon, d, watch   Run continuously with automatic reconnection
```

---

## Environment Variables

### vncrec Options

All CLI flags are available as environment variables. These work identically whether running the binary or the Docker image.

| Variable | Description | Default |
|----------|-------------|---------|
| `VR_VNC_HOST` | VNC server hostname | `localhost` |
| `VR_VNC_PORT` | VNC server port | `5900` |
| `VR_VNC_PASSWORD` | VNC password | `secret` |
| `VR_OUTPUT_PATH` | Output directory | `/recordings` (Docker) / current dir |
| `VR_FORMAT` | Output format (`mp4` or `hls`) | `mp4` |
| `VR_MP4_MAX_DURATION` | Max MP4 file duration (seconds) | `1800` (30 min) |
| `VR_HLS_SEGMENT_DURATION` | HLS segment duration (seconds) | `30` |
| `VR_HLS_MAX_DURATION` | HLS max retention (seconds) | `172800` (2 days) |
| `VR_RETRY_DELAY` | Initial daemon retry delay (seconds) | `5` |
| `VR_MAX_RETRY_DELAY` | Maximum daemon retry delay cap (seconds) | `60` |
| `VR_FRAMERATE` | Recording framerate | `30` |
| `VR_CRF` | Quality (lower = better) | `35` |
| `VR_FFMPEG_BIN` | FFmpeg executable path | `/usr/bin/ffmpeg` (Docker) |

### Tailscale Options (Docker only)

These are handled by the Docker entrypoint. Set `TS_AUTHKEY` to enable Tailscale.

| Variable | Description | Default |
|----------|-------------|---------|
| `TS_AUTHKEY` | Tailscale auth key (`tskey-auth-...`). Enables Tailscale when set. | _(disabled)_ |
| `TS_VNC_HOST` | Tailscale machine name or hostname of the VNC server. Resolves to Tailscale IP and sets `VR_VNC_HOST` automatically. | _(not set)_ |
| `TS_EXTRA_ARGS` | Additional arguments passed to `tailscale up` (e.g. `--hostname=vncrec`) | _(empty)_ |
| `TS_STATE_DIR` | Directory where tailscaled stores its state | `/var/lib/tailscale` |

---

## Docker Usage

### Pull from GitHub Container Registry

```bash
docker pull ghcr.io/varmakarthik12/vncrec:latest

# Or pin to a specific commit SHA
docker pull ghcr.io/varmakarthik12/vncrec:a1b2c3d
```

### Basic Recording

```bash
# Single MP4 recording (exits when done)
docker run --rm \
  -e VR_VNC_HOST=192.168.1.100 \
  -e VR_VNC_PASSWORD=secret \
  -v /path/to/recordings:/recordings \
  ghcr.io/varmakarthik12/vncrec:latest

# Override command to single-shot (not daemon)
docker run --rm \
  -e VR_VNC_HOST=192.168.1.100 \
  -v /path/to/recordings:/recordings \
  ghcr.io/varmakarthik12/vncrec:latest \
  ""   # empty string = default recorder (no daemon)
```

### Daemon Mode (recommended for production)

```bash
docker run -d \
  --name vncrec \
  --restart unless-stopped \
  -e VR_VNC_HOST=my-vnc-server.local \
  -e VR_VNC_PASSWORD=secret \
  -e VR_FORMAT=hls \
  -e VR_HLS_MAX_DURATION=86400 \
  -v /mnt/storage/recordings:/recordings \
  ghcr.io/varmakarthik12/vncrec:latest
```

### Docker Compose

```yaml
services:
  vncrec:
    image: ghcr.io/varmakarthik12/vncrec:latest
    restart: unless-stopped
    environment:
      VR_VNC_HOST: 192.168.1.100
      VR_VNC_PASSWORD: secret
      VR_FORMAT: mp4
      VR_MP4_MAX_DURATION: 1800
      VR_FRAMERATE: 30
      VR_CRF: 35
    volumes:
      - ./recordings:/recordings
```

---

## Tailscale Integration

The Docker image includes a full Tailscale installation. This allows you to record VNC sessions from **any device on your Tailscale network** — even across NAT, firewalls, or different cloud regions — without any VPN configuration or port forwarding.

### Step 1 — Get a Tailscale Auth Key

1. Go to [https://login.tailscale.com/admin/settings/keys](https://login.tailscale.com/admin/settings/keys)
2. Click **Generate auth key**
3. Recommended settings:
   - ✅ **Reusable** — so the container can reconnect after restart
   - ✅ **Ephemeral** — the device is automatically removed from your tailnet when the container stops
   - Set an appropriate **expiry** (or disable expiry for long-running containers)
4. Copy the generated key — it looks like `tskey-auth-kXXXXXXXXX-XXXXXXXXXXXXXXXXXXXXXXXXX`

### Step 2 — Run with Tailscale

The container uses **userspace networking mode** — no extra Linux capabilities or `/dev/net/tun` device needed.

```bash
docker run -d \
  --name vncrec-ts \
  --restart unless-stopped \
  -e TS_AUTHKEY=tskey-auth-kXXXXXXXXX-XXXXXXXXXXXXXXXXXXXXXXXXX \
  -e TS_VNC_HOST=my-desktop \
  -e VR_VNC_PASSWORD=secret \
  -v /path/to/recordings:/recordings \
  ghcr.io/varmakarthik12/vncrec:latest
```

`TS_VNC_HOST=my-desktop` is the **Tailscale machine name** of the device running the VNC server (visible in your [Tailscale admin console](https://login.tailscale.com/admin/machines)). The entrypoint automatically resolves it to the correct Tailscale IP and sets `VR_VNC_HOST`.

### Step 3 — Verify connection

```bash
# Check logs to see the Tailscale IP resolution
docker logs vncrec-ts
```

You should see output like:
```
[entrypoint] TS_AUTHKEY detected — starting Tailscale...
[entrypoint] Tailscale connected! This node IP: 100.x.y.z
[entrypoint] Resolved my-desktop -> 100.a.b.c
[entrypoint] VNC target: 100.a.b.c:5900
[entrypoint] Launching: vncrec daemon
```

### Tailscale + Docker Compose

```yaml
services:
  vncrec:
    image: ghcr.io/varmakarthik12/vncrec:latest
    restart: unless-stopped
    environment:
      # Tailscale
      TS_AUTHKEY: tskey-auth-kXXXXXXXXX-XXXXXXXXXXXXXXXXXXXXXXXXX
      TS_VNC_HOST: my-remote-desktop      # Tailscale machine name
      TS_EXTRA_ARGS: --hostname=vncrec-recorder  # optional: set this node's name
      # vncrec settings
      VR_VNC_PASSWORD: secret
      VR_FORMAT: hls
      VR_HLS_MAX_DURATION: 172800
    volumes:
      - ./recordings:/recordings
      - tailscale-state:/var/lib/tailscale

volumes:
  tailscale-state:   # Persist Tailscale state so the node doesn't re-auth on restart
```

> **Tip:** Store `TS_AUTHKEY` as a Docker secret or in a `.env` file rather than inline in `docker-compose.yml`.

### Tailscale Advanced Options

```bash
# Set a custom hostname for this recorder node in the tailnet
-e TS_EXTRA_ARGS="--hostname=vncrec-prod"

# Use a specific Tailscale exit node
-e TS_EXTRA_ARGS="--exit-node=<exit-node-ip>"

# Accept all routes from the tailnet
-e TS_EXTRA_ARGS="--accept-routes"

# Combine multiple extra args (space-separated)
-e TS_EXTRA_ARGS="--hostname=vncrec --accept-routes"
```

---

## Daemon Mode

Daemon mode provides resilient, long-running recording:

- **Automatic Reconnection** — Retries indefinitely if VNC connection drops
- **Exponential Backoff** — Starts at configurable delay (default: 5s), doubles up to a configurable max
- **Continuous Recording** — Seamlessly continues recording after reconnection

```bash
vncrec daemon --host myhost --password secret

# Aliases
vncrec d --host myhost
vncrec watch --host myhost
```

---

## Examples

```bash
# MP4 with 1-hour max duration
vncrec --host myhost --mp4-max-duration 3600

# High quality MP4 at 60fps
vncrec --host myhost --framerate 60 --crf 20

# HLS with 1-day retention
vncrec --host myhost --format hls --hls-max-duration 86400

# HLS with 10-second segments for lower latency
vncrec --host myhost --format hls --hls-segment-duration 10

# Using environment variables
export VR_VNC_HOST=192.168.1.100
export VR_VNC_PASSWORD=secret
export VR_FORMAT=mp4
vncrec daemon
```

---

## Building the Docker Image Locally

```bash
git clone https://github.com/varmakarthik12/vncrec.git
cd vncrec

# Build for your local platform
docker build -t vncrec .

# Test it
docker run --rm \
  -e VR_VNC_HOST=192.168.1.100 \
  -e VR_VNC_PASSWORD=secret \
  -v $(pwd)/recordings:/recordings \
  vncrec
```

---


## Requirements

- **FFmpeg** — Bundled in the Docker image. For binary installs, must be in `PATH` or specified with `--ffmpeg`.
- **Go 1.21+** — For building from source.
- **Docker** — For container usage.
- **Tailscale account** — Optional, only needed for Tailscale connectivity.
