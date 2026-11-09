#!/usr/bin/env bash
# Copyright 2026 Alberto Cavalcante.
# See LICENSE for licensing terms.
set -euo pipefail
umask 077

if [[ $# -ne 1 || -z $1 ]]; then
  echo "usage: install.sh PARENT_DIRECTORY" >&2
  exit 1
fi
if [[ ${RUNNER_OS:-} != Linux ]]; then
  echo "verified Go installer requires Linux" >&2
  exit 1
fi
# SHA256 pins are from https://go.dev/dl/?mode=json&include=all.
case "${RUNNER_ARCH:-}" in
  ARM64)
    sky_arch=arm64
    sky_sha=4a97373d49fcacdcf3694fea368a500b00ee3e963974f3e7514132717632f052
    ;;
  X64)
    sky_arch=amd64
    sky_sha=42d158b4d8f7b61ac0a830567c940a86098fb7aac52e467a5ebec03ef5cc2f8d
    ;;
  *)
    echo "unsupported runner architecture: ${RUNNER_ARCH:-unset}" >&2
    exit 1
    ;;
esac
: "${GITHUB_PATH:?GITHUB_PATH is required}"
: "${GITHUB_ENV:?GITHUB_ENV is required}"
sky_parent=$1
case "$sky_parent" in
  *$'\n'* | *$'\r'*)
    echo "SDK parent directory must not contain newlines" >&2
    exit 1
    ;;
esac
mkdir -p -- "$sky_parent"
sky_parent=$(cd -- "$sky_parent" && pwd -P)
sky_install=$(mktemp -d "$sky_parent/sky-go.XXXXXX")
sky_keep=false
cleanup() {
  if [[ $sky_keep != true ]]; then
    rm -rf -- "$sky_install"
  fi
}
trap cleanup EXIT

curl --fail --silent --show-error --location \
  "https://go.dev/dl/go1.26.9.linux-$sky_arch.tar.gz" \
  --output "$sky_install/sdk.tar.gz"
printf '%s  %s\n' "$sky_sha" "$sky_install/sdk.tar.gz" | sha256sum -c - >/dev/null

tar -xzf "$sky_install/sdk.tar.gz" -C "$sky_install"
export GOROOT="$sky_install/go" GOTOOLCHAIN=local
sky_version=$("$GOROOT/bin/go" version)
if [[ $sky_version != "go version go1.26.9 linux/$sky_arch" ]]; then
  echo "unexpected Go version: $sky_version" >&2
  exit 1
fi
rm "$sky_install/sdk.tar.gz"
printf '%s/bin\n' "$GOROOT" >>"$GITHUB_PATH"
printf 'GOROOT=%s\nGOTOOLCHAIN=local\n' "$GOROOT" >>"$GITHUB_ENV"
sky_keep=true
printf '%s\n' "$sky_version"
