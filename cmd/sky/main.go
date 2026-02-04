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
	"strings"
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
	"ls":    "skyls",
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
	case "init":
		return runPluginInit(args[1:], stdout, stderr)
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
		printUnknownCommandHelp(stderr, args[0])
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

// printUnknownCommandHelp prints a helpful error message for unknown commands.
func printUnknownCommandHelp(w io.Writer, cmdName string) {
	writef(w, "sky: unknown command %q\n\n", cmdName)

	// Find similar core commands
	suggestions := findSimilarCommands(cmdName)
	if len(suggestions) > 0 {
		writeln(w, "Did you mean one of these?")
		for _, s := range suggestions {
			writef(w, "  sky %-8s %s\n", s.name, s.desc)
		}
		writeln(w)
	}

	writeln(w, "To install a plugin:")
	writef(w, "  sky plugin install %s\n", cmdName)
	writef(w, "  sky plugin search %s\n", cmdName)
}

type commandSuggestion struct {
	name string
	desc string
}

// coreCommandDescriptions provides descriptions for suggestions.
var coreCommandDescriptions = map[string]string{
	"fmt":   "format Starlark files",
	"lint":  "lint Starlark files",
	"check": "static analysis",
	"query": "query Starlark sources",
	"test":  "run Starlark tests",
	"doc":   "generate documentation",
	"repl":  "interactive REPL",
	"ls":    "language server (LSP)",
}

// findSimilarCommands finds core commands similar to the input.
func findSimilarCommands(input string) []commandSuggestion {
	var suggestions []commandSuggestion
	input = strings.ToLower(input)

	for cmd := range coreCommands {
		// Check for prefix match
		if strings.HasPrefix(cmd, input) || strings.HasPrefix(input, cmd) {
			suggestions = append(suggestions, commandSuggestion{
				name: cmd,
				desc: coreCommandDescriptions[cmd],
			})
			continue
		}
		// Check for Levenshtein distance <= 2
		if levenshtein(input, cmd) <= 2 {
			suggestions = append(suggestions, commandSuggestion{
				name: cmd,
				desc: coreCommandDescriptions[cmd],
			})
		}
	}

	// Sort by name
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].name < suggestions[j].name
	})

	return suggestions
}

// levenshtein computes the Levenshtein distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	if len(a) > len(b) {
		a, b = b, a
	}

	prev := make([]int, len(a)+1)
	curr := make([]int, len(a)+1)

	for i := range prev {
		prev[i] = i
	}

	for j := 1; j <= len(b); j++ {
		curr[0] = j
		for i := 1; i <= len(a); i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min(
				prev[i]+1,      // deletion
				curr[i-1]+1,    // insertion
				prev[i-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(a)]
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
	writeln(w, "  ls           language server (LSP)")
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

func runPluginInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	wasm := fs.Bool("wasm", false, "create a WASM plugin template")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		writeln(stderr, "usage: sky plugin init <name> [--wasm]")
		writeln(stderr)
		writeln(stderr, "Creates a new plugin project with boilerplate code.")
		return 2
	}

	name := fs.Arg(0)

	// Validate plugin name
	if err := plugins.ValidateName(name); err != nil {
		writef(stderr, "sky: %v\n", err)
		writeln(stderr, "Plugin names must start with a letter and contain only lowercase letters, digits, and hyphens.")
		return 1
	}

	// Check if directory already exists
	if _, err := os.Stat(name); err == nil {
		writef(stderr, "sky: directory %q already exists\n", name)
		return 1
	}

	// Create directory
	if err := os.MkdirAll(name, 0755); err != nil {
		writef(stderr, "sky: failed to create directory: %v\n", err)
		return 1
	}

	// Write files
	if err := writePluginTemplate(name, *wasm); err != nil {
		writef(stderr, "sky: failed to create plugin files: %v\n", err)
		// Clean up on failure
		_ = os.RemoveAll(name)
		return 1
	}

	writef(stdout, "Created plugin %q\n\n", name)
	writeln(stdout, "Next steps:")
	writef(stdout, "  cd %s\n", name)
	if *wasm {
		writeln(stdout, "  GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm")
		writef(stdout, "  sky plugin install --path ./plugin.wasm %s\n", name)
	} else {
		writeln(stdout, "  go build -o plugin")
		writef(stdout, "  sky plugin install --path ./plugin %s\n", name)
	}
	writef(stdout, "  sky %s\n", name)

	return 0
}

func writePluginTemplate(name string, wasm bool) error {
	var mainGo string

	if wasm {
		// WASM-specific template
		mainGo = fmt.Sprintf(`//go:build wasip1

// Package main implements a WASM Sky plugin.
//
// WASM plugins run in a sandboxed environment with limited capabilities:
//   - No direct filesystem access
//   - No network access
//   - Limited memory (~16MB default)
//
// Build with:
//
//	GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm
//
// Or with TinyGo for smaller binaries:
//
//	tinygo build -o plugin.wasm -target=wasip1 .
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	pluginName    = %q
	pluginVersion = "0.1.0"
	pluginSummary = "A Sky WASM plugin"
)

func main() {
	// Verify we're running as a Sky plugin
	if os.Getenv("SKY_PLUGIN") != "1" {
		fmt.Fprintf(os.Stderr, "This is a Sky plugin. Run it with: sky %%s\n", pluginName)
		os.Exit(1)
	}

	// Handle metadata request from sky
	if os.Getenv("SKY_PLUGIN_MODE") == "metadata" {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"api_version": 1,
			"name":        pluginName,
			"version":     pluginVersion,
			"summary":     pluginSummary,
			"commands": []map[string]string{
				{"name": pluginName, "summary": pluginSummary},
			},
		})
		return
	}

	// Run the plugin
	run()
}

func run() {
	args := os.Args[1:]

	// Simple argument parsing (no flag package for TinyGo compatibility)
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printHelp()
			return
		case "-v", "--version":
			fmt.Printf("%%s %%s\n", pluginName, pluginVersion)
			return
		}
	}

	// Your plugin logic here
	fmt.Println("Hello from WASM plugin:", pluginName)

	// Access workspace info via environment variables
	if root := os.Getenv("SKY_WORKSPACE_ROOT"); root != "" {
		fmt.Println("Workspace:", root)
	}

	// Note: WASM plugins cannot access the filesystem directly
	fmt.Println("(Running in WASM sandbox)")
}

func printHelp() {
	fmt.Printf("Usage: %%s [options]\n\n", pluginName)
	fmt.Println("A Sky WASM plugin.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, --help      Show this help message")
	fmt.Println("  -v, --version   Show version")
	fmt.Println()
	fmt.Println("Environment Variables (set by Sky):")
	fmt.Println("  SKY_WORKSPACE_ROOT  Workspace root directory")
	fmt.Println("  SKY_CONFIG_DIR      Sky configuration directory")
	fmt.Println("  SKY_OUTPUT_FORMAT   Preferred output format (text/json)")
}
`, name)
	} else {
		// Native template
		mainGo = fmt.Sprintf(`package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const (
	pluginName    = %q
	pluginVersion = "0.1.0"
	pluginSummary = "A Sky plugin"
)

func main() {
	// Verify we're running as a Sky plugin
	if os.Getenv("SKY_PLUGIN") != "1" {
		fmt.Fprintf(os.Stderr, "This is a Sky plugin. Run it with: sky %%s\n", pluginName)
		os.Exit(1)
	}

	// Handle metadata request from sky
	if os.Getenv("SKY_PLUGIN_MODE") == "metadata" {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"api_version": 1,
			"name":        pluginName,
			"version":     pluginVersion,
			"summary":     pluginSummary,
			"commands": []map[string]string{
				{"name": pluginName, "summary": pluginSummary},
			},
		})
		return
	}

	// Run the plugin
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet(pluginName, flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "show version")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Printf("%%s %%s\n", pluginName, pluginVersion)
		return 0
	}

	// Your plugin logic here
	fmt.Println("Hello from", pluginName)

	// Access workspace info via environment variables
	if root := os.Getenv("SKY_WORKSPACE_ROOT"); root != "" {
		fmt.Println("Workspace:", root)
	}

	return 0
}
`, name)
	}

	if err := os.WriteFile(filepath.Join(name, "main.go"), []byte(mainGo), 0644); err != nil {
		return err
	}

	// Write go.mod
	goMod := fmt.Sprintf(`module %s

go 1.21
`, name)

	if err := os.WriteFile(filepath.Join(name, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	// Write README.md
	var readme string
	if wasm {
		readme = fmt.Sprintf(`# %s

A Sky WASM plugin.

## Build

### With Go (larger binary, full compatibility)

`+"```bash\nGOOS=wasip1 GOARCH=wasm go build -o plugin.wasm\n```"+`

### With TinyGo (smaller binary)

`+"```bash\ntinygo build -o plugin.wasm -target=wasip1 .\n```"+`

## Install

`+"```bash\nsky plugin install --path ./plugin.wasm %s\n```"+`

## Usage

`+"```bash\nsky %s\n```"+`

## WASI Limitations

WASM plugins run in a sandboxed environment:

- **No filesystem access** - Use environment variables for paths
- **No network access** - All I/O through stdin/stdout
- **Limited memory** - Default ~16MB

For filesystem-heavy operations, consider a native plugin.

## Environment Variables

These are set by Sky when running your plugin:

| Variable | Description |
|----------|-------------|
| `+"`SKY_PLUGIN`"+` | Always "1" when running as a plugin |
| `+"`SKY_PLUGIN_MODE`"+` | "exec" or "metadata" |
| `+"`SKY_PLUGIN_NAME`"+` | The plugin's registered name |
| `+"`SKY_WORKSPACE_ROOT`"+` | Workspace root directory |
| `+"`SKY_CONFIG_DIR`"+` | Sky configuration directory |
| `+"`SKY_OUTPUT_FORMAT`"+` | "text" or "json" |
`, name, name, name)
	} else {
		readme = fmt.Sprintf(`# %s

A Sky plugin.

## Build

`+"```bash\ngo build -o plugin\n```"+`

## Install

`+"```bash\nsky plugin install --path ./plugin %s\n```"+`

## Usage

`+"```bash\nsky %s\n```"+`

## Environment Variables

These are set by Sky when running your plugin:

| Variable | Description |
|----------|-------------|
| `+"`SKY_PLUGIN`"+` | Always "1" when running as a plugin |
| `+"`SKY_PLUGIN_MODE`"+` | "exec" or "metadata" |
| `+"`SKY_PLUGIN_NAME`"+` | The plugin's registered name |
| `+"`SKY_WORKSPACE_ROOT`"+` | Workspace root directory |
| `+"`SKY_CONFIG_DIR`"+` | Sky configuration directory |
| `+"`SKY_OUTPUT_FORMAT`"+` | "text" or "json" |
`, name, name, name)
	}

	if err := os.WriteFile(filepath.Join(name, "README.md"), []byte(readme), 0644); err != nil {
		return err
	}

	return nil
}

func printPluginUsage(w io.Writer) {
	writeln(w, "usage: sky plugin <command> [args]")
	writeln(w)
	writeln(w, "commands:")
	writeln(w, "  init <name>              create a new plugin project")
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
