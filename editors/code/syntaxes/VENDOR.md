# Vendored Starlark Grammar

This directory contains TextMate grammar files vendored from the
[vscode-bazel](https://github.com/bazelbuild/vscode-bazel) extension.

## Source

- **Repository**: https://github.com/bazelbuild/vscode-bazel
- **Ref**: master
- **Vendored**: 2026-02-05
- **Tool**: go run ./tools/vendor-grammar

## Files

| File                        | License           | Description                                 |
| --------------------------- | ----------------- | ------------------------------------------- |
| starlark.tmLanguage.json    | MIT (MagicPython) | TextMate grammar for Starlark               |
| starlark.tmLanguage.license | -                 | License for the grammar file                |
| starlark.configuration.json | Apache 2.0        | Language configuration (brackets, comments) |
| bazelrc.tmLanguage.yaml     | Apache 2.0        | TextMate grammar for .bazelrc files         |
| bazelrc.configuration.json  | Apache 2.0        | Language configuration for .bazelrc         |

## Licenses

### starlark.tmLanguage.json

The Starlark grammar is derived from MagicPython and is licensed under the
**MIT License**. See starlark.tmLanguage.license for the full license text.

### Other files

Other files are from the vscode-bazel project and are licensed under the
**Apache License 2.0**.

## Updating

To update the vendored files:

```bash
go run ./tools/vendor-grammar -ref master
```

To vendor from a specific tag or commit:

```bash
go run ./tools/vendor-grammar -ref v0.10.0
```

## Local Modifications

If you make local modifications to these files, document them here:

- (none yet)

## Attribution

The Starlark TextMate grammar is derived from:

- [MagicPython](https://github.com/MagicStack/MagicPython) - MIT License
- [vscode-bazel](https://github.com/bazelbuild/vscode-bazel) - Apache 2.0
