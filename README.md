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

## Installation

### Building from Source

```bash
git clone https://github.com/varmakarthik12/vncrec.git
cd vncrec
go build -o vncrec .
```

### Docker

```bash
docker build -t vncrec .
docker run -v $(pwd)/recordings:/recordings vncrec --host <vnc-host>
```

## Quick Start

```bash
# MP4 recording (default) - creates output-SUFFIX.mp4 files
vncrec --host 192.168.1.100 --password mypassword

# HLS recording - creates stream.m3u8 + .ts segments
vncrec --host 192.168.1.100 --password mypassword --format hls

# Daemon mode with automatic reconnection
vncrec daemon --host 192.168.1.100 --password mypassword
```

## Output Structure

**MP4 Format (default):**
```
./recordings/
├── output-17384756.mp4   # Rotates every 30 min (configurable)
└── output-17385123.mp4
```

**HLS Format:**
```
./recordings/
├── stream.m3u8           # HLS playlist
├── segment_20260131_010530_00001.ts
└── segment_20260131_010530_00002.ts
```

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
   --framerate value             Recording framerate (default: 30) [$VR_FRAMERATE]
   --crf value                   Quality setting, lower = better (default: 35) [$VR_CRF]
   --ffmpeg value                Path to ffmpeg binary (default: "ffmpeg") [$VR_FFMPEG_BIN]
   --help, -h                    Show help
   --version, -v                 Print version

COMMANDS:
   daemon, d, watch   Run continuously with automatic reconnection
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VR_VNC_HOST` | VNC server hostname | `localhost` |
| `VR_VNC_PORT` | VNC server port | `5900` |
| `VR_VNC_PASSWORD` | VNC password | `secret` |
| `VR_OUTPUT_PATH` | Output directory | Current directory |
| `VR_FORMAT` | Output format (`mp4` or `hls`) | `mp4` |
| `VR_MP4_MAX_DURATION` | Max MP4 file duration (seconds) | `1800` (30 min) |
| `VR_HLS_SEGMENT_DURATION` | HLS segment duration (seconds) | `30` |
| `VR_HLS_MAX_DURATION` | HLS max retention (seconds) | `172800` (2 days) |
| `VR_FRAMERATE` | Recording framerate | `30` |
| `VR_CRF` | Quality (lower = better) | `35` |
| `VR_FFMPEG_BIN` | FFmpeg executable path | `ffmpeg` |

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

## Daemon Mode

Daemon mode provides resilient, long-running recording:

- **Automatic Reconnection** — Retries indefinitely if VNC connection drops
- **Exponential Backoff** — Starts at 5s delay, doubles up to 2 minutes max
- **Continuous Recording** — Seamlessly continues recording after reconnection

```bash
vncrec daemon --host myhost --password secret

# Aliases
vncrec d --host myhost
vncrec watch --host myhost
```

## Requirements

- **FFmpeg** — Must be installed and in PATH (or specify with `--ffmpeg`)
- **Go 1.18+** — For building from source

