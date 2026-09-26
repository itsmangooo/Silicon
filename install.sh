#!/bin/sh
set -eu

MODE="install"
REF="${SILICON_VERSION:-main}"
REPOSITORY_URL="${SILICON_REPOSITORY_URL:-https://github.com/itsmangooo/Silicon.git}"
PUBLIC_URL="${SILICON_PUBLIC_URL:-http://localhost}"
HTTP_PORT="${SILICON_HTTP_PORT:-80}"
if [ "$(id -u)" -eq 0 ]; then DEFAULT_INSTALL_DIR="/opt/silicon"; else DEFAULT_INSTALL_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/silicon"; fi
INSTALL_DIR="${SILICON_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
TEST_MODE="${SILICON_INSTALL_TEST_MODE:-false}"
READINESS_ATTEMPTS="${SILICON_READINESS_ATTEMPTS:-60}"

fail() { printf 'Silicon installer: %s\n' "$*" >&2; exit 1; }
info() { printf 'Silicon installer: %s\n' "$*"; }
usage() { printf '%s\n' 'Usage: install.sh [--update] [--version REF] [--install-dir PATH] [--public-url URL] [--http-port PORT]'; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --update) MODE="update"; shift ;;
    --version) [ "$#" -ge 2 ] || fail '--version requires a value'; REF=$2; shift 2 ;;
    --install-dir) [ "$#" -ge 2 ] || fail '--install-dir requires a value'; INSTALL_DIR=$2; shift 2 ;;
    --public-url) [ "$#" -ge 2 ] || fail '--public-url requires a value'; PUBLIC_URL=$2; shift 2 ;;
    --http-port) [ "$#" -ge 2 ] || fail '--http-port requires a value'; HTTP_PORT=$2; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) fail "unknown option: $1" ;;
  esac
done

OS_NAME="${SILICON_TEST_OS:-$(uname -s)}"
ARCH_NAME="${SILICON_TEST_ARCH:-$(uname -m)}"
[ "$OS_NAME" = "Linux" ] || fail "unsupported operating system: $OS_NAME (Linux is required)"
case "$ARCH_NAME" in x86_64|amd64|aarch64|arm64) ;; *) fail "unsupported architecture: $ARCH_NAME" ;; esac
case "$PUBLIC_URL" in http://*|https://*) ;; *) fail '--public-url must be an absolute HTTP or HTTPS URL' ;; esac
case "$PUBLIC_URL" in *" "*) fail '--public-url contains unsafe whitespace' ;; esac
case "$HTTP_PORT" in ''|*[!0-9]*) fail '--http-port must be numeric' ;; esac
[ "$HTTP_PORT" -ge 1 ] && [ "$HTTP_PORT" -le 65535 ] || fail '--http-port must be between 1 and 65535'

if [ "$TEST_MODE" != "true" ]; then
  command -v git >/dev/null 2>&1 || fail 'git is required'
  command -v openssl >/dev/null 2>&1 || fail 'openssl is required for secure configuration generation'
  command -v curl >/dev/null 2>&1 || fail 'curl is required for readiness checks'
  command -v docker >/dev/null 2>&1 || fail 'Docker is required'
  docker version >/dev/null 2>&1 || fail 'Docker is installed but the daemon is unavailable to this user'
  docker compose version >/dev/null 2>&1 || fail 'Docker Compose v2 is required'
fi

SOURCE_DIR="$INSTALL_DIR/source"
CONFIG_DIR="$INSTALL_DIR/config"
DATA_DIR="$INSTALL_DIR/data"
ENV_FILE="$CONFIG_DIR/silicon.env"
mkdir -p "$CONFIG_DIR" "$DATA_DIR/postgres"
chmod 700 "$CONFIG_DIR"

if [ -f "$ENV_FILE" ]; then
  info "existing installation detected at $INSTALL_DIR; preserving configuration and data"
else
  [ "$MODE" != "update" ] || fail "no existing installation was found at $INSTALL_DIR"
  umask 077
  if [ "$TEST_MODE" = "true" ]; then
    DATABASE_PASSWORD="test-generated-database-password"
    ENCRYPTION_KEY="dGVzdC1vbmx5LWtleS0zMi1ieXRlcy1sb25nISEhISE="
  else
    DATABASE_PASSWORD=$(openssl rand -hex 32)
    ENCRYPTION_KEY=$(openssl rand -base64 32 | tr -d '\n')
  fi
  COOKIE_SECURE="false"; TRUST_FORWARDED_PROTO="false"; BIND_ADDRESS="0.0.0.0"
  case "$PUBLIC_URL" in https://*) COOKIE_SECURE="true"; TRUST_FORWARDED_PROTO="true"; BIND_ADDRESS="127.0.0.1" ;; esac
  cat > "$ENV_FILE" <<EOF
POSTGRES_DB=silicon
POSTGRES_USER=silicon
POSTGRES_PASSWORD=$DATABASE_PASSWORD
SILICON_ENCRYPTION_KEY=$ENCRYPTION_KEY
SILICON_PUBLIC_URL=$PUBLIC_URL
SILICON_COOKIE_SECURE=$COOKIE_SECURE
SILICON_BIND_ADDRESS=$BIND_ADDRESS
SILICON_HTTP_PORT=$HTTP_PORT
SILICON_TRUST_FORWARDED_PROTO=$TRUST_FORWARDED_PROTO
SILICON_DATA_DIR=$DATA_DIR
SILICON_AGENT_EXPECTED_VERSION=0.1.0
SILICON_LOG_LEVEL=info
EOF
  chmod 600 "$ENV_FILE"
  unset DATABASE_PASSWORD ENCRYPTION_KEY
  info "generated production configuration at $ENV_FILE"
fi

if [ "$TEST_MODE" = "true" ]; then
  info "test mode completed without starting containers"
  exit 0
fi

if [ ! -d "$SOURCE_DIR/.git" ]; then
  [ ! -e "$SOURCE_DIR" ] || fail "$SOURCE_DIR exists but is not a Silicon Git checkout"
  info "obtaining Silicon source ref $REF"
  git clone --filter=blob:none "$REPOSITORY_URL" "$SOURCE_DIR"
  if [ "$REF" = "main" ]; then git -C "$SOURCE_DIR" checkout main; else git -C "$SOURCE_DIR" checkout --detach "$REF"; fi
elif [ "$MODE" = "update" ]; then
  [ -z "$(git -C "$SOURCE_DIR" status --porcelain)" ] || fail 'installed source contains local changes; refusing to overwrite them'
  info "updating Silicon source to $REF"
  git -C "$SOURCE_DIR" fetch --tags origin
  if [ "$REF" = "main" ]; then
    git -C "$SOURCE_DIR" checkout main
    git -C "$SOURCE_DIR" merge --ff-only origin/main
  else
    git -C "$SOURCE_DIR" checkout --detach "$REF"
  fi
fi

compose() { docker compose --env-file "$ENV_FILE" -f "$SOURCE_DIR/docker-compose.production.yml" "$@"; }
info 'building Silicon services'
# Build completes before running containers, so a build failure leaves the current installation running.
compose build || fail 'service build failed; the existing installation was not replaced'
info 'starting Silicon and applying embedded migrations'
compose up -d || fail 'service startup failed; inspect docker compose logs'

READY="false"; attempt=0
while [ "$attempt" -lt "$READINESS_ATTEMPTS" ]; do
  if curl -fsS "http://127.0.0.1:$HTTP_PORT/healthz" >/dev/null 2>&1; then READY="true"; break; fi
  attempt=$((attempt + 1)); sleep 2
done
if [ "$READY" != "true" ]; then
  compose ps >&2 || true
  fail 'Silicon did not become ready before the readiness deadline; existing PostgreSQL data and configuration were preserved'
fi

info "Silicon is ready at $PUBLIC_URL"
info "Persistent data: $DATA_DIR"
info "Configuration: $ENV_FILE"
info "Update with: $SOURCE_DIR/install.sh --update --install-dir $INSTALL_DIR"
