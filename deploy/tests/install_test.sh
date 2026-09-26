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
if [ "$1" = "clone" ]; then
  for argument in "$@"; do destination=$argument; done
  mkdir -p "$destination/.git"
  : > "$destination/docker-compose.production.yml"
fi
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
expect_failure 'missing Docker' env PATH="$TEST_PATH" FAKE_DOCKER_MODE=missing SILICON_INSTALL_DIR="$TEMP_ROOT/missing-docker" sh "$REPOSITORY_ROOT/install.sh"
expect_failure 'missing Compose' env PATH="$TEST_PATH" FAKE_DOCKER_MODE=missing-compose SILICON_INSTALL_DIR="$TEMP_ROOT/missing-compose" sh "$REPOSITORY_ROOT/install.sh"

INSTALL_DIR="$TEMP_ROOT/fresh"
env PATH="$TEST_PATH" SILICON_INSTALL_DIR="$INSTALL_DIR" SILICON_PUBLIC_URL=http://localhost SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh" >/dev/null
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

expect_failure 'startup failure' env PATH="$TEST_PATH" FAKE_DOCKER_MODE=startup SILICON_INSTALL_DIR="$TEMP_ROOT/startup" SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh"
expect_failure 'readiness failure' env PATH="$TEST_PATH" FAKE_CURL_MODE=fail SILICON_INSTALL_DIR="$TEMP_ROOT/readiness" SILICON_READINESS_ATTEMPTS=1 sh "$REPOSITORY_ROOT/install.sh"

printf 'installer tests passed\n'
