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
	cmd.Env = append(os.Environ(), pluginEnv(plugin.Name, mode)...)
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

// pluginEnv returns the environment variables for plugin execution.
func pluginEnv(name, mode string) []string {
	env := []string{
		EnvPlugin + "=1",
		EnvPluginMode + "=" + mode,
		EnvPluginName + "=" + name,
	}

	// Add workspace root (v1.1)
	if root := FindWorkspaceRoot(); root != "" {
		env = append(env, EnvWorkspaceRoot+"="+root)
	}

	// Add config dir (v1.1)
	if configDir := configDirPath(); configDir != "" {
		env = append(env, EnvConfigDir+"="+configDir)
	}

	// Propagate output format if set
	if format := os.Getenv(EnvOutputFormat); format != "" {
		env = append(env, EnvOutputFormat+"="+format)
	}

	// Propagate no color if set
	if os.Getenv(EnvNoColor) != "" || os.Getenv("NO_COLOR") != "" {
		env = append(env, EnvNoColor+"=1")
	}

	// Propagate verbose if set
	if verbose := os.Getenv(EnvVerbose); verbose != "" {
		env = append(env, EnvVerbose+"="+verbose)
	}

	return env
}

// configDirPath returns the Sky config directory path.
func configDirPath() string {
	store, err := DefaultStore()
	if err != nil {
		return ""
	}
	return store.Root
}

// splitEnvVar splits an environment variable string "KEY=value" into parts.
func splitEnvVar(kv string) []string {
	idx := 0
	for i := 0; i < len(kv); i++ {
		if kv[i] == '=' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return nil
	}
	return []string{kv[:idx], kv[idx+1:]}
}
