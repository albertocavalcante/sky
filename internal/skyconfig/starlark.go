package skyconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"go.starlark.net/starlark"
)

// DefaultStarlarkTimeout is the default execution timeout for Starlark config files.
const DefaultStarlarkTimeout = 5 * time.Second

// ErrConfigureNotFound is returned when sky.star doesn't define a configure() function.
var ErrConfigureNotFound = errors.New("sky.star must define a configure() function")

// ErrConfigureReturnType is returned when configure() doesn't return a dict.
var ErrConfigureReturnType = errors.New("configure() must return a dict")

// LoadStarlarkConfig loads a configuration from a Starlark file.
// The file must define a configure() function that returns a dict.
// The execution is sandboxed: no filesystem or network access, with a timeout.
func LoadStarlarkConfig(path string, timeout time.Duration) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	// Create a cancellable context for timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create thread with timeout
	thread := &starlark.Thread{
		Name: path,
	}

	// Set up cancellation
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			thread.Cancel("execution timeout")
		case <-done:
		}
	}()
	defer close(done)

	// Execute the file with sandboxed predeclared
	globals, err := starlark.ExecFile(thread, path, data, configPredeclared())
	if err != nil {
		return nil, fmt.Errorf("executing config %s: %w", path, err)
	}

	// Look for configure function
	configureFn, ok := globals["configure"]
	if !ok {
		return nil, fmt.Errorf("%s: %w", path, ErrConfigureNotFound)
	}

	fn, ok := configureFn.(*starlark.Function)
	if !ok {
		return nil, fmt.Errorf("%s: configure must be a function, got %s", path, configureFn.Type())
	}

	// Call configure()
	result, err := starlark.Call(thread, fn, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: calling configure(): %w", path, err)
	}

	// Convert result to Config
	dict, ok := result.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("%s: %w, got %s", path, ErrConfigureReturnType, result.Type())
	}

	return dictToConfig(dict)
}

// configPredeclared returns the predeclared values for config Starlark files.
// This is a sandboxed environment with no filesystem or network access.
func configPredeclared() starlark.StringDict {
	return starlark.StringDict{
		"getenv":    starlark.NewBuiltin("getenv", builtinGetenv),
		"host_os":   starlark.String(runtime.GOOS),
		"host_arch": starlark.String(runtime.GOARCH),
		"duration":  starlark.NewBuiltin("duration", builtinDuration),
		"struct":    starlark.NewBuiltin("struct", builtinStruct),
	}
}

// builtinGetenv implements getenv(name, default="") -> string.
func builtinGetenv(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var defaultVal starlark.String
	if err := starlark.UnpackArgs("getenv", args, kwargs, "name", &name, "default?", &defaultVal); err != nil {
		return nil, err
	}

	val := os.Getenv(name)
	if val == "" {
		return defaultVal, nil
	}
	return starlark.String(val), nil
}

// builtinDuration implements duration(s) -> string.
// Validates that the string is a valid Go duration.
func builtinDuration(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s string
	if err := starlark.UnpackArgs("duration", args, kwargs, "s", &s); err != nil {
		return nil, err
	}

	// Validate the duration format
	if _, err := time.ParseDuration(s); err != nil {
		return nil, fmt.Errorf("invalid duration %q: %w", s, err)
	}

	return starlark.String(s), nil
}

// builtinStruct implements a simple struct constructor.
func builtinStruct(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, errors.New("struct: positional arguments not allowed")
	}

	// Create a dict from kwargs
	d := starlark.NewDict(len(kwargs))
	for _, kv := range kwargs {
		if err := d.SetKey(starlark.String(string(kv[0].(starlark.String))), kv[1]); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// dictToConfig converts a Starlark dict to a Config struct.
func dictToConfig(d *starlark.Dict) (*Config, error) {
	cfg := DefaultConfig()

	// Extract "test" section
	if testVal, found, _ := d.Get(starlark.String("test")); found {
		testDict, ok := testVal.(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("test must be a dict, got %s", testVal.Type())
		}
		if err := parseTestConfig(testDict, &cfg.Test); err != nil {
			return nil, fmt.Errorf("parsing test config: %w", err)
		}
	}

	// Extract "lint" section
	if lintVal, found, _ := d.Get(starlark.String("lint")); found {
		lintDict, ok := lintVal.(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("lint must be a dict, got %s", lintVal.Type())
		}
		if err := parseLintConfig(lintDict, &cfg.Lint); err != nil {
			return nil, fmt.Errorf("parsing lint config: %w", err)
		}
	}

	return cfg, nil
}

// parseTestConfig parses the test section from a Starlark dict.
func parseTestConfig(d *starlark.Dict, cfg *TestConfig) error {
	// timeout
	if v, found, _ := d.Get(starlark.String("timeout")); found {
		s, ok := starlark.AsString(v)
		if !ok {
			return fmt.Errorf("timeout must be a string, got %s", v.Type())
		}
		dur, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", s, err)
		}
		cfg.Timeout = Duration{dur}
	}

	// parallel
	if v, found, _ := d.Get(starlark.String("parallel")); found {
		switch val := v.(type) {
		case starlark.String:
			cfg.Parallel = string(val)
		case starlark.Int:
			i, _ := val.Int64()
			cfg.Parallel = fmt.Sprintf("%d", i)
		default:
			return fmt.Errorf("parallel must be a string or int, got %s", v.Type())
		}
	}

	// prelude
	if v, found, _ := d.Get(starlark.String("prelude")); found {
		list, ok := v.(*starlark.List)
		if !ok {
			return fmt.Errorf("prelude must be a list, got %s", v.Type())
		}
		cfg.Prelude = nil
		for i := 0; i < list.Len(); i++ {
			s, ok := starlark.AsString(list.Index(i))
			if !ok {
				return fmt.Errorf("prelude[%d] must be a string", i)
			}
			cfg.Prelude = append(cfg.Prelude, s)
		}
	}

	// prefix
	if v, found, _ := d.Get(starlark.String("prefix")); found {
		s, ok := starlark.AsString(v)
		if !ok {
			return fmt.Errorf("prefix must be a string, got %s", v.Type())
		}
		cfg.Prefix = s
	}

	// fail_fast
	if v, found, _ := d.Get(starlark.String("fail_fast")); found {
		b, ok := v.(starlark.Bool)
		if !ok {
			return fmt.Errorf("fail_fast must be a bool, got %s", v.Type())
		}
		cfg.FailFast = bool(b)
	}

	// verbose
	if v, found, _ := d.Get(starlark.String("verbose")); found {
		b, ok := v.(starlark.Bool)
		if !ok {
			return fmt.Errorf("verbose must be a bool, got %s", v.Type())
		}
		cfg.Verbose = bool(b)
	}

	// coverage
	if v, found, _ := d.Get(starlark.String("coverage")); found {
		coverageDict, ok := v.(*starlark.Dict)
		if !ok {
			return fmt.Errorf("coverage must be a dict, got %s", v.Type())
		}
		if err := parseCoverageConfig(coverageDict, &cfg.Coverage); err != nil {
			return fmt.Errorf("parsing coverage config: %w", err)
		}
	}

	return nil
}

// parseCoverageConfig parses the coverage section from a Starlark dict.
func parseCoverageConfig(d *starlark.Dict, cfg *CoverageConfig) error {
	// enabled
	if v, found, _ := d.Get(starlark.String("enabled")); found {
		b, ok := v.(starlark.Bool)
		if !ok {
			return fmt.Errorf("enabled must be a bool, got %s", v.Type())
		}
		cfg.Enabled = bool(b)
	}

	// fail_under
	if v, found, _ := d.Get(starlark.String("fail_under")); found {
		switch val := v.(type) {
		case starlark.Int:
			i, _ := val.Int64()
			cfg.FailUnder = float64(i)
		case starlark.Float:
			cfg.FailUnder = float64(val)
		default:
			return fmt.Errorf("fail_under must be a number, got %s", v.Type())
		}
	}

	// output
	if v, found, _ := d.Get(starlark.String("output")); found {
		s, ok := starlark.AsString(v)
		if !ok {
			return fmt.Errorf("output must be a string, got %s", v.Type())
		}
		cfg.Output = s
	}

	return nil
}

// parseLintConfig parses the lint section from a Starlark dict.
func parseLintConfig(d *starlark.Dict, cfg *LintConfig) error {
	// enable
	if v, found, _ := d.Get(starlark.String("enable")); found {
		list, ok := v.(*starlark.List)
		if !ok {
			return fmt.Errorf("enable must be a list, got %s", v.Type())
		}
		cfg.Enable = nil
		for i := 0; i < list.Len(); i++ {
			s, ok := starlark.AsString(list.Index(i))
			if !ok {
				return fmt.Errorf("enable[%d] must be a string", i)
			}
			cfg.Enable = append(cfg.Enable, s)
		}
	}

	// disable
	if v, found, _ := d.Get(starlark.String("disable")); found {
		list, ok := v.(*starlark.List)
		if !ok {
			return fmt.Errorf("disable must be a list, got %s", v.Type())
		}
		cfg.Disable = nil
		for i := 0; i < list.Len(); i++ {
			s, ok := starlark.AsString(list.Index(i))
			if !ok {
				return fmt.Errorf("disable[%d] must be a string", i)
			}
			cfg.Disable = append(cfg.Disable, s)
		}
	}

	// warnings_as_errors
	if v, found, _ := d.Get(starlark.String("warnings_as_errors")); found {
		b, ok := v.(starlark.Bool)
		if !ok {
			return fmt.Errorf("warnings_as_errors must be a bool, got %s", v.Type())
		}
		cfg.WarningsAsErrors = bool(b)
	}

	return nil
}
