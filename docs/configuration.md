# Configuration

Sky tools support unified configuration through project-level config files. This allows you to define consistent settings for `skytest`, `skylint`, and other sky tools without repeating flags on every invocation.

## Configuration Formats

Sky supports two configuration formats:

| Format       | File                                            | Best For                           |
| ------------ | ----------------------------------------------- | ---------------------------------- |
| **TOML**     | `sky.toml`                                      | Simple, static configuration       |
| **Starlark** | `config.sky` (canonical) or `sky.star` (legacy) | Dynamic, conditional configuration |

**When to use which:**

- Use **sky.toml** when your configuration is straightforward and doesn't change based on environment (local dev, CI, etc.).
- Use **config.sky** when you need conditional logic based on environment variables, OS, or architecture.

> **Note:** Only one config file may exist per directory. If multiple config files are found (e.g., both `config.sky` and `sky.toml`), sky will report an error.

## Quick Start

Create a `sky.toml` in your project root:

```toml
[test]
timeout = "60s"
parallel = "auto"
prelude = ["test/helpers.star"]
```

Or create a `config.sky` for dynamic configuration:

```python
def configure():
    ci = getenv("CI", "") != ""
    return {
        "test": {
            "timeout": "120s" if ci else "30s",
            "parallel": "1" if ci else "auto",
        },
    }
```

## Config File Discovery

Sky automatically discovers configuration files by walking up the directory tree from your current working directory. The search stops at the repository root (the directory containing `.git`).

**Discovery order:**

1. **CLI flag:** `--config=path/to/config.sky` (highest priority)
2. **Environment variable:** `SKY_CONFIG=/path/to/config.sky`
3. **Directory walk:** Starting from the current directory, moving up to the repository root

**File priority in each directory:**

1. `config.sky` (canonical Starlark config)
2. `sky.star` (legacy Starlark config)
3. `sky.toml` (TOML config)

If no config file is found, sensible defaults are used.

### Example Directory Structure

```
my-project/
├── .git/
├── config.sky          # <- Found and used
├── src/
│   └── lib.star
└── tests/
    └── lib_test.star   # Running skytest here uses ../config.sky
```

## CLI Usage

### Specifying a Config File

```bash
# Use a specific config file
skytest --config=ci-config.sky tests/

# Use a custom Starlark execution timeout (default: 5s)
skytest --config=config.sky --config-timeout=10s tests/
```

### Environment Variable

```bash
# Set config path via environment
export SKY_CONFIG=/path/to/config.sky
skytest tests/

# Override per-command
SKY_CONFIG=local.sky skytest tests/
```

### Precedence

CLI flags always override config file settings. The full precedence order is:

1. CLI flags (highest)
2. Config file settings
3. Built-in defaults (lowest)

## TOML Configuration Reference

### Complete Example

```toml
# sky.toml - Full configuration example

[test]
# Per-test timeout (Go duration format: "30s", "1m", "1h30m")
timeout = "60s"

# Parallelism: "auto" (use all CPUs), "1" (sequential), or a number
parallel = "auto"

# Prelude files loaded before each test file
prelude = ["test/helpers.star", "test/fixtures.star"]

# Test function prefix (default: "test_")
prefix = "test_"

# Stop on first test failure
fail_fast = false

# Enable verbose output
verbose = false

[test.coverage]
# Enable coverage collection (EXPERIMENTAL)
enabled = false

# Fail if coverage falls below this percentage
fail_under = 80.0

# Coverage output file path
output = "coverage.json"

[lint]
# Rules or categories to enable
enable = ["all"]

# Rules or patterns to disable
disable = ["native-*"]

# Treat warnings as errors
warnings_as_errors = false
```

### Test Configuration Options

| Option      | Type   | Default           | Description                            |
| ----------- | ------ | ----------------- | -------------------------------------- |
| `timeout`   | string | `"30s"`           | Per-test timeout in Go duration format |
| `parallel`  | string | `""` (sequential) | `"auto"`, `"1"`, or a specific number  |
| `prelude`   | list   | `[]`              | Prelude files to load before tests     |
| `prefix`    | string | `"test_"`         | Test function name prefix              |
| `fail_fast` | bool   | `false`           | Stop on first failure                  |
| `verbose`   | bool   | `false`           | Enable verbose output                  |

### Coverage Configuration Options

| Option       | Type   | Default           | Description                 |
| ------------ | ------ | ----------------- | --------------------------- |
| `enabled`    | bool   | `false`           | Enable coverage collection  |
| `fail_under` | float  | `0`               | Minimum coverage percentage |
| `output`     | string | `"coverage.json"` | Coverage output file path   |

### Lint Configuration Options

| Option               | Type | Default | Description                   |
| -------------------- | ---- | ------- | ----------------------------- |
| `enable`             | list | `[]`    | Rules or categories to enable |
| `disable`            | list | `[]`    | Rules or patterns to disable  |
| `warnings_as_errors` | bool | `false` | Treat warnings as errors      |

### Duration Format

Durations use Go's duration format:

| Unit     | Example   | Description              |
| -------- | --------- | ------------------------ |
| `s`      | `"30s"`   | Seconds                  |
| `m`      | `"5m"`    | Minutes                  |
| `h`      | `"1h"`    | Hours                    |
| Combined | `"1h30m"` | 1 hour and 30 minutes    |
| Combined | `"2m30s"` | 2 minutes and 30 seconds |

## Starlark Configuration Reference

Starlark config files must define a `configure()` function that returns a dictionary.

### Basic Structure

```python
def configure():
    return {
        "test": {
            "timeout": "60s",
            "parallel": "auto",
        },
        "lint": {
            "enable": ["all"],
        },
    }
```

### Available Builtins

Starlark config files have access to these predeclared values and functions:

| Builtin                    | Type     | Description                                     |
| -------------------------- | -------- | ----------------------------------------------- |
| `getenv(name, default="")` | function | Get environment variable value                  |
| `host_os`                  | string   | Current OS (`"darwin"`, `"linux"`, `"windows"`) |
| `host_arch`                | string   | Current architecture (`"amd64"`, `"arm64"`)     |
| `duration(s)`              | function | Validate and return a duration string           |
| `struct(**kwargs)`         | function | Create a dict from keyword arguments            |

### Builtin Reference

#### getenv(name, default="")

Returns the value of an environment variable, or the default if not set.

```python
def configure():
    ci = getenv("CI", "") != ""
    github_actions = getenv("GITHUB_ACTIONS", "") == "true"
    custom_timeout = getenv("TEST_TIMEOUT", "30s")

    return {
        "test": {
            "timeout": custom_timeout,
            "parallel": "1" if ci else "auto",
        },
    }
```

#### host_os

A string containing the current operating system. Values match Go's `runtime.GOOS`:

- `"darwin"` - macOS
- `"linux"` - Linux
- `"windows"` - Windows

```python
def configure():
    # Use more parallelism on Linux servers
    parallel = "auto"
    if host_os == "linux":
        parallel = "8"

    return {
        "test": {
            "parallel": parallel,
        },
    }
```

#### host_arch

A string containing the current CPU architecture. Values match Go's `runtime.GOARCH`:

- `"amd64"` - x86-64
- `"arm64"` - ARM64/Apple Silicon

```python
def configure():
    # Longer timeouts on emulated architectures
    timeout = "30s"
    if host_arch == "arm64" and host_os == "linux":
        timeout = "60s"  # Might be running under emulation

    return {
        "test": {
            "timeout": timeout,
        },
    }
```

#### duration(s)

Validates that a string is a valid Go duration and returns it. This is useful for catching typos early.

```python
def configure():
    # This will fail at config load time if the format is invalid
    timeout = duration("60s")

    return {
        "test": {
            "timeout": timeout,
        },
    }
```

#### struct(**kwargs)

Creates a dictionary from keyword arguments. Useful for readable nested configuration.

```python
def configure():
    return {
        "test": struct(
            timeout = "60s",
            parallel = "auto",
            coverage = struct(
                enabled = True,
                fail_under = 80,
            ),
        ),
    }
```

### Execution Environment

Starlark config files run in a **sandboxed environment**:

- No filesystem access (use `getenv` instead of reading files)
- No network access
- No module loading (`load()` is not available)
- 5-second execution timeout by default (configurable via `--config-timeout`)

This ensures config files load quickly and cannot cause security issues.

## Real-World Examples

### Basic TOML for Simple Projects

```toml
# sky.toml
[test]
timeout = "30s"
verbose = true
```

### CI/CD Conditional Logic

```python
# config.sky - Different settings for CI vs local development

def configure():
    ci = getenv("CI", "") != ""

    if ci:
        return {
            "test": {
                # Longer timeout for CI (cold caches, shared resources)
                "timeout": "120s",
                # Sequential execution for deterministic results
                "parallel": "1",
                # Fail fast to save CI minutes
                "fail_fast": True,
                # Coverage required in CI
                "coverage": {
                    "enabled": True,
                    "fail_under": 80,
                },
            },
        }
    else:
        return {
            "test": {
                # Shorter timeout for local dev
                "timeout": "30s",
                # Use all cores locally
                "parallel": "auto",
                # See all failures at once
                "fail_fast": False,
            },
        }
```

### OS-Specific Settings

```python
# config.sky - Platform-specific configuration

def configure():
    timeout = "30s"
    parallel = "auto"

    # Windows often needs longer timeouts
    if host_os == "windows":
        timeout = "60s"
        parallel = "4"  # Windows handles fewer parallel processes well

    # macOS with Apple Silicon is fast
    if host_os == "darwin" and host_arch == "arm64":
        timeout = "15s"
        parallel = "auto"

    return {
        "test": {
            "timeout": timeout,
            "parallel": parallel,
        },
    }
```

### Shared Prelude with Environment-Specific Overrides

```python
# config.sky - Common prelude with environment tweaks

def configure():
    # Common preludes for all environments
    preludes = [
        "test/helpers.star",
        "test/fixtures.star",
    ]

    # Add mock prelude in CI to avoid external dependencies
    if getenv("CI", "") != "":
        preludes.append("test/mocks.star")

    return {
        "test": {
            "prelude": preludes,
            "timeout": getenv("TEST_TIMEOUT", "30s"),
        },
    }
```

### Monorepo with Team-Specific Settings

```python
# config.sky - Different settings based on team conventions

def configure():
    team = getenv("TEAM", "default")

    configs = {
        "platform": {
            "timeout": "120s",
            "prefix": "test_",
            "fail_fast": False,
        },
        "frontend": {
            "timeout": "30s",
            "prefix": "spec_",  # Different naming convention
            "verbose": True,
        },
        "default": {
            "timeout": "60s",
            "prefix": "test_",
        },
    }

    return {
        "test": configs.get(team, configs["default"]),
    }
```

## Troubleshooting

### Multiple Config Files Error

**Error:** `multiple config files found in the same directory; use only one`

**Cause:** You have more than one of `config.sky`, `sky.star`, or `sky.toml` in the same directory.

**Solution:** Remove the extra config files. If migrating from `sky.star` to `config.sky`, delete the old file.

```bash
# Check for config files
ls config.sky sky.star sky.toml 2>/dev/null
```

### Config Timeout Error

**Error:** `execution timeout` when loading Starlark config

**Cause:** Your `configure()` function is taking too long, possibly due to an infinite loop.

**Solution:**

1. Check for infinite loops in your config
2. Increase the timeout: `--config-timeout=10s`
3. Simplify complex computations

```python
# BAD - infinite loop
def configure():
    while True:
        pass  # Never returns

# GOOD - simple conditionals
def configure():
    ci = getenv("CI", "") != ""
    return {"test": {"timeout": "60s" if ci else "30s"}}
```

### Invalid Duration Format

**Error:** `invalid duration "60"` or `invalid duration "one minute"`

**Cause:** Duration strings must use Go's duration format.

**Solution:** Use proper duration format with unit suffixes.

```toml
# BAD
timeout = "60"        # Missing unit
timeout = "one minute" # Not a valid format

# GOOD
timeout = "60s"       # 60 seconds
timeout = "1m"        # 1 minute
timeout = "1m30s"     # 1 minute 30 seconds
```

### Config Not Being Found

**Symptom:** Default settings are used even though you have a config file.

**Diagnostic steps:**

1. Run with verbose mode to see which config is used:
   ```bash
   skytest -v tests/
   # Output: skytest: using config /path/to/config.sky
   ```

2. Check your current directory:
   ```bash
   pwd
   ls config.sky sky.star sky.toml
   ```

3. Verify the config file is in the directory tree:
   ```bash
   # Walk up looking for config
   while [ ! -f config.sky ] && [ ! -f sky.star ] && [ ! -f sky.toml ]; do
       cd ..
       [ "$PWD" = "/" ] && break
   done
   ls config.sky sky.star sky.toml 2>/dev/null
   ```

### Starlark Syntax Error

**Error:** `executing config /path/to/config.sky: ...`

**Cause:** Syntax error in your Starlark config file.

**Solution:** Check your Starlark syntax. Common issues:

```python
# BAD - Python-style True/False must be capitalized
"verbose": true   # Starlark uses True

# GOOD
"verbose": True

# BAD - missing comma
return {
    "test": {
        "timeout": "30s"   # Missing comma
        "parallel": "auto"
    }
}

# GOOD
return {
    "test": {
        "timeout": "30s",
        "parallel": "auto",
    },
}
```

### configure() Must Return a Dict

**Error:** `configure() must return a dict, got string`

**Cause:** Your `configure()` function returns something other than a dictionary.

**Solution:** Ensure you return a dictionary:

```python
# BAD
def configure():
    return "timeout=60s"  # Returns string

# GOOD
def configure():
    return {
        "test": {
            "timeout": "60s",
        },
    }
```

## Migration Guide

### From No Config to sky.toml

1. Create `sky.toml` in your project root:
   ```toml
   [test]
   timeout = "30s"
   ```

2. Remove flags from your test commands:
   ```bash
   # Before
   skytest --timeout=30s tests/

   # After
   skytest tests/
   ```

### From sky.toml to config.sky

If you need conditional logic:

1. Create `config.sky` with equivalent settings:
   ```python
   def configure():
       return {
           "test": {
               "timeout": "30s",
               # Add conditional logic as needed
           },
       }
   ```

2. Delete `sky.toml`:
   ```bash
   rm sky.toml
   ```

### From sky.star to config.sky

The `sky.star` filename is still supported but deprecated. To migrate:

1. Rename the file:
   ```bash
   mv sky.star config.sky
   ```

No changes to the file contents are needed - the format is identical.

## Best Practices

1. **Start simple:** Begin with `sky.toml` and migrate to `config.sky` only when you need conditional logic.

2. **Commit your config:** Config files should be version controlled so all team members use the same settings.

3. **Document overrides:** If team members need to override settings locally, document how:
   ```bash
   # Override for local development
   SKY_CONFIG=local.sky skytest tests/
   ```

4. **Use CI detection:** The `CI` environment variable is set by most CI systems (GitHub Actions, GitLab CI, CircleCI, etc.):
   ```python
   ci = getenv("CI", "") != ""
   ```

5. **Keep configs fast:** Avoid complex computations in `configure()`. The function should return quickly.

6. **Test your config:** Run with `-v` to verify your config is being loaded correctly:
   ```bash
   skytest -v tests/
   ```
