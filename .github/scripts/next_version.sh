#!/usr/bin/env bash
set -euo pipefail

# Determine the next semantic version based on conventional commits since the last tag.
# Tags are expected to be in the form vMAJOR.MINOR.PATCH.

LAST_TAG="$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)"

# Collect commit messages since LAST_TAG (exclude merge commits for clarity)
LOG_RANGE="${LAST_TAG}..HEAD"
COMMITS=$(git log --no-merges --pretty=format:%H "$LOG_RANGE" || true)

if [ -z "$COMMITS" ]; then
  # No new commits => no version bump
  NEXT_VERSION="$LAST_TAG"
else
  RAW_LOG=$(git log --no-merges --pretty=format:'%s%n%b%n==END==' "$LOG_RANGE")
  MAJOR_BUMP=0
  MINOR_BUMP=0
  PATCH_BUMP=0

  # Detect breaking changes ("!" in type scope or BREAKING CHANGE footer)
  if echo "$RAW_LOG" | grep -Eq '(^|\n)(feat|fix|perf|refactor|chore|docs|style|test)(\([^)]*\))?!:'; then
    MAJOR_BUMP=1
  elif echo "$RAW_LOG" | grep -Eq 'BREAKING CHANGE'; then
    MAJOR_BUMP=1
  fi

  # Detect features
  if [ "$MAJOR_BUMP" -eq 0 ] && echo "$RAW_LOG" | grep -Eq '(^|\n)feat(\(|:)' ; then
    MINOR_BUMP=1
  fi

  # Detect fixes/perf/refactor for patch (only if not major/minor)
  if [ "$MAJOR_BUMP" -eq 0 ] && [ "$MINOR_BUMP" -eq 0 ] && echo "$RAW_LOG" | grep -Eq '(^|\n)(fix|perf|refactor)(\(|:)' ; then
    PATCH_BUMP=1
  fi

  if [ $MAJOR_BUMP -eq 0 ] && [ $MINOR_BUMP -eq 0 ] && [ $PATCH_BUMP -eq 0 ]; then
    # No conventional commit types found => keep version
    NEXT_VERSION="$LAST_TAG"
  else
    # Parse current version numbers
    CUR="${LAST_TAG#v}"
    MAJOR=${CUR%%.*}
    REST=${CUR#*.}
    MINOR=${REST%%.*}
    PATCH=${CUR##*.}
    if [ $MAJOR_BUMP -eq 1 ]; then
      MAJOR=$((MAJOR+1)); MINOR=0; PATCH=0
    elif [ $MINOR_BUMP -eq 1 ]; then
      MINOR=$((MINOR+1)); PATCH=0
    else
      PATCH=$((PATCH+1))
    fi
    NEXT_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
  fi
fi

echo "$LAST_TAG" > LAST_TAG
echo "$NEXT_VERSION" > NEXT_VERSION
echo "Last tag: $LAST_TAG" >&2
echo "Next version: $NEXT_VERSION" >&2
