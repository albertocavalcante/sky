#!/usr/bin/env bash
# Copyright 2026 Alberto Cavalcante.
# See LICENSE for licensing terms.
set -euo pipefail
umask 077

sky_action_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
sky_installer=${1:-$sky_action_dir/install.sh}
sky_test=$(mktemp -d "${TMPDIR:-/tmp}/sky-go-test.XXXXXX")
trap 'rm -rf -- "$sky_test"' EXIT
mkdir -p "$sky_test/stubs" "$sky_test/payload/go/bin"
export SKY_TEST_REAL_TAR
SKY_TEST_REAL_TAR=$(command -v tar)

cat >"$sky_test/payload/go/bin/go" <<'EOF'
#!/usr/bin/env bash
printf 'executed\n' >>"$SKY_TEST_EXECUTED"
printf 'go version go1.26.9 linux/%s\n' "$SKY_TEST_ARCH"
EOF
chmod 755 "$sky_test/payload/go/bin/go"
"$SKY_TEST_REAL_TAR" -czf "$sky_test/sdk.tar.gz" -C "$sky_test/payload" go
export SKY_TEST_ARCHIVE="$sky_test/sdk.tar.gz"

cat >"$sky_test/stubs/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
sky_output=
sky_url=
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      sky_output=$2
      shift 2
      ;;
    https://*)
      sky_url=$1
      shift
      ;;
    *) shift ;;
  esac
done
[[ $sky_url == "https://go.dev/dl/go1.26.9.linux-$SKY_TEST_ARCH.tar.gz" ]]
[[ -n $sky_output ]]
cp "$SKY_TEST_ARCHIVE" "$sky_output"
printf 'provided valid test archive\n' >>"$SKY_TEST_DOWNLOADED"
EOF
cat >"$sky_test/stubs/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'extracted\n' >>"$SKY_TEST_EXTRACTED"
exec "$SKY_TEST_REAL_TAR" "$@"
EOF
chmod 755 "$sky_test/stubs/curl" "$sky_test/stubs/tar"

# Fall back to shasum only where sha256sum is absent (for example, older macOS).
if ! command -v sha256sum >/dev/null; then
  cat >"$sky_test/stubs/sha256sum" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exec shasum -a 256 "$@"
EOF
  chmod 755 "$sky_test/stubs/sha256sum"
fi
export PATH="$sky_test/stubs:$PATH"

# Prove the checksum tool accepts an actual matching digest, then rejects a
# mismatch. Otherwise a missing/broken checksum tool could make rejection pass.
sky_fixture_sha=$(sha256sum "$SKY_TEST_ARCHIVE")
printf '%s\n' "$sky_fixture_sha" | sha256sum -c - >/dev/null
if printf '%064d  %s\n' 0 "$SKY_TEST_ARCHIVE" | sha256sum -c - >/dev/null 2>&1; then
  echo "checksum control accepted an incorrect digest" >&2
  exit 1
fi

for sky_runner_arch in ARM64 X64; do
  case "$sky_runner_arch" in ARM64) export SKY_TEST_ARCH=arm64 ;; X64) export SKY_TEST_ARCH=amd64 ;; esac
  sky_case="$sky_test/$SKY_TEST_ARCH"
  mkdir "$sky_case"
  export SKY_TEST_EXECUTED="$sky_case/executed"
  export SKY_TEST_EXTRACTED="$sky_case/extracted"
  export SKY_TEST_DOWNLOADED="$sky_case/downloaded"
  export GITHUB_PATH="$sky_case/github-path" GITHUB_ENV="$sky_case/github-env"
  : >"$GITHUB_PATH"
  : >"$GITHUB_ENV"
  if RUNNER_OS=Linux RUNNER_ARCH="$sky_runner_arch" \
    bash "$sky_installer" "$sky_case/sdk parent" >"$sky_case/install.log" 2>&1; then
    cat "$sky_case/install.log" >&2
    echo "installer accepted a valid archive with an untrusted checksum ($sky_runner_arch)" >&2
    exit 1
  fi
  [[ -s $SKY_TEST_DOWNLOADED ]]
  [[ ! -e $SKY_TEST_EXTRACTED && ! -e $SKY_TEST_EXECUTED ]]
  [[ ! -s $GITHUB_PATH && ! -s $GITHUB_ENV ]]
  [[ -z $(find "$sky_case/sdk parent" -mindepth 1 -print -quit) ]]
  printf 'PASS %s: checksum mismatch rejected before extraction/execution; no exports or retained SDK\n' "$sky_runner_arch"
done
