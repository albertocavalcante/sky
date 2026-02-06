package skyconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTOMLConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, cfg *Config)
		wantErr bool
	}{
		{
			name: "basic test config",
			content: `
[test]
timeout = "60s"
parallel = "auto"
prelude = ["test/helpers.star"]
prefix = "test_"
fail_fast = true
verbose = true
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Test.Timeout.Duration != 60*time.Second {
					t.Errorf("timeout = %v, want 60s", cfg.Test.Timeout.Duration)
				}
				if cfg.Test.Parallel != "auto" {
					t.Errorf("parallel = %q, want %q", cfg.Test.Parallel, "auto")
				}
				if len(cfg.Test.Prelude) != 1 || cfg.Test.Prelude[0] != "test/helpers.star" {
					t.Errorf("prelude = %v, want [test/helpers.star]", cfg.Test.Prelude)
				}
				if cfg.Test.Prefix != "test_" {
					t.Errorf("prefix = %q, want %q", cfg.Test.Prefix, "test_")
				}
				if !cfg.Test.FailFast {
					t.Error("fail_fast = false, want true")
				}
				if !cfg.Test.Verbose {
					t.Error("verbose = false, want true")
				}
			},
		},
		{
			name: "coverage config",
			content: `
[test.coverage]
enabled = true
fail_under = 80.5
output = "coverage.json"
`,
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Test.Coverage.Enabled {
					t.Error("coverage.enabled = false, want true")
				}
				if cfg.Test.Coverage.FailUnder != 80.5 {
					t.Errorf("coverage.fail_under = %v, want 80.5", cfg.Test.Coverage.FailUnder)
				}
				if cfg.Test.Coverage.Output != "coverage.json" {
					t.Errorf("coverage.output = %q, want %q", cfg.Test.Coverage.Output, "coverage.json")
				}
			},
		},
		{
			name: "lint config",
			content: `
[lint]
enable = ["all"]
disable = ["native-*"]
warnings_as_errors = true
`,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.Lint.Enable) != 1 || cfg.Lint.Enable[0] != "all" {
					t.Errorf("lint.enable = %v, want [all]", cfg.Lint.Enable)
				}
				if len(cfg.Lint.Disable) != 1 || cfg.Lint.Disable[0] != "native-*" {
					t.Errorf("lint.disable = %v, want [native-*]", cfg.Lint.Disable)
				}
				if !cfg.Lint.WarningsAsErrors {
					t.Error("lint.warnings_as_errors = false, want true")
				}
			},
		},
		{
			name:    "empty config",
			content: "",
			check: func(t *testing.T, cfg *Config) {
				// Should parse without error
			},
		},
		{
			name:    "invalid toml",
			content: "this is not valid toml [[[",
			wantErr: true,
		},
		{
			name: "invalid duration",
			content: `
[test]
timeout = "not-a-duration"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "sky.toml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := LoadTOMLConfig(configPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadTOMLConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoadStarlarkConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		ext     string // file extension (default: .star)
		env     map[string]string
		check   func(t *testing.T, cfg *Config)
		wantErr bool
	}{
		{
			name: "basic configure function",
			content: `
def configure():
    return {
        "test": {
            "timeout": "90s",
            "parallel": "4",
            "prelude": ["helpers.star"],
        },
    }
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Test.Timeout.Duration != 90*time.Second {
					t.Errorf("timeout = %v, want 90s", cfg.Test.Timeout.Duration)
				}
				if cfg.Test.Parallel != "4" {
					t.Errorf("parallel = %q, want %q", cfg.Test.Parallel, "4")
				}
				if len(cfg.Test.Prelude) != 1 || cfg.Test.Prelude[0] != "helpers.star" {
					t.Errorf("prelude = %v, want [helpers.star]", cfg.Test.Prelude)
				}
			},
		},
		{
			name: "config.sky extension",
			ext:  ".sky",
			content: `
def configure():
    return {
        "test": {
            "timeout": "60s",
        },
    }
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Test.Timeout.Duration != 60*time.Second {
					t.Errorf("timeout = %v, want 60s", cfg.Test.Timeout.Duration)
				}
			},
		},
		{
			name: "conditional with getenv",
			content: `
def configure():
    ci = getenv("CI", "") != ""
    return {
        "test": {
            "timeout": "120s" if ci else "30s",
            "parallel": "1" if ci else "auto",
        },
    }
`,
			env: map[string]string{"CI": "true"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Test.Timeout.Duration != 120*time.Second {
					t.Errorf("timeout = %v, want 120s (CI=true)", cfg.Test.Timeout.Duration)
				}
				if cfg.Test.Parallel != "1" {
					t.Errorf("parallel = %q, want %q (CI=true)", cfg.Test.Parallel, "1")
				}
			},
		},
		{
			name: "conditional without CI",
			content: `
def configure():
    ci = getenv("CI", "") != ""
    return {
        "test": {
            "timeout": "120s" if ci else "30s",
            "parallel": "1" if ci else "auto",
        },
    }
`,
			env: map[string]string{"CI": ""}, // Explicitly clear CI for this test
			check: func(t *testing.T, cfg *Config) {
				if cfg.Test.Timeout.Duration != 30*time.Second {
					t.Errorf("timeout = %v, want 30s (CI not set)", cfg.Test.Timeout.Duration)
				}
				if cfg.Test.Parallel != "auto" {
					t.Errorf("parallel = %q, want %q (CI not set)", cfg.Test.Parallel, "auto")
				}
			},
		},
		{
			name: "host_os and host_arch",
			content: `
def configure():
    return {
        "test": {
            "parallel": "auto" if host_os == "darwin" or host_os == "linux" else "1",
        },
    }
`,
			check: func(t *testing.T, cfg *Config) {
				// Should succeed without error; actual value depends on OS
				if cfg.Test.Parallel == "" {
					t.Error("parallel should be set")
				}
			},
		},
		{
			name: "duration builtin",
			content: `
def configure():
    timeout = duration("45s")
    return {
        "test": {
            "timeout": timeout,
        },
    }
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Test.Timeout.Duration != 45*time.Second {
					t.Errorf("timeout = %v, want 45s", cfg.Test.Timeout.Duration)
				}
			},
		},
		{
			name: "invalid duration",
			content: `
def configure():
    return {
        "test": {
            "timeout": duration("invalid"),
        },
    }
`,
			wantErr: true,
		},
		{
			name: "coverage config",
			content: `
def configure():
    return {
        "test": {
            "coverage": {
                "enabled": True,
                "fail_under": 80,
            },
        },
    }
`,
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Test.Coverage.Enabled {
					t.Error("coverage.enabled = false, want true")
				}
				if cfg.Test.Coverage.FailUnder != 80 {
					t.Errorf("coverage.fail_under = %v, want 80", cfg.Test.Coverage.FailUnder)
				}
			},
		},
		{
			name: "lint config",
			content: `
def configure():
    return {
        "lint": {
            "enable": ["all"],
            "disable": ["native-*"],
            "warnings_as_errors": True,
        },
    }
`,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.Lint.Enable) != 1 || cfg.Lint.Enable[0] != "all" {
					t.Errorf("lint.enable = %v, want [all]", cfg.Lint.Enable)
				}
				if len(cfg.Lint.Disable) != 1 || cfg.Lint.Disable[0] != "native-*" {
					t.Errorf("lint.disable = %v, want [native-*]", cfg.Lint.Disable)
				}
				if !cfg.Lint.WarningsAsErrors {
					t.Error("lint.warnings_as_errors = false, want true")
				}
			},
		},
		{
			name:    "missing configure function",
			content: `x = 1`,
			wantErr: true,
		},
		{
			name: "configure returns non-dict",
			content: `
def configure():
    return "not a dict"
`,
			wantErr: true,
		},
		{
			name:    "syntax error",
			content: `def configure( = {}`,
			wantErr: true,
		},
		{
			name: "parallel as int",
			content: `
def configure():
    return {
        "test": {
            "parallel": 4,
        },
    }
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Test.Parallel != "4" {
					t.Errorf("parallel = %q, want %q", cfg.Test.Parallel, "4")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			// Create temp file
			tmpDir := t.TempDir()
			ext := tt.ext
			if ext == "" {
				ext = ".star"
			}
			configPath := filepath.Join(tmpDir, "config"+ext)
			if err := os.WriteFile(configPath, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := LoadStarlarkConfig(configPath, DefaultStarlarkTimeout)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadStarlarkConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestStarlarkTimeout(t *testing.T) {
	content := `
def configure():
    # Infinite loop
    while True:
        pass
    return {}
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.sky")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	start := time.Now()
	_, err := LoadStarlarkConfig(configPath, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}

	// Should complete within reasonable time of the timeout
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestDiscoverConfig(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		wantFile string
		wantErr  bool
	}{
		{
			name: "finds config.sky (canonical)",
			setup: func(t *testing.T, dir string) {
				content := `def configure():
    return {"test": {"timeout": "60s"}}
`
				if err := os.WriteFile(filepath.Join(dir, "config.sky"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFile: "config.sky",
		},
		{
			name: "finds sky.star (legacy)",
			setup: func(t *testing.T, dir string) {
				content := `def configure():
    return {"test": {"timeout": "60s"}}
`
				if err := os.WriteFile(filepath.Join(dir, "sky.star"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFile: "sky.star",
		},
		{
			name: "finds sky.toml",
			setup: func(t *testing.T, dir string) {
				content := `[test]
timeout = "60s"
`
				if err := os.WriteFile(filepath.Join(dir, "sky.toml"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantFile: "sky.toml",
		},
		{
			name: "prefers config.sky over sky.star",
			setup: func(t *testing.T, dir string) {
				// Create both files - should error (conflict)
				content := `def configure():
    return {}
`
				if err := os.WriteFile(filepath.Join(dir, "config.sky"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "sky.star"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true, // Conflict
		},
		{
			name: "conflict between sky.star and sky.toml",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "sky.toml"), []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
				content := `def configure():
    return {}
`
				if err := os.WriteFile(filepath.Join(dir, "sky.star"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true, // Conflict
		},
		{
			name: "conflict between all three",
			setup: func(t *testing.T, dir string) {
				content := `def configure():
    return {}
`
				if err := os.WriteFile(filepath.Join(dir, "config.sky"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "sky.star"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "sky.toml"), []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true, // Conflict
		},
		{
			name: "finds config in parent",
			setup: func(t *testing.T, dir string) {
				// Create subdir and config in parent
				subdir := filepath.Join(dir, "subdir")
				if err := os.MkdirAll(subdir, 0o755); err != nil {
					t.Fatal(err)
				}
				content := `[test]
timeout = "60s"
`
				if err := os.WriteFile(filepath.Join(dir, "sky.toml"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}

				// Change to subdir
				if err := os.Chdir(subdir); err != nil {
					t.Fatal(err)
				}
			},
			wantFile: "sky.toml",
		},
		{
			name:     "no config returns defaults",
			setup:    func(t *testing.T, dir string) {},
			wantFile: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Save current working directory
			origDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Chdir(origDir) }()

			// Clear any existing SKY_CONFIG env var
			t.Setenv(EnvConfig, "")

			// Create a .git directory to act as the root
			if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}

			tt.setup(t, tmpDir)

			// Discover from tmpDir
			cfg, configPath, err := DiscoverConfig(tmpDir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DiscoverConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if tt.wantFile == "" {
				if configPath != "" {
					t.Errorf("expected no config file, got %q", configPath)
				}
			} else {
				if filepath.Base(configPath) != tt.wantFile {
					t.Errorf("configPath = %q, want %q", filepath.Base(configPath), tt.wantFile)
				}
			}

			if cfg == nil {
				t.Error("cfg should not be nil")
			}
		})
	}
}

func TestDiscoverConfigEnvVar(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config file
	configPath := filepath.Join(tmpDir, "custom-config.sky")
	content := `def configure():
    return {"test": {"timeout": "99s"}}
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set SKY_CONFIG env var
	t.Setenv(EnvConfig, configPath)

	// Should use env var path even when there's another config
	anotherDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(anotherDir, "sky.toml"), []byte("[test]\ntimeout = \"1s\""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, foundPath, err := DiscoverConfig(anotherDir)
	if err != nil {
		t.Fatalf("DiscoverConfig() error = %v", err)
	}

	if foundPath != configPath {
		t.Errorf("foundPath = %q, want %q", foundPath, configPath)
	}

	if cfg.Test.Timeout.Duration != 99*time.Second {
		t.Errorf("timeout = %v, want 99s", cfg.Test.Timeout.Duration)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test TOML
	tomlPath := filepath.Join(tmpDir, "test.toml")
	if err := os.WriteFile(tomlPath, []byte(`[test]
timeout = "30s"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tomlPath)
	if err != nil {
		t.Fatalf("LoadConfig(toml) error = %v", err)
	}
	if cfg.Test.Timeout.Duration != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", cfg.Test.Timeout.Duration)
	}

	// Test .star extension
	starPath := filepath.Join(tmpDir, "test.star")
	if err := os.WriteFile(starPath, []byte(`def configure():
    return {"test": {"timeout": "45s"}}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err = LoadConfig(starPath)
	if err != nil {
		t.Fatalf("LoadConfig(star) error = %v", err)
	}
	if cfg.Test.Timeout.Duration != 45*time.Second {
		t.Errorf("timeout = %v, want 45s", cfg.Test.Timeout.Duration)
	}

	// Test .sky extension (canonical)
	skyPath := filepath.Join(tmpDir, "config.sky")
	if err := os.WriteFile(skyPath, []byte(`def configure():
    return {"test": {"timeout": "55s"}}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err = LoadConfig(skyPath)
	if err != nil {
		t.Fatalf("LoadConfig(sky) error = %v", err)
	}
	if cfg.Test.Timeout.Duration != 55*time.Second {
		t.Errorf("timeout = %v, want 55s", cfg.Test.Timeout.Duration)
	}

	// Test unsupported extension
	jsonPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(jsonPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadConfig(jsonPath)
	if err == nil {
		t.Error("expected error for unsupported extension")
	}
}

func TestConfigMerge(t *testing.T) {
	base := DefaultConfig()
	base.Test.Timeout = Duration{30 * time.Second}
	base.Test.Parallel = "auto"

	other := &Config{
		Test: TestConfig{
			Timeout:  Duration{60 * time.Second},
			Prelude:  []string{"helpers.star"},
			FailFast: true,
		},
	}

	base.Merge(other)

	if base.Test.Timeout.Duration != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", base.Test.Timeout.Duration)
	}
	if base.Test.Parallel != "auto" {
		t.Errorf("parallel = %q, want %q (should keep original)", base.Test.Parallel, "auto")
	}
	if len(base.Test.Prelude) != 1 || base.Test.Prelude[0] != "helpers.star" {
		t.Errorf("prelude = %v, want [helpers.star]", base.Test.Prelude)
	}
	if !base.Test.FailFast {
		t.Error("fail_fast = false, want true")
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"30s", 30 * time.Second, false},
		{"1m", 1 * time.Minute, false},
		{"1h30m", 90 * time.Minute, false},
		{"", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalText(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && d.Duration != tt.want {
				t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, d.Duration, tt.want)
			}
		})
	}
}
