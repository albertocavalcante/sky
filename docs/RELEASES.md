# Snapshot Releases

Sky does not use release tags yet. A build is identified by commit metadata:

```text
v0.0.0-YYYYMMDDHHMMSS-<commit12>
```

- `YYYYMMDDHHMMSS` is the UTC commit timestamp.
- `<commit12>` is the first 12 characters of the commit hash.
- The full commit hash and commit timestamp are embedded in binaries.

This keeps distribution simple: no mutable labels and no release promotion
state.

## Building Locally

```bash
version="$(go run ./tools/release)"
ldflags="$(go run ./tools/release ldflags)"
go build -trimpath -ldflags="-s -w ${ldflags}" -o dist/sky ./cmd/sky
./dist/sky version
```

## GitHub Artifacts

The Snapshot workflow builds binaries for Linux, macOS, and Windows. It attaches
artifacts to the workflow run. It does not create tags or GitHub Releases.
