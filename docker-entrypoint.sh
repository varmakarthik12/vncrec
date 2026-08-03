#!/bin/sh
# =============================================================================
# docker-entrypoint.sh
#
# Handles optional Tailscale connectivity before launching vncrec.
#
# Tailscale Environment Variables:
#   TS_AUTHKEY        - Tailscale auth key (tskey-auth-...). Set to enable Tailscale.
#   TS_EXTRA_ARGS     - Extra arguments passed to `tailscale up` (optional).
#   TS_VNC_HOST       - Tailscale machine name or hostname. When set, overrides
#                       VR_VNC_HOST with the resolved Tailscale IP of that device.
#   TS_STATE_DIR      - Where tailscaled stores state (default: /var/lib/tailscale).
#
# vncrec Environment Variables (passed through directly):
#   VR_VNC_HOST, VR_VNC_PORT, VR_VNC_PASSWORD, VR_OUTPUT_PATH,
#   VR_FORMAT, VR_FRAMERATE, VR_CRF, VR_MP4_MAX_DURATION,
#   VR_HLS_SEGMENT_DURATION, VR_HLS_MAX_DURATION, VR_RETRY_DELAY,
#   VR_MAX_RETRY_DELAY, VR_FFMPEG_BIN
# =============================================================================

set -e

# ---------------------------------------------------------------------------
# Tailscale — only activate if TS_AUTHKEY is provided
# ---------------------------------------------------------------------------
if [ -n "${TS_AUTHKEY}" ]; then
    echo "[entrypoint] TS_AUTHKEY detected — starting Tailscale..."

    # Ensure state directory exists
    mkdir -p "${TS_STATE_DIR:-/var/lib/tailscale}"

    # If /dev/net/tun exists, tailscaled can use kernel networking.
    # Otherwise, it falls back to userspace networking with SOCKS5 proxy.
    if [ -c /dev/net/tun ]; then
        echo "[entrypoint] /dev/net/tun found — running tailscaled in kernel networking mode..."
        tailscaled \
            --statedir="${TS_STATE_DIR:-/var/lib/tailscale}" \
            --socket=/var/run/tailscale/tailscaled.sock \
            2>&1 &
    else
        echo "[entrypoint] /dev/net/tun not found — running tailscaled in userspace mode with SOCKS5 proxy (127.0.0.1:1055)..."
        tailscaled \
            --tun=userspace-networking \
            --socks5-server=127.0.0.1:1055 \
            --outbound-http-proxy-listen=127.0.0.1:1055 \
            --statedir="${TS_STATE_DIR:-/var/lib/tailscale}" \
            --socket=/var/run/tailscale/tailscaled.sock \
            2>&1 &
        
        # Set proxy env vars so standard Go net dialers / HTTP clients use the proxy
        export ALL_PROXY="socks5://127.0.0.1:1055"
        export HTTP_PROXY="http://127.0.0.1:1055"
        export HTTPS_PROXY="http://127.0.0.1:1055"
        export all_proxy="socks5://127.0.0.1:1055"
        export http_proxy="http://127.0.0.1:1055"
        export https_proxy="http://127.0.0.1:1055"
    fi

    TAILSCALED_PID=$!
    echo "[entrypoint] tailscaled started (PID: ${TAILSCALED_PID})"

    # Give tailscaled a moment to start
    sleep 2

    # Authenticate and connect to the tailnet
    echo "[entrypoint] Connecting to Tailscale tailnet..."
    tailscale up \
        --authkey="${TS_AUTHKEY}" \
        ${TS_EXTRA_ARGS}

    echo "[entrypoint] Waiting for Tailscale to be ready..."
    # Poll until we have a Tailscale IP (up to 30 seconds)
    WAIT=0
    while [ $WAIT -lt 30 ]; do
        TS_IP=$(tailscale ip -4 2>/dev/null || true)
        if [ -n "${TS_IP}" ]; then
            echo "[entrypoint] Tailscale connected! This node IP: ${TS_IP}"
            break
        fi
        sleep 1
        WAIT=$((WAIT + 1))
    done

    if [ -z "${TS_IP}" ]; then
        echo "[entrypoint] WARNING: Tailscale did not obtain an IP within 30s. Proceeding anyway..."
    fi

    # If TS_VNC_HOST is set, resolve it to a Tailscale IP and override VR_VNC_HOST
    if [ -n "${TS_VNC_HOST}" ]; then
        echo "[entrypoint] Resolving Tailscale peer: ${TS_VNC_HOST}..."
        RESOLVED_IP=$(tailscale ip "${TS_VNC_HOST}" 2>/dev/null | head -1 || true)

        if [ -n "${RESOLVED_IP}" ]; then
            echo "[entrypoint] Resolved ${TS_VNC_HOST} -> ${RESOLVED_IP}"
            export VR_VNC_HOST="${RESOLVED_IP}"
        else
            echo "[entrypoint] WARNING: Could not resolve '${TS_VNC_HOST}' via Tailscale."
            echo "[entrypoint] Using TS_VNC_HOST directly as VR_VNC_HOST: ${TS_VNC_HOST}"
            export VR_VNC_HOST="${TS_VNC_HOST}"
        fi
    fi

    # Trap SIGTERM/SIGINT to cleanly disconnect Tailscale when the container stops
    trap 'echo "[entrypoint] Shutting down Tailscale..."; tailscale down 2>/dev/null; kill $TAILSCALED_PID 2>/dev/null; exit 0' TERM INT
else
    echo "[entrypoint] TS_AUTHKEY not set — running without Tailscale."
fi

# ---------------------------------------------------------------------------
# Validate critical config
# ---------------------------------------------------------------------------
echo "[entrypoint] VNC target: ${VR_VNC_HOST:-localhost}:${VR_VNC_PORT:-5900}"
echo "[entrypoint] Output path: ${VR_OUTPUT_PATH:-/recordings}"
echo "[entrypoint] Format: ${VR_FORMAT:-mp4}"
echo "[entrypoint] Launching: vncrec $*"

# ---------------------------------------------------------------------------
# Launch vncrec — exec replaces this shell so signals pass through correctly
# ---------------------------------------------------------------------------
exec /usr/local/bin/vncrec "$@"
