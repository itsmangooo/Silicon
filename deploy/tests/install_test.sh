#!/bin/sh
set -eu

CDPATH=''
REPOSITORY_ROOT=$(cd -- "$(dirname "$0")/../.." && pwd)
TEMP_ROOT=$(mktemp -d)
trap 'rm -rf "$TEMP_ROOT"' EXIT HUP INT TERM
FAKE_BIN="$TEMP_ROOT/bin"
mkdir -p "$FAKE_BIN"

cat > "$FAKE_BIN/docker" <<'EOF'
#!/bin/sh
case "${FAKE_DOCKER_MODE:-success}:$*" in
  missing:version) exit 1 ;;
  missing-compose:"compose version") exit 1 ;;
  startup:*up\ -d*) exit 1 ;;
  *) exit 0 ;;
esac
EOF
cat > "$FAKE_BIN/git" <<'EOF'
#!/bin/sh
[ -z "${FAKE_GIT_LOG:-}" ] || printf '%s\n' "$*" >> "$FAKE_GIT_LOG"
if [ "$1" = "clone" ]; then
  for argument in "$@"; do destination=$argument; done
  mkdir -p "$destination/.git"
  : > "$destination/docker-compose.production.yml"
fi
case "$*" in *"rev-parse HEAD"*) printf '%s\n' '0123456789abcdef0123456789abcdef01234567' ;; esac
exit 0
EOF
cat > "$FAKE_BIN/curl" <<'EOF'
#!/bin/sh
[ "${FAKE_CURL_MODE:-success}" != "fail" ]
EOF
chmod +x "$FAKE_BIN/docker" "$FAKE_BIN/git" "$FAKE_BIN/curl"
TEST_PATH="$FAKE_BIN:$PATH"

expect_failure() {
  description=$1; shift
  if "$@" >"$TEMP_ROOT/output" 2>&1; then
    printf 'expected failure: %s\n' "$description" >&2
    exit 1
  fi
}

expect_failure 'unsupported OS' env SILICON_INSTALL_TEST_MODE=true SILICON_TEST_OS=Darwin SILICON_INSTALL_DIR="$TEMP_ROOT/os" sh "$REPOSITORY_ROOT/install.sh"
export SILICON_TEST_OS=Linux

# Port preflight behavior is tested without opening real listeners so the suite remains deterministic.
PORT_FREE_DIR="$TEMP_ROOT/port-80-free"
env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_DIR="$PORT_FREE_DIR" sh "$REPOSITORY_ROOT/install.sh" >/dev/null
grep -q '^SILICON_HTTP_PORT=80$' "$PORT_FREE_DIR/config/silicon.env"
grep -q '^SILICON_PUBLIC_URL=http://localhost$' "$PORT_FREE_DIR/config/silicon.env"

expect_failure 'occupied default port' env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_TEST_PORTS_IN_USE=80 SILICON_INSTALL_DIR="$TEMP_ROOT/port-80-occupied" sh "$REPOSITORY_ROOT/install.sh"
grep -q 'HTTP port 80 is already occupied' "$TEMP_ROOT/output"
grep -q -- '--http-port PORT' "$TEMP_ROOT/output"

CUSTOM_FREE_DIR="$TEMP_ROOT/custom-port-free"
env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_DIR="$CUSTOM_FREE_DIR" sh "$REPOSITORY_ROOT/install.sh" --http-port 18080 >/dev/null
grep -q '^SILICON_HTTP_PORT=18080$' "$CUSTOM_FREE_DIR/config/silicon.env"
grep -q '^SILICON_PUBLIC_URL=http://localhost:18080$' "$CUSTOM_FREE_DIR/config/silicon.env"

expect_failure 'occupied custom port' env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_TEST_PORTS_IN_USE=18081 SILICON_INSTALL_DIR="$TEMP_ROOT/custom-port-occupied" sh "$REPOSITORY_ROOT/install.sh" --http-port 18081
grep -q 'HTTP port 18081 is already occupied' "$TEMP_ROOT/output"

expect_failure 'mismatched HTTP public URL port' env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_DIR="$TEMP_ROOT/mismatched-public-url" sh "$REPOSITORY_ROOT/install.sh" --http-port 18081 --public-url http://localhost:18082
grep -q -- '--public-url uses HTTP port 18082 but the selected --http-port is 18081' "$TEMP_ROOT/output"

HTTPS_PROXY_DIR="$TEMP_ROOT/https-proxy-port"
env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_DIR="$HTTPS_PROXY_DIR" sh "$REPOSITORY_ROOT/install.sh" --http-port 18081 --public-url https://silicon.example.com >/dev/null
grep -q '^SILICON_HTTP_PORT=18081$' "$HTTPS_PROXY_DIR/config/silicon.env"
grep -q '^SILICON_PUBLIC_URL=https://silicon.example.com$' "$HTTPS_PROXY_DIR/config/silicon.env"

INTERACTIVE_DIR="$TEMP_ROOT/interactive-port"
printf '\n' | env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_TEST_INTERACTIVE=true SILICON_INSTALL_TEST_PORTS_IN_USE=80 SILICON_INSTALL_DIR="$INTERACTIVE_DIR" sh "$REPOSITORY_ROOT/install.sh" >/dev/null
grep -q '^SILICON_HTTP_PORT=8080$' "$INTERACTIVE_DIR/config/silicon.env"
grep -q '^SILICON_PUBLIC_URL=http://localhost:8080$' "$INTERACTIVE_DIR/config/silicon.env"

PRESERVED_PORT_DIR="$TEMP_ROOT/preserved-port"
env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_DIR="$PRESERVED_PORT_DIR" sh "$REPOSITORY_ROOT/install.sh" --http-port 18082 >/dev/null
PRESERVED_PORT_HASH=$(sha256sum "$PRESERVED_PORT_DIR/config/silicon.env" | cut -d ' ' -f 1)
env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_DIR="$PRESERVED_PORT_DIR" sh "$REPOSITORY_ROOT/install.sh" >/dev/null
[ "$PRESERVED_PORT_HASH" = "$(sha256sum "$PRESERVED_PORT_DIR/config/silicon.env" | cut -d ' ' -f 1)" ] || { printf 'repeat install did not preserve the configured port\n' >&2; exit 1; }
grep -q '^SILICON_HTTP_PORT=18082$' "$PRESERVED_PORT_DIR/config/silicon.env"
grep -q '^SILICON_PUBLIC_URL=http://localhost:18082$' "$PRESERVED_PORT_DIR/config/silicon.env"
expect_failure 'repeat install port override' env SILICON_INSTALL_TEST_MODE=true SILICON_INSTALL_DIR="$PRESERVED_PORT_DIR" sh "$REPOSITORY_ROOT/install.sh" --http-port 19000
grep -q 'existing installation uses HTTP port 18082' "$TEMP_ROOT/output"
[ "$PRESERVED_PORT_HASH" = "$(sha256sum "$PRESERVED_PORT_DIR/config/silicon.env" | cut -d ' ' -f 1)" ] || { printf 'rejected port override changed existing configuration\n' >&2; exit 1; }

expect_failure 'missing Docker' env PATH="$TEST_PATH" FAKE_DOCKER_MODE=missing SILICON_INSTALL_DIR="$TEMP_ROOT/missing-docker" sh "$REPOSITORY_ROOT/install.sh"
expect_failure 'missing Compose' env PATH="$TEST_PATH" FAKE_DOCKER_MODE=missing-compose SILICON_INSTALL_DIR="$TEMP_ROOT/missing-compose" sh "$REPOSITORY_ROOT/install.sh"

INSTALL_DIR="$TEMP_ROOT/fresh"
env PATH="$TEST_PATH" SILICON_INSTALL_DIR="$INSTALL_DIR" SILICON_PUBLIC_URL=http://localhost:18083 SILICON_HTTP_PORT=18083 SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh" >/dev/null
ENV_FILE="$INSTALL_DIR/config/silicon.env"
[ -s "$ENV_FILE" ] || { printf 'fresh configuration was not generated\n' >&2; exit 1; }
BEFORE=$(sha256sum "$ENV_FILE" | cut -d ' ' -f 1)
printf '\nCUSTOM_SETTING=preserve-me\n' >> "$ENV_FILE"
PRESERVED=$(sha256sum "$ENV_FILE" | cut -d ' ' -f 1)
env PATH="$TEST_PATH" SILICON_INSTALL_DIR="$INSTALL_DIR" SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh" >/dev/null
AFTER=$(sha256sum "$ENV_FILE" | cut -d ' ' -f 1)
[ "$PRESERVED" = "$AFTER" ] || { printf 'repeat installation overwrote configuration\n' >&2; exit 1; }
[ "$BEFORE" != "$AFTER" ] || { printf 'test fixture did not change configuration\n' >&2; exit 1; }
grep -q '^CUSTOM_SETTING=preserve-me$' "$ENV_FILE"

# The panel runner uses the release-only installer path. It must select the
# exact tag and preserve both configuration bytes and persistent data.
mkdir -p "$INSTALL_DIR/data/postgres"
printf '%s\n' 'persistent-database-marker' > "$INSTALL_DIR/data/postgres/user-data"
UPDATE_CONFIG_HASH=$(sha256sum "$ENV_FILE" | cut -d ' ' -f 1)
FAKE_GIT_LOG="$TEMP_ROOT/git-update.log"
env PATH="$TEST_PATH" FAKE_GIT_LOG="$FAKE_GIT_LOG" SILICON_REQUIRE_TAGGED_RELEASE=true SILICON_UPDATE_RUNNER=true SILICON_INSTALL_DIR="$INSTALL_DIR" SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh" --update --version v0.4.2 >/dev/null
[ "$UPDATE_CONFIG_HASH" = "$(sha256sum "$ENV_FILE" | cut -d ' ' -f 1)" ] || { printf 'tagged update changed silicon.env\n' >&2; exit 1; }
grep -q '^persistent-database-marker$' "$INSTALL_DIR/data/postgres/user-data"
grep -Fq 'checkout --detach refs/tags/v0.4.2' "$FAKE_GIT_LOG"
expect_failure 'release-only update from main' env PATH="$TEST_PATH" SILICON_REQUIRE_TAGGED_RELEASE=true SILICON_INSTALL_DIR="$INSTALL_DIR" sh "$REPOSITORY_ROOT/install.sh" --update --version main
grep -q 'exact stable semantic tag' "$TEMP_ROOT/output"

expect_failure 'startup failure' env PATH="$TEST_PATH" FAKE_DOCKER_MODE=startup SILICON_HTTP_PORT=18084 SILICON_INSTALL_DIR="$TEMP_ROOT/startup" SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh"
expect_failure 'readiness failure' env PATH="$TEST_PATH" FAKE_CURL_MODE=fail SILICON_HTTP_PORT=18085 SILICON_INSTALL_DIR="$TEMP_ROOT/readiness" SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh"

printf 'installer tests passed\n'
