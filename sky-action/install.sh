#!/usr/bin/env bash
# Build skytest and sky-ci from a checkout of the requested sky ref and put
# them on PATH for later steps.
#
# `go install pkg@version` cannot be used: sky's go.mod has replace
# directives, which Go refuses outside the main module. Building inside a
# checkout (module context) honours them.
#
# Inputs (environment):
#   INPUT_VERSION       branch, tag or full commit SHA; empty = the action's own checkout
#   GITHUB_ACTION_PATH  <sky checkout>/sky-action (set by GitHub for composite actions)
#   GITHUB_PATH         file that receives the bin directory
#   RUNNER_TEMP         scratch directory (falls back to mktemp)
#   SKY_REPO_URL        override of the clone URL (tests)
set -euo pipefail

: "${GITHUB_ACTION_PATH:?GITHUB_ACTION_PATH is required}"
: "${GITHUB_PATH:?GITHUB_PATH is required}"

version="${INPUT_VERSION:-}"
repo_url="${SKY_REPO_URL:-https://github.com/albertocavalcante/sky}"
work="${RUNNER_TEMP:-$(mktemp -d)}"
bin_dir="$work/sky-bin"

if [ -z "$version" ]; then
  # The action's own checkout is already at the requested ref.
  src="$(cd "$GITHUB_ACTION_PATH/.." && pwd)"
  if [ ! -f "$src/go.mod" ]; then
    echo "error: no go.mod in $src; cannot build from the action checkout" >&2
    exit 1
  fi
else
  src="$work/sky-src"
  rm -rf "$src"
  git init --quiet "$src"
  git -C "$src" remote add origin "$repo_url"
  if ! git -C "$src" fetch --quiet --depth 1 origin "$version"; then
    echo "error: cannot fetch ref '$version' from $repo_url (use a branch, tag or full commit SHA)" >&2
    exit 1
  fi
  git -C "$src" -c advice.detachedHead=false checkout --quiet FETCH_HEAD
fi

mkdir -p "$bin_dir"
# GOTOOLCHAIN=auto lets a ref whose go.mod needs a newer Go than the
# bootstrap toolchain download the right one.
(cd "$src" && GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" go build -o "$bin_dir/" ./cmd/skytest ./cmd/sky-ci)

path_entry="$bin_dir"
if command -v cygpath >/dev/null 2>&1; then
  path_entry="$(cygpath -w "$bin_dir")"
fi
echo "$path_entry" >>"$GITHUB_PATH"
echo "Installed skytest and sky-ci from ${version:-the action checkout} into $bin_dir"
