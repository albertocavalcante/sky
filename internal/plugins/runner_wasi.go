package plugins

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

func runWasm(ctx context.Context, plugin Plugin, mode string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	wasmBytes, err := os.ReadFile(plugin.Path)
	if err != nil {
		return 1, err
	}

	runtime := wazero.NewRuntime(ctx)
	defer func() { _ = runtime.Close(ctx) }()

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return 1, err
	}

	argv := append([]string{plugin.Name}, args...)
	config := wazero.NewModuleConfig().
		WithArgs(argv...).
		WithEnv(EnvPlugin, "1").
		WithEnv(EnvPluginMode, mode).
		WithEnv(EnvPluginName, plugin.Name).
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr)

	_, err = runtime.InstantiateWithConfig(ctx, wasmBytes, config)
	if err == nil {
		return 0, nil
	}

	var exitErr *sys.ExitError
	if errors.As(err, &exitErr) {
		return int(exitErr.ExitCode()), nil
	}
	return 1, err
}
