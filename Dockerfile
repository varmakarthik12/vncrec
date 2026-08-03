# ============================================================
# Stage 1: Build the vncrec binary
# ============================================================
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache Go module downloads separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /vncrec .

# ============================================================
# Stage 2: Runtime image with FFmpeg + Tailscale
# ============================================================
FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/varmakarthik12/vncrec" \
      org.opencontainers.image.description="VNC screen recorder with FFmpeg and optional Tailscale support" \
      org.opencontainers.image.licenses="MIT"

# ---- Install FFmpeg -----------------------------------------
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    ca-certificates \
    curl \
    iproute2 \
    iptables \
    && rm -rf /var/lib/apt/lists/*

# ---- Install Tailscale --------------------------------------
# Uses the official Tailscale install script to pull the latest stable release.
# Installs both `tailscaled` (daemon) and `tailscale` (CLI).
RUN curl -fsSL https://tailscale.com/install.sh | sh

# ---- Copy vncrec binary -------------------------------------
COPY --from=builder /vncrec /usr/local/bin/vncrec

# ---- Entrypoint script --------------------------------------
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# ---- Default output volume ----------------------------------
# Mount a host directory here to persist recordings.
VOLUME ["/recordings"]

# ---- Environment variable defaults --------------------------
# All VR_* vars map 1:1 to vncrec CLI flags.
# Tailscale vars are handled by the entrypoint script.
ENV VR_OUTPUT_PATH=/recordings \
    VR_VNC_HOST=localhost \
    VR_VNC_PORT=5900 \
    VR_VNC_PASSWORD=secret \
    VR_FORMAT=mp4 \
    VR_FRAMERATE=30 \
    VR_CRF=35 \
    VR_MP4_MAX_DURATION=1800 \
    VR_HLS_SEGMENT_DURATION=30 \
    VR_HLS_MAX_DURATION=172800 \
    VR_RETRY_DELAY=5 \
    VR_MAX_RETRY_DELAY=60 \
    VR_FFMPEG_BIN=/usr/bin/ffmpeg \
    # Tailscale (optional) — set TS_AUTHKEY to enable
    TS_AUTHKEY="" \
    TS_EXTRA_ARGS="" \
    TS_VNC_HOST="" \
    TS_STATE_DIR=/var/lib/tailscale

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

# Default command: run in daemon mode for resilient long-running recording.
# Override with "" or a different subcommand as needed.
CMD ["daemon"]
