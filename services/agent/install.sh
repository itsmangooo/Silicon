#!/bin/sh
set -eu

CONTROL_PLANE_URL=""
SERVER_ID=""
ENROLLMENT_TOKEN=""
VERSION="control-plane"
ALLOW_INSECURE="false"
CONFIG_DIR="/etc/silicon-agent"
BIN_PATH="/usr/local/bin/silicon-agent"

fail() { printf 'Silicon Agent installer: %s\n' "$*" >&2; exit 1; }
info() { printf 'Silicon Agent installer: %s\n' "$*"; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --url) [ "$#" -ge 2 ] || fail '--url requires a value'; CONTROL_PLANE_URL=$2; shift 2 ;;
    --server) [ "$#" -ge 2 ] || fail '--server requires a value'; SERVER_ID=$2; shift 2 ;;
    --token) [ "$#" -ge 2 ] || fail '--token requires a value'; ENROLLMENT_TOKEN=$2; shift 2 ;;
    --version) [ "$#" -ge 2 ] || fail '--version requires a value'; VERSION=$2; shift 2 ;;
    --allow-insecure) ALLOW_INSECURE="true"; shift ;;
    *) fail "unknown option: $1" ;;
  esac
done

[ "$(uname -s)" = "Linux" ] || fail 'only Linux is supported'
[ "$(id -u)" -eq 0 ] || fail 'run this installer as root (the Silicon command uses sudo)'
[ -n "$CONTROL_PLANE_URL" ] || fail '--url is required'
[ -n "$SERVER_ID" ] || fail '--server is required'
[ -n "$ENROLLMENT_TOKEN" ] || fail '--token is required'
command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v sha256sum >/dev/null 2>&1 || fail 'sha256sum is required'
command -v docker >/dev/null 2>&1 || fail 'Docker is required'
docker version >/dev/null 2>&1 || fail 'Docker is installed but unavailable to root'

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

case "$CONTROL_PLANE_URL" in
  https://*) ;;
  http://*) [ "$ALLOW_INSECURE" = "true" ] || fail 'control-plane URL must use HTTPS' ;;
  *) fail 'control-plane URL must be an absolute HTTPS URL' ;;
esac
CONTROL_PLANE_URL=${CONTROL_PLANE_URL%/}

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM
ASSET="silicon-agent-linux-$ARCH"
if [ "$VERSION" = "control-plane" ]; then
  BINARY_URL="$CONTROL_PLANE_URL/api/v1/agent/download/linux/$ARCH"
  CHECKSUM_URL="$CONTROL_PLANE_URL/api/v1/agent/download/linux/$ARCH/checksum"
else
  BINARY_URL="https://github.com/itsmangooo/Silicon/releases/download/$VERSION/$ASSET"
  CHECKSUM_URL="$BINARY_URL.sha256"
fi

info "downloading verified Linux $ARCH Agent"
if [ "$ALLOW_INSECURE" = "true" ]; then
  curl -fL "$BINARY_URL" -o "$TMP_DIR/$ASSET"
  curl -fL "$CHECKSUM_URL" -o "$TMP_DIR/$ASSET.sha256"
else
  curl -fL --proto '=https' --tlsv1.2 "$BINARY_URL" -o "$TMP_DIR/$ASSET"
  curl -fL --proto '=https' --tlsv1.2 "$CHECKSUM_URL" -o "$TMP_DIR/$ASSET.sha256"
fi
(cd "$TMP_DIR" && sha256sum -c "$ASSET.sha256") || fail 'Agent checksum verification failed'
install -m 0755 "$TMP_DIR/$ASSET" "$BIN_PATH"
install -d -m 0700 "$CONFIG_DIR"

# The token is passed only to the enrollment process and is never written to the service file.
if [ "$ALLOW_INSECURE" = "true" ]; then
  "$BIN_PATH" enroll --url "$CONTROL_PLANE_URL" --server "$SERVER_ID" --token "$ENROLLMENT_TOKEN" --config "$CONFIG_DIR/agent.json" --allow-insecure
else
  "$BIN_PATH" enroll --url "$CONTROL_PLANE_URL" --server "$SERVER_ID" --token "$ENROLLMENT_TOKEN" --config "$CONFIG_DIR/agent.json"
fi
unset ENROLLMENT_TOKEN

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
  cat > /etc/systemd/system/silicon-agent.service <<'EOF'
[Unit]
Description=Silicon Agent
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/silicon-agent --config /etc/silicon-agent/agent.json
Restart=always
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/etc/silicon-agent /run /tmp

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now silicon-agent.service
  systemctl is-active --quiet silicon-agent.service || fail 'systemd service did not start'
else
  info 'systemd was not detected; starting the Agent in the background'
  nohup "$BIN_PATH" --config "$CONFIG_DIR/agent.json" >/var/log/silicon-agent.log 2>&1 &
fi

CONNECTED="false"
attempt=0
while [ "$attempt" -lt 20 ]; do
  if "$BIN_PATH" check --config "$CONFIG_DIR/agent.json" >/dev/null 2>&1; then CONNECTED="true"; break; fi
  attempt=$((attempt + 1)); sleep 1
done
[ "$CONNECTED" = "true" ] || fail 'Agent started but the control plane did not confirm its connection; inspect the Agent logs'
info "Agent enrolled and connected to $CONTROL_PLANE_URL"
