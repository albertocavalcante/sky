# Bazel Setup

Sky uses Bazel with bzlmod enabled and Bazel 9.

## Version

- `.bazelversion` pins Bazel `9.0.0rc3`.
- `.bazelrc` enables bzlmod and isolated module extension usage.

## Formatting

Rules from `aspect_rules_lint` provide `format_multirun`:

```bash
bazel run //:format
bazel run //:format.check
```

## Linting

Go linting is handled via `nogo` from `rules_go` and runs automatically during
Bazel builds.

```bash
bazel build //...
```

To opt out of nogo for generated code, add `tags = ["no-nogo"]`.
