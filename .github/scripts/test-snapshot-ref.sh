#!/usr/bin/env bash
# Copyright 2026 Alberto Cavalcante. See LICENSE for licensing terms.
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$0")" && pwd)
resolver=${RESOLVER:-$script_dir/resolve-snapshot.sh}
fixture_dir=$(mktemp -d)
trap 'rm -rf "$fixture_dir"' EXIT
repo=$fixture_dir/repo
mkdir "$repo"
git init -q -b main "$repo"
git -C "$repo" config user.name 'Snapshot fixture'
git -C "$repo" config user.email 'fixture@example.invalid'
git -C "$repo" -c core.hooksPath=/dev/null -c commit.gpgsign=false commit -qm initial --allow-empty
ancestor=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" -c core.hooksPath=/dev/null -c commit.gpgsign=false commit -qm trusted --allow-empty
trusted=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" -c tag.gpgsign=false tag -a annotated -m fixture
annotation=$(git -C "$repo" rev-parse annotated)
git -C "$repo" checkout -qb untrusted
git -C "$repo" -c core.hooksPath=/dev/null -c commit.gpgsign=false commit -qm unreviewed --allow-empty
untrusted=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" checkout -q --detach "$trusted"

invoke() {
  env GITHUB_REPOSITORY=albertocavalcante/sky GITHUB_EVENT_NAME=workflow_dispatch \
    GITHUB_REF=refs/heads/main GITHUB_REF_PROTECTED=true \
    GITHUB_WORKFLOW_REF=albertocavalcante/sky/.github/workflows/release.yml@refs/heads/main \
    GITHUB_SHA="$trusted" INPUT_REF=main "$@" "$resolver" "$repo"
}
accept() {
  local expected=$1
  shift
  local actual
  actual=$(invoke "$@")
  [[ "$actual" == "$expected" ]] || {
    echo "selected $actual, want $expected" >&2
    exit 1
  }
}
reject() {
  if invoke "$@" >"$fixture_dir/output" 2>"$fixture_dir/error"; then
    echo "unsafe snapshot input/context accepted: $*" >&2
    exit 1
  fi
  [[ ! -s "$fixture_dir/output" ]] || {
    echo 'rejection emitted a commit' >&2
    exit 1
  }
}

accept "$trusted"
accept "$trusted" INPUT_REF=
accept "$ancestor" INPUT_REF="$ancestor"
accept "$trusted" GITHUB_EVENT_NAME=push
reject GITHUB_REPOSITORY=someone/else
reject GITHUB_EVENT_NAME=pull_request
reject GITHUB_REF=refs/heads/untrusted
reject GITHUB_REF_PROTECTED=false
reject GITHUB_WORKFLOW_REF=albertocavalcante/sky/.github/workflows/other.yml@refs/heads/main
reject GITHUB_SHA=main
reject GITHUB_SHA="$annotation"
reject INPUT_REF=untrusted
reject INPUT_REF=annotated
reject INPUT_REF=HEAD
reject INPUT_REF="$trusted^"
reject INPUT_REF="${trusted^^}"
reject INPUT_REF=0000000000000000000000000000000000000000
reject INPUT_REF="$annotation"
reject INPUT_REF="$untrusted"
# Deliberately test literal shell syntax as data, with an absolute sentinel.
reject INPUT_REF="\$(touch '$fixture_dir/should-not-exist')"
[[ ! -e "$fixture_dir/should-not-exist" ]]
# Moving the main ref after the event must not change its captured selection.
git -C "$repo" update-ref refs/heads/main "$untrusted"
accept "$trusted"
# A later source checkout consumes the selected object, not the mutable branch.
selected=$(invoke INPUT_REF="$ancestor")
git -C "$repo" checkout -q --detach "$selected"
[[ $(git -C "$repo" rev-parse HEAD) == "$ancestor" ]]
echo 'Snapshot trust/selection fixtures pass.'
