// Command release prints immutable snapshot build metadata.
//
// The project is not using release tags yet. A build is identified by the
// commit hash and the commit timestamp, formatted as a Go pseudo-version:
//
//	v0.0.0-YYYYMMDDHHMMSS-abcdef123456
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 2 {
		fatal("usage: release [version|ldflags]")
	}

	command := "version"
	if len(os.Args) == 2 {
		command = os.Args[1]
	}

	info, err := currentBuild()
	if err != nil {
		fatal("%v", err)
	}

	switch command {
	case "version":
		fmt.Println(info.Version)
	case "ldflags":
		fmt.Printf("-X github.com/albertocavalcante/sky/internal/version.version=%s ", info.Version)
		fmt.Printf("-X github.com/albertocavalcante/sky/internal/version.commit=%s ", info.Commit)
		fmt.Printf("-X github.com/albertocavalcante/sky/internal/version.date=%s\n", info.Date)
	default:
		fatal("unknown command %q", command)
	}
}

type buildInfo struct {
	Version string
	Commit  string
	Date    string
}

func currentBuild() (buildInfo, error) {
	commit, err := git("rev-parse", "HEAD")
	if err != nil {
		return buildInfo{}, err
	}
	shortCommit, err := git("rev-parse", "--short=12", "HEAD")
	if err != nil {
		return buildInfo{}, err
	}
	date, err := git("show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return buildInfo{}, err
	}
	parsed, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return buildInfo{}, fmt.Errorf("parsing commit date %q: %w", date, err)
	}

	timestamp := parsed.UTC().Format("20060102150405")
	return buildInfo{
		Version: "v0.0.0-" + timestamp + "-" + shortCommit,
		Commit:  commit,
		Date:    date,
	}, nil
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
