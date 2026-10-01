#!/bin/sh
set -eu

REPOSITORY_ROOT=${SILICON_REPOSITORY_ROOT:-$(cd -- "$(dirname "$0")/../.." && pwd)}
SERIES_FILE=${SILICON_RELEASE_SERIES_FILE:-$REPOSITORY_ROOT/.github/release-series}

[ -f "$SERIES_FILE" ] || { printf 'release series file is missing: %s\n' "$SERIES_FILE" >&2; exit 1; }
SERIES=$(tr -d '[:space:]' < "$SERIES_FILE")
printf '%s\n' "$SERIES" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' || {
  printf 'release series must contain MAJOR.MINOR, for example 0.1\n' >&2
  exit 1
}

PREFIX="v$SERIES."
MAX_PATCH=-1
for TAG in $(git -C "$REPOSITORY_ROOT" tag --list "$PREFIX*"); do
  PATCH=${TAG#"$PREFIX"}
  printf '%s\n' "$PATCH" | grep -Eq '^(0|[1-9][0-9]*)$' || continue
  if [ "$PATCH" -gt "$MAX_PATCH" ]; then MAX_PATCH=$PATCH; fi
done

printf '%s%s\n' "$PREFIX" "$((MAX_PATCH + 1))"
