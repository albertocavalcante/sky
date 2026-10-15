#!/usr/bin/env bash
# Offline tests for install.sh. Run: bash sky-action/install_test.sh
# Uses a fixture module that, like sky, has a replace directive.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
install_sh="$here/install.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failures=0
fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com
export GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export GOFLAGS=-mod=mod GOPROXY=off GOTOOLCHAIN=local

# make_repo DIR MARKER: module with a replace directive and two commands.
make_repo() {
  local dir="$1" marker="$2"
  mkdir -p "$dir/cmd/skytest" "$dir/cmd/sky-ci" "$dir/sky-action" "$dir/dep"
  printf 'module example.com/dep\n\ngo 1.21\n' >"$dir/dep/go.mod"
  printf 'package dep\n\nconst Marker = "%s"\n' "$marker" >"$dir/dep/dep.go"
  cat >"$dir/go.mod" <<MOD
module example.com/sky

go 1.21

require example.com/dep v0.0.0

replace example.com/dep => ./dep
MOD
  local c
  for c in skytest sky-ci; do
    cat >"$dir/cmd/$c/main.go" <<GO
package main

import (
	"fmt"

	"example.com/dep"
)

func main() { fmt.Println("$c", dep.Marker) }
GO
  done
}

origin="$tmp/origin"
make_repo "$origin" v1
git init --quiet -b main "$origin"
git -C "$origin" config uploadpack.allowAnySHA1InWant true
git -C "$origin" add dep cmd go.mod
git -C "$origin" commit --quiet -m one
sha1="$(git -C "$origin" rev-parse HEAD)"
git -C "$origin" tag rel-1
printf 'package dep\n\nconst Marker = "v2"\n' >"$origin/dep/dep.go"
git -C "$origin" commit --quiet -am two

# run_install NAME VERSION ACTION_PATH [URL]: sets out, gh_path, rt_dir, status.
run_install() {
  local name="$1" version="$2" action_path="$3" url="${4:-file://$origin}"
  rt_dir="$tmp/rt-$name"
  mkdir -p "$rt_dir"
  gh_path="$rt_dir/path"
  : >"$gh_path"
  status=0
  out="$(INPUT_VERSION="$version" GITHUB_ACTION_PATH="$action_path" \
    GITHUB_PATH="$gh_path" RUNNER_TEMP="$rt_dir" SKY_REPO_URL="$url" \
    bash "$install_sh" 2>&1)" || status=$?
}

# 1. Default version builds from the local checkout (never touches the URL).
local_checkout="$tmp/local"
make_repo "$local_checkout" local
run_install default "" "$local_checkout/sky-action" file:///nonexistent
if [ "$status" -ne 0 ]; then
  fail "default: exit $status: $out"
elif [ "$("$rt_dir/sky-bin/skytest")" != "skytest local" ]; then
  fail "default: skytest not built from local checkout"
elif [ "$(grep -c . "$gh_path")" -ne 1 ] || ! grep -q 'sky-bin' "$gh_path"; then
  fail "default: bin dir missing from GITHUB_PATH: $(cat "$gh_path")"
fi

# 2. Explicit branch clones that ref (tip of main is the v2 commit).
run_install branch main "$tmp/unused/sky-action"
if [ "$status" -ne 0 ]; then
  fail "branch: exit $status: $out"
elif [ "$("$rt_dir/sky-bin/sky-ci")" != "sky-ci v2" ]; then
  fail "branch: sky-ci not built from main"
fi

# 3. Explicit tag and full commit SHA resolve to the older commit.
for ref in rel-1 "$sha1"; do
  run_install ref "$ref" "$tmp/unused/sky-action"
  if [ "$status" -ne 0 ]; then
    fail "ref $ref: exit $status: $out"
  elif [ "$("$rt_dir/sky-bin/skytest")" != "skytest v1" ]; then
    fail "ref $ref: wrong content"
  fi
done

# 4. Unknown ref fails with a clear message and adds nothing to PATH.
run_install bad does-not-exist "$tmp/unused/sky-action"
if [ "$status" -eq 0 ]; then
  fail "unknown ref: expected failure"
elif ! grep -q "cannot fetch ref 'does-not-exist'" <<<"$out"; then
  fail "unknown ref: unclear message: $out"
elif [ -s "$gh_path" ]; then
  fail "unknown ref: GITHUB_PATH was written"
fi

if [ "$failures" -ne 0 ]; then
  echo "$failures failure(s)" >&2
  exit 1
fi
echo "ok"
