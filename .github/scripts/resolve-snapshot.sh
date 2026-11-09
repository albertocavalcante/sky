#!/usr/bin/env bash
# Copyright 2026 Alberto Cavalcante. See LICENSE for licensing terms.
# Select data only. Authentication and execution belong to dependent jobs.
set -euo pipefail
fail() {
  echo "Snapshot refused: $*" >&2
  exit 1
}
[[ $# == 1 ]] || fail 'repository required'
[[ ${GITHUB_REPOSITORY:-} == albertocavalcante/sky ]] || fail 'wrong repository'
[[ ${GITHUB_EVENT_NAME:-} == push || ${GITHUB_EVENT_NAME:-} == workflow_dispatch ]] || fail 'wrong event'
[[ ${GITHUB_REF:-} == refs/heads/main && ${GITHUB_REF_PROTECTED:-} == true ]] || fail 'protected main required'
[[ ${GITHUB_WORKFLOW_REF:-} == albertocavalcante/sky/.github/workflows/release.yml@refs/heads/main ]] || fail 'trusted workflow required'
[[ ${GITHUB_SHA:-} =~ ^[0-9a-f]{40}$ ]] || fail 'invalid captured main SHA'

repo=$1
unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_CONFIG_COUNT GIT_CONFIG_PARAMETERS
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_ALLOW_PROTOCOL=file GIT_NO_LAZY_FETCH=1
read_git() { git --no-replace-objects -C "$repo" -c core.hooksPath=/dev/null "$@"; }
[[ $(read_git cat-file -t "$GITHUB_SHA") == commit ]] || fail 'captured main is not a commit'
[[ $(read_git rev-parse HEAD) == "$GITHUB_SHA" ]] || fail 'checkout differs from captured main'
selected=${INPUT_REF:-main}
if [[ "$selected" == main ]]; then selected=$GITHUB_SHA; fi
[[ "$selected" =~ ^[0-9a-f]{40}$ ]] || fail 'use main or a full lowercase commit SHA'
[[ $(read_git cat-file -t "$selected") == commit ]] || fail 'selected object is not a commit'
read_git merge-base --is-ancestor "$selected" "$GITHUB_SHA" || fail 'selected commit is outside captured main history'
printf '%s\n' "$selected"
