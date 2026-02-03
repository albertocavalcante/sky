package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/albertocavalcante/sky/internal/plugins"
	"github.com/albertocavalcante/sky/internal/version"
)

// coreCommands maps short aliases to standalone binary names.
// These commands are dispatched to co-located binaries before falling back to plugins.
var coreCommands = map[string]string{
	"fmt":   "skyfmt",
	"lint":  "skylint",
	"check": "skycheck",
	"query": "skyquery",
	"repl":  "skyrepl",
	"test":  "skytest",
	"doc":   "skydoc",
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || isHelp(args[0]) {
		printUsage(stderr)
		return 0
	}

	switch args[0] {
	case "version":
		writef(stdout, "sky %s\n", version.String())
		return 0
	case "plugin":
		return runPlugin(args[1:], stdout, stderr)
	case "help":
		printUsage(stderr)
		return 0
	default:
		// Check for core command aliases (fmt, lint, check, etc.)
		if _, ok := coreCommands[args[0]]; ok {
			return runCoreCommand(args[0], args[1:], stdout, stderr)
		}
		// Check if it's an embedded tool by full name (skylint, skyfmt, etc.)
		if tool := getEmbeddedTool(args[0]); tool != nil {
			return tool(context.Background(), args[1:], os.Stdin, stdout, stderr)
		}
		return runInstalledPlugin(args, stdout, stderr)
	}
}

// runCoreCommand runs a core command.
// Resolution order:
// 1. Embedded tools (if built with -tags=sky_full)
// 2. External binary in same directory as sky executable
// 3. External binary in PATH
// 4. Plugin system
func runCoreCommand(name string, args []string, stdout, stderr io.Writer) int {
	// First, check for embedded tool
	if tool := getEmbeddedTool(name); tool != nil {
		return tool(context.Background(), args, os.Stdin, stdout, stderr)
	}

	// Try to find the external binary
	binary := coreCommands[name]
	if binary == "" {
		binary = name // Use name directly if not in alias map
	}

	path, err := findCoreBinary(binary)
	if err != nil {
		// Fall back to plugin system
		return runInstalledPlugin(append([]string{name}, args...), stdout, stderr)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	return 0
}

// findCoreBinary looks for a core command binary.
// It first checks the same directory as the sky binary, then falls back to PATH.
func findCoreBinary(name string) (string, error) {
	// Try to find the binary alongside the sky executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Fall back to PATH lookup
	return exec.LookPath(name)
}

func runPlugin(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || isHelp(args[0]) {
		printPluginUsage(stderr)
		return 0
	}

	switch args[0] {
	case "list":
		return runPluginList(stdout, stderr)
	case "install":
		return runPluginInstall(args[1:], stdout, stderr)
	case "remove":
		return runPluginRemove(args[1:], stdout, stderr)
	case "inspect":
		return runPluginInspect(args[1:], stdout, stderr)
	case "search":
		return runPluginSearch(args[1:], stdout, stderr)
	case "marketplace":
		return runMarketplace(args[1:], stdout, stderr)
	default:
		writef(stderr, "unknown plugin command %q\n", args[0])
		printPluginUsage(stderr)
		return 2
	}
}

func runPluginList(stdout, stderr io.Writer) int {
	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	list, err := store.LoadPlugins()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	if len(list) == 0 {
		writeln(stdout, "no plugins installed")
		return 0
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	writeln(writer, "NAME\tTYPE\tVERSION\tSOURCE\tDESCRIPTION")
	for _, plugin := range list {
		writef(writer, "%s\t%s\t%s\t%s\t%s\n", plugin.Name, plugin.EffectiveType(), plugin.Version, plugin.Source, plugin.Description)
	}
	_ = writer.Flush()
	return 0
}

func runPluginInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("path", "", "path to local plugin binary")
	url := fs.String("url", "", "URL to download plugin binary")
	marketplace := fs.String("marketplace", "", "marketplace name (optional)")
	versionFlag := fs.String("version", "", "plugin version metadata")
	sha := fs.String("sha256", "", "expected sha256 for --url downloads")
	typeFlag := fs.String("type", "", "plugin type (exe|wasm)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		writeln(stderr, "usage: sky plugin install <name> [--path PATH | --url URL] [--marketplace NAME] [--type exe|wasm]")
		return 2
	}
	name := fs.Arg(0)

	if *path != "" && *url != "" {
		writeln(stderr, "sky: only one of --path or --url is allowed")
		return 2
	}
	if *typeFlag != "" && *path == "" && *url == "" {
		writeln(stderr, "sky: --type requires --path or --url")
		return 2
	}

	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	pluginType, err := plugins.ParsePluginType(*typeFlag)
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	if *typeFlag == "" {
		if *path != "" {
			pluginType = plugins.DetectPluginType(*path)
		}
		if *url != "" {
			pluginType = plugins.DetectPluginType(*url)
		}
	}

	ctx := context.Background()
	var plugin plugins.Plugin
	if *path != "" {
		plugin, err = store.InstallFromPath(name, *path, *versionFlag, pluginType)
	} else if *url != "" {
		plugin, err = store.InstallFromURL(ctx, name, *url, *sha, *versionFlag, "", pluginType)
	} else {
		plugin, err = store.InstallFromMarketplace(ctx, name, *marketplace)
	}
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	writef(stdout, "installed %s (%s)\n", plugin.Name, plugin.Version)
	return 0
}

func runPluginRemove(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		writeln(stderr, "usage: sky plugin remove <name>")
		return 2
	}

	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	removed, err := store.RemovePlugin(fs.Arg(0))
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	writef(stdout, "removed %s\n", removed.Name)
	return 0
}

func runPluginInspect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		writeln(stderr, "usage: sky plugin inspect <name>")
		return 2
	}

	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	plugin, err := store.FindPlugin(fs.Arg(0))
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	if plugin == nil {
		writef(stderr, "sky: plugin %q not installed\n", fs.Arg(0))
		return 1
	}

	runner := plugins.Runner{}
	metadata, err := runner.Metadata(context.Background(), *plugin)
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	if metadata.Version != "" {
		plugin.Version = metadata.Version
	}
	if metadata.Summary != "" {
		plugin.Description = metadata.Summary
	}
	plugin.Type = plugin.EffectiveType()
	if err := store.UpsertPlugin(*plugin); err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	payload, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	writeln(stdout, string(payload))
	return 0
}

func runPluginSearch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	marketplace := fs.String("marketplace", "", "marketplace name (optional)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		writeln(stderr, "usage: sky plugin search <query> [--marketplace NAME]")
		return 2
	}

	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	results, err := store.SearchMarketplaces(context.Background(), fs.Arg(0), *marketplace)
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	writeln(writer, "NAME\tVERSION\tMARKETPLACE\tDESCRIPTION\tURL")
	for _, result := range results {
		writef(writer, "%s\t%s\t%s\t%s\t%s\n", result.Plugin.Name, result.Plugin.Version, result.Marketplace.Name, result.Plugin.Description, result.Plugin.URL)
	}
	_ = writer.Flush()
	return 0
}

func runMarketplace(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || isHelp(args[0]) {
		printMarketplaceUsage(stderr)
		return 0
	}

	switch args[0] {
	case "list":
		return runMarketplaceList(stdout, stderr)
	case "add":
		return runMarketplaceAdd(args[1:], stdout, stderr)
	case "remove":
		return runMarketplaceRemove(args[1:], stdout, stderr)
	default:
		writef(stderr, "unknown marketplace command %q\n", args[0])
		printMarketplaceUsage(stderr)
		return 2
	}
}

func runMarketplaceList(stdout, stderr io.Writer) int {
	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	list, err := store.LoadMarketplaces()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	if len(list) == 0 {
		writeln(stdout, "no marketplaces configured")
		return 0
	}

	writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	writeln(writer, "NAME\tURL\tADDED")
	for _, marketplace := range list {
		added := ""
		if !marketplace.AddedAt.IsZero() {
			added = marketplace.AddedAt.Format(time.RFC3339)
		}
		writef(writer, "%s\t%s\t%s\n", marketplace.Name, marketplace.URL, added)
	}
	_ = writer.Flush()
	return 0
}

func runMarketplaceAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("marketplace add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		writeln(stderr, "usage: sky plugin marketplace add <name> <url>")
		return 2
	}

	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	marketplace := plugins.Marketplace{
		Name:    fs.Arg(0),
		URL:     fs.Arg(1),
		AddedAt: time.Now().UTC(),
	}
	if err := store.UpsertMarketplace(marketplace); err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	writef(stdout, "marketplace %s added\n", marketplace.Name)
	return 0
}

func runMarketplaceRemove(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("marketplace remove", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		writeln(stderr, "usage: sky plugin marketplace remove <name>")
		return 2
	}

	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	removed, err := store.RemoveMarketplace(fs.Arg(0))
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	writef(stdout, "marketplace %s removed\n", removed.Name)
	return 0
}

func runInstalledPlugin(args []string, stdout, stderr io.Writer) int {
	store, err := plugins.DefaultStore()
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}

	plugin, err := store.FindPlugin(args[0])
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	if plugin == nil {
		writef(stderr, "unknown command %q\n", args[0])
		writeln(stderr, "install plugins with: sky plugin search <query>")
		return 2
	}
	runner := plugins.Runner{}
	exitCode, err := runner.Run(context.Background(), *plugin, args[1:], os.Stdin, stdout, stderr)
	if err != nil {
		writef(stderr, "sky: %v\n", err)
		return 1
	}
	return exitCode
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help"
}

// Helper functions for writing output.
// Write errors are intentionally ignored because:
//  1. These functions write to stdout/stderr where there's no reasonable recovery
//     if the terminal/pipe is broken (EPIPE, etc.)
//  2. If we can't write error messages, we can't report the write failure either
//  3. The exit code still reflects the actual operation status
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

func printUsage(w io.Writer) {
	writeln(w, "usage: sky <command> [args]")
	writeln(w)
	writeln(w, "starlark tools:")
	writeln(w, "  fmt          format Starlark files")
	writeln(w, "  lint         lint Starlark files")
	writeln(w, "  check        static analysis for Starlark files")
	writeln(w, "  query        query Starlark sources")
	writeln(w, "  test         run Starlark tests")
	writeln(w, "  doc          generate documentation")
	writeln(w, "  repl         interactive Starlark REPL")
	writeln(w)
	writeln(w, "management:")
	writeln(w, "  plugin       manage plugins")
	writeln(w, "  version      show version")
	writeln(w)
	writeln(w, "plugin-first:")
	writeln(w, "  unknown commands are resolved to installed plugins")
	writeln(w)
	writeln(w, "run \"sky plugin --help\" for plugin commands")
}

func printPluginUsage(w io.Writer) {
	writeln(w, "usage: sky plugin <command> [args]")
	writeln(w)
	writeln(w, "commands:")
	writeln(w, "  list                     list installed plugins")
	writeln(w, "  install <name>           install a plugin")
	writeln(w, "  inspect <name>           inspect plugin metadata")
	writeln(w, "  remove <name>            remove a plugin")
	writeln(w, "  search <query>           search marketplaces")
	writeln(w, "  marketplace <command>    manage marketplaces")
}

func printMarketplaceUsage(w io.Writer) {
	writeln(w, "usage: sky plugin marketplace <command> [args]")
	writeln(w)
	writeln(w, "commands:")
	writeln(w, "  list                     list marketplaces")
	writeln(w, "  add <name> <url>          add or update a marketplace")
	writeln(w, "  remove <name>             remove a marketplace")
}
