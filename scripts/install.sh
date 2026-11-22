#!/bin/bash
set -euo pipefail

# =============================================================================
# SiteKit Agent Provisioning Script
# =============================================================================

AGENT_TOKEN="${1:-}"
SAAS_URL="${2:-}"
SERVER_ID="${3:-}"
RELEASE_BASE_URL="${RELEASE_BASE_URL:-https://github.com/DevTTSOL/sitekit-agent/releases/download}"
AGENT_VERSION="${AGENT_VERSION:-}"
AGENT_SHA256="${AGENT_SHA256:-}"
AGENT_DIR="/opt/sitekit"
AGENT_BIN="$AGENT_DIR/sentinel"
SERVICE_FILE="/etc/systemd/system/sitekit-agent.service"

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
is_installed() { dpkg -l "$1" 2>/dev/null | grep -q "^ii"; }

if [[ "$(id -u)" -ne 0 ]]; then
    echo "install.sh must be run as root" >&2
    exit 1
fi

if [[ -z "$AGENT_TOKEN" || -z "$SAAS_URL" ]]; then
    echo "Usage: AGENT_VERSION=<tag> AGENT_SHA256=<sha256> install.sh <agent_token> <saas_url> [server_id]" >&2
    exit 1
fi

case "$SAAS_URL" in
    https://*) ;;
    http://localhost|http://localhost:*|http://127.0.0.1|http://127.0.0.1:*|http://\[::1\]|http://\[::1\]:*) ;;
    *) echo "saas_url must use HTTPS outside loopback development" >&2; exit 1 ;;
esac

case "$RELEASE_BASE_URL" in
    https://*) ;;
    *) echo "RELEASE_BASE_URL must use HTTPS" >&2; exit 1 ;;
esac

if [[ "$AGENT_TOKEN$SAAS_URL$SERVER_ID" == *$'\n'* || "$AGENT_TOKEN$SAAS_URL$SERVER_ID" == *$'\r'* ]]; then
    echo "Configuration values must not contain line breaks" >&2
    exit 1
fi

log "Updating package lists..."
apt-get update -qq

for pkg in curl ca-certificates git unzip ufw fail2ban sudo; do
    if is_installed "$pkg"; then
        log "$pkg already installed, skipping"
    else
        log "Installing $pkg..."
        apt-get install -y -qq "$pkg"
    fi
done

install -d -o root -g root -m 0755 "$AGENT_DIR"

if [[ ! -f "$AGENT_BIN" || "${FORCE_UPDATE:-0}" == "1" ]]; then
    if [[ -z "$AGENT_VERSION" || ! "$AGENT_VERSION" =~ ^v[0-9A-Za-z._-]+$ ]]; then
        echo "AGENT_VERSION must be an exact release tag" >&2
        exit 1
    fi
    if [[ ! "$AGENT_SHA256" =~ ^[0-9a-fA-F]{64}$ ]]; then
        echo "AGENT_SHA256 must contain the independently pinned SHA-256 checksum" >&2
        exit 1
    fi

    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
    esac

    download_dir="$(mktemp -d /var/tmp/sitekit-agent.XXXXXX)"
    trap 'rm -rf "$download_dir"' EXIT
    artifact_name="sentinel-linux-$arch"
    download_path="$download_dir/$artifact_name"
    download_url="$RELEASE_BASE_URL/$AGENT_VERSION/$artifact_name"

    log "Downloading $artifact_name from $download_url"
    curl --proto '=https' --tlsv1.2 -fsSL --retry 3 "$download_url" -o "$download_path"

    actual_sha256="$(sha256sum "$download_path" | cut -d' ' -f1)"
    if [[ "${actual_sha256,,}" != "${AGENT_SHA256,,}" ]]; then
        echo "Agent checksum mismatch; refusing to install" >&2
        exit 1
    fi

    if [[ ! -f "$download_path" || -L "$download_path" ]]; then
        echo "Downloaded agent is not a regular file" >&2
        exit 1
    fi
    install -o root -g root -m 0755 "$download_path" "$AGENT_BIN"
    log "Verified agent installed"
fi

chown root:root "$AGENT_DIR" "$AGENT_BIN"
chmod 0755 "$AGENT_DIR" "$AGENT_BIN"

yaml_quote() {
    local value
    value="$(printf '%s' "$1" | sed "s/'/''/g")"
    printf "'%s'" "$value"
}

log "Writing agent configuration..."
{
    printf 'saas_url: %s\n' "$(yaml_quote "$SAAS_URL")"
    printf 'agent_token: %s\n' "$(yaml_quote "$AGENT_TOKEN")"
    printf 'server_id: %s\n' "$(yaml_quote "$SERVER_ID")"
    printf 'poll_interval: 5s\n'
    printf 'stats_interval: 60s\n'
    printf 'log_level: info\n'
} > "$AGENT_DIR/agent.yaml"
chown root:root "$AGENT_DIR/agent.yaml"
chmod 0600 "$AGENT_DIR/agent.yaml"

if systemctl list-unit-files hostman-agent.service --no-legend 2>/dev/null | grep -q hostman-agent; then
    systemctl stop hostman-agent.service || true
    systemctl disable hostman-agent.service || true
    rm -f /etc/systemd/system/hostman-agent.service
fi

log "Setting up systemd service..."
cat > "$SERVICE_FILE" <<'EOF'
[Unit]
Description=SiteKit Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/sitekit
ExecStart=/opt/sitekit/sentinel
Restart=always
RestartSec=5
UMask=0022

[Install]
WantedBy=multi-user.target
EOF
chown root:root "$SERVICE_FILE"
chmod 0644 "$SERVICE_FILE"

systemctl daemon-reload
systemctl enable sitekit-agent.service
systemctl restart sitekit-agent.service
log "Agent service started"

log "Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow http
ufw allow https
ufw --force enable

log "SiteKit agent installation complete"
