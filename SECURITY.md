# Security Policy

Sky is pre-1.0 software. Plugins, LSP features, and coverage instrumentation are
still changing.

## Reporting

Report suspected vulnerabilities privately with a GitHub security advisory. If
advisories are unavailable, contact the maintainer privately.

Include:

- Affected commit hash or snapshot version.
- Reproduction steps.
- Expected and actual behavior.
- Any logs, inputs, or plugin manifests needed to reproduce.

## Supported Versions

Until tagged releases exist, support is based on commit hashes. Fixes target
`main`; users should pin known-good commits.

## Plugin Risk

Native plugins run with the user's permissions. Install plugins only from
trusted sources and prefer checksum-pinned installs. Review WASI plugins too.
