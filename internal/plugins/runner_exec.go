package plugins

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

func runExec(ctx context.Context, plugin Plugin, mode string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, plugin.Path, args...)
	cmd.Env = append(os.Environ(),
		EnvPlugin+"=1",
		EnvPluginMode+"="+mode,
		EnvPluginName+"="+plugin.Name,
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
