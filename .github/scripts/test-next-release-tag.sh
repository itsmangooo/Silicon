#!/bin/sh
set -eu

SCRIPT_DIR=$(cd -- "$(dirname "$0")" && pwd)
TEMP_ROOT=$(mktemp -d)
trap 'rm -rf "$TEMP_ROOT"' EXIT HUP INT TERM
REPOSITORY="$TEMP_ROOT/repository"
SERIES_FILE="$TEMP_ROOT/release-series"

git init -q "$REPOSITORY"
git -C "$REPOSITORY" config user.name 'Silicon Release Test'
git -C "$REPOSITORY" config user.email 'release-test@example.invalid'
printf 'fixture\n' > "$REPOSITORY/fixture"
git -C "$REPOSITORY" add fixture
git -C "$REPOSITORY" commit -qm 'fixture'

next_tag() {
  SILICON_REPOSITORY_ROOT="$REPOSITORY" SILICON_RELEASE_SERIES_FILE="$SERIES_FILE" sh "$SCRIPT_DIR/next-release-tag.sh"
}

printf '0.1\n' > "$SERIES_FILE"
[ "$(next_tag)" = 'v0.1.0' ] || { printf 'first release is not v0.1.0\n' >&2; exit 1; }
git -C "$REPOSITORY" tag v0.1.0
git -C "$REPOSITORY" tag v0.1.2
git -C "$REPOSITORY" tag v0.1.9-beta.1
git -C "$REPOSITORY" tag v0.2.7
[ "$(next_tag)" = 'v0.1.3' ] || { printf 'patch release did not increment the current series\n' >&2; exit 1; }

printf '0.2\n' > "$SERIES_FILE"
[ "$(next_tag)" = 'v0.2.8' ] || { printf 'deliberate minor series did not preserve its patch history\n' >&2; exit 1; }

printf 'main\n' > "$SERIES_FILE"
if next_tag > /dev/null 2>&1; then
  printf 'invalid release series was accepted\n' >&2
  exit 1
fi

printf 'automatic release tag tests passed\n'
