#!/bin/sh
set -eu

MODE="install"
REF="${SILICON_VERSION:-main}"
REPOSITORY_URL="${SILICON_REPOSITORY_URL:-https://github.com/itsmangooo/Silicon.git}"
if [ "${SILICON_PUBLIC_URL+x}" = "x" ]; then PUBLIC_URL_EXPLICIT="true"; else PUBLIC_URL_EXPLICIT="false"; fi
if [ "${SILICON_HTTP_PORT+x}" = "x" ]; then HTTP_PORT_EXPLICIT="true"; else HTTP_PORT_EXPLICIT="false"; fi
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
    --public-url) [ "$#" -ge 2 ] || fail '--public-url requires a value'; PUBLIC_URL=$2; PUBLIC_URL_EXPLICIT="true"; shift 2 ;;
    --http-port) [ "$#" -ge 2 ] || fail '--http-port requires a value'; HTTP_PORT=$2; HTTP_PORT_EXPLICIT="true"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) fail "unknown option: $1" ;;
  esac
done

SOURCE_DIR="$INSTALL_DIR/source"
CONFIG_DIR="$INSTALL_DIR/config"
DATA_DIR="$INSTALL_DIR/data"
ENV_FILE="$CONFIG_DIR/silicon.env"
EXISTING_INSTALLATION="false"

config_value() {
  key=$1
  sed -n "s/^${key}=//p" "$ENV_FILE" | tail -n 1
}

if [ -f "$ENV_FILE" ]; then
  EXISTING_INSTALLATION="true"
  CONFIGURED_HTTP_PORT=$(config_value SILICON_HTTP_PORT)
  CONFIGURED_PUBLIC_URL=$(config_value SILICON_PUBLIC_URL)
  [ -n "$CONFIGURED_HTTP_PORT" ] || fail "existing configuration at $ENV_FILE has no SILICON_HTTP_PORT"
  [ -n "$CONFIGURED_PUBLIC_URL" ] || fail "existing configuration at $ENV_FILE has no SILICON_PUBLIC_URL"
  if [ "$HTTP_PORT_EXPLICIT" = "true" ] && [ "$HTTP_PORT" != "$CONFIGURED_HTTP_PORT" ]; then
    fail "existing installation uses HTTP port $CONFIGURED_HTTP_PORT; refusing to replace it with $HTTP_PORT or overwrite $ENV_FILE"
  fi
  if [ "$PUBLIC_URL_EXPLICIT" = "true" ] && [ "$PUBLIC_URL" != "$CONFIGURED_PUBLIC_URL" ]; then
    fail "existing installation uses public URL $CONFIGURED_PUBLIC_URL; refusing to replace it or overwrite $ENV_FILE"
  fi
  HTTP_PORT=$CONFIGURED_HTTP_PORT
  PUBLIC_URL=$CONFIGURED_PUBLIC_URL
fi

OS_NAME="${SILICON_TEST_OS:-$(uname -s)}"
ARCH_NAME="${SILICON_TEST_ARCH:-$(uname -m)}"
[ "$OS_NAME" = "Linux" ] || fail "unsupported operating system: $OS_NAME (Linux is required)"
case "$ARCH_NAME" in x86_64|amd64|aarch64|arm64) ;; *) fail "unsupported architecture: $ARCH_NAME" ;; esac
case "$PUBLIC_URL" in http://*|https://*) ;; *) fail '--public-url must be an absolute HTTP or HTTPS URL' ;; esac
case "$PUBLIC_URL" in *" "*) fail '--public-url contains unsafe whitespace' ;; esac
case "$HTTP_PORT" in ''|*[!0-9]*) fail '--http-port must be numeric' ;; esac
if [ "$HTTP_PORT" -lt 1 ] || [ "$HTTP_PORT" -gt 65535 ]; then
  fail '--http-port must be between 1 and 65535'
fi

port_is_occupied() {
  port=$1
  if [ "$TEST_MODE" = "true" ]; then
    case ",${SILICON_INSTALL_TEST_PORTS_IN_USE:-}," in *",$port,"*) return 0 ;; *) return 1 ;; esac
  fi
  if command -v ss >/dev/null 2>&1; then
    ss -H -ltn "sport = :$port" 2>/dev/null | grep -q .
    return $?
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | grep -q .
    return $?
  fi
  port_hex=$(printf '%04X' "$port")
  awk -v port="$port_hex" 'NR > 1 { split($2, address, ":"); if (toupper(address[2]) == port && $4 == "0A") found = 1 } END { exit !found }' /proc/net/tcp /proc/net/tcp6 2>/dev/null
}

port_conflict_details() {
  port=$1
  if [ "$TEST_MODE" = "true" ]; then
    printf 'simulated listener on TCP port %s\n' "$port"
    return
  fi
  details=""
  if command -v ss >/dev/null 2>&1; then
    details=$(ss -H -ltnp "sport = :$port" 2>/dev/null || true)
  elif command -v lsof >/dev/null 2>&1; then
    details=$(lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)
  fi
  if command -v docker >/dev/null 2>&1; then
    containers=$(docker ps --filter "publish=$port" --format 'Docker container {{.Names}} ({{.ID}}): {{.Ports}}' 2>/dev/null || true)
    if [ -n "$containers" ]; then details=$(printf '%s\n%s' "$details" "$containers"); fi
  fi
  if [ -n "$details" ]; then printf '%s\n' "$details"; else printf 'another process is listening on TCP port %s (process details are unavailable to this user)\n' "$port"; fi
}

find_available_port() {
  for candidate in 8080 8081 8082 8083 8000 8888; do
    if ! port_is_occupied "$candidate"; then printf '%s\n' "$candidate"; return 0; fi
  done
  return 1
}

is_interactive_install() {
  if [ "$TEST_MODE" = "true" ] && [ "${SILICON_INSTALL_TEST_INTERACTIVE:-false}" = "true" ]; then return 0; fi
  [ -t 0 ] && [ -t 1 ]
}

existing_installation_owns_port() {
  [ "$EXISTING_INSTALLATION" = "true" ] || return 1
  [ -f "$SOURCE_DIR/docker-compose.production.yml" ] || return 1
  published=$(docker compose --env-file "$ENV_FILE" -f "$SOURCE_DIR/docker-compose.production.yml" port frontend 80 2>/dev/null || true)
  printf '%s\n' "$published" | grep -E "(^|:)$HTTP_PORT$" >/dev/null 2>&1
}

if [ "$TEST_MODE" != "true" ]; then
  command -v git >/dev/null 2>&1 || fail 'git is required'
  command -v openssl >/dev/null 2>&1 || fail 'openssl is required for secure configuration generation'
  command -v curl >/dev/null 2>&1 || fail 'curl is required for readiness checks'
  command -v docker >/dev/null 2>&1 || fail 'Docker is required'
  docker version >/dev/null 2>&1 || fail 'Docker is installed but the daemon is unavailable to this user'
  docker compose version >/dev/null 2>&1 || fail 'Docker Compose v2 is required'
fi

PORT_CHANGED_INTERACTIVELY="false"
while port_is_occupied "$HTTP_PORT"; do
  if existing_installation_owns_port; then
    info "existing Silicon frontend is already using configured HTTP port $HTTP_PORT"
    break
  fi
  info "HTTP port $HTTP_PORT is already occupied:"
  port_conflict_details "$HTTP_PORT" | sed 's/^/  /' >&2
  if [ "$EXISTING_INSTALLATION" = "true" ]; then
    fail "configured HTTP port $HTTP_PORT is in use by another service; stop the conflicting service or deliberately update $ENV_FILE"
  fi
  if ! is_interactive_install; then
    fail "requested HTTP port $HTTP_PORT is unavailable; rerun with --http-port PORT (and a matching HTTP --public-url when supplied)"
  fi
  SUGGESTED_PORT=$(find_available_port || true)
  if [ "$TEST_MODE" = "true" ] && [ "${SILICON_INSTALL_TEST_INTERACTIVE:-false}" = "true" ]; then
    if [ -n "$SUGGESTED_PORT" ]; then printf 'Choose an available HTTP port [%s]: ' "$SUGGESTED_PORT" >&2; else printf 'Choose another HTTP port: ' >&2; fi
  else
    if [ -n "$SUGGESTED_PORT" ]; then printf 'Choose an available HTTP port [%s]: ' "$SUGGESTED_PORT" >/dev/tty; else printf 'Choose another HTTP port: ' >/dev/tty; fi
  fi
  if [ "$TEST_MODE" = "true" ] && [ "${SILICON_INSTALL_TEST_INTERACTIVE:-false}" = "true" ]; then
    IFS= read -r SELECTED_PORT || fail 'no HTTP port was selected'
  else
    IFS= read -r SELECTED_PORT </dev/tty || fail 'no HTTP port was selected'
  fi
  if [ -z "$SELECTED_PORT" ]; then SELECTED_PORT=$SUGGESTED_PORT; fi
  case "$SELECTED_PORT" in ''|*[!0-9]*) info "'$SELECTED_PORT' is not a numeric TCP port"; continue ;; esac
  if [ "$SELECTED_PORT" -lt 1 ] || [ "$SELECTED_PORT" -gt 65535 ]; then info "port must be between 1 and 65535"; continue; fi
  HTTP_PORT=$SELECTED_PORT
  PORT_CHANGED_INTERACTIVELY="true"
done

case "$PUBLIC_URL" in
  http://*)
    public_rest=${PUBLIC_URL#http://}
    public_authority=${public_rest%%/*}
    case "$public_rest" in */*) public_suffix=/${public_rest#*/} ;; *) public_suffix="" ;; esac
    public_port=""
    case "$public_authority" in
      \[*\]:*) public_port=${public_authority##*:}; public_host=${public_authority%:*} ;;
      \[*\]) public_host=$public_authority ;;
      *:*) public_port=${public_authority##*:}; public_host=${public_authority%:*} ;;
      *) public_host=$public_authority ;;
    esac
    if [ -n "$public_port" ]; then
      case "$public_port" in *[!0-9]*) fail '--public-url contains an invalid HTTP port' ;; esac
      if [ "$public_port" != "$HTTP_PORT" ]; then
        if [ "$PORT_CHANGED_INTERACTIVELY" = "true" ]; then
          OLD_PUBLIC_URL=$PUBLIC_URL
          PUBLIC_URL="http://$public_host:$HTTP_PORT$public_suffix"
          info "updated public URL from $OLD_PUBLIC_URL to $PUBLIC_URL to match the selected HTTP port"
        else
          fail "--public-url uses HTTP port $public_port but the selected --http-port is $HTTP_PORT"
        fi
      fi
    elif [ "$HTTP_PORT" != "80" ]; then
      if [ "$EXISTING_INSTALLATION" = "true" ]; then
        fail "existing SILICON_PUBLIC_URL in $ENV_FILE does not include configured HTTP port $HTTP_PORT; correct the existing configuration explicitly"
      fi
      if [ "$PUBLIC_URL_EXPLICIT" = "true" ] && [ "$PORT_CHANGED_INTERACTIVELY" != "true" ]; then
        fail "HTTP --public-url must include :$HTTP_PORT to match the selected --http-port"
      fi
      OLD_PUBLIC_URL=$PUBLIC_URL
      PUBLIC_URL="http://$public_host:$HTTP_PORT$public_suffix"
      info "using public URL $PUBLIC_URL for HTTP port $HTTP_PORT"
      if [ "$PORT_CHANGED_INTERACTIVELY" = "true" ] && [ "$PUBLIC_URL_EXPLICIT" = "true" ]; then
        info "updated public URL from $OLD_PUBLIC_URL after the interactive port selection"
      fi
    fi
    ;;
esac

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
