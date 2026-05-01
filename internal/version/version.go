// Package version provides immutable build identity for sky tools.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

var (
	// These variables are set by release builds via -ldflags -X.
	version = "dev"
	commit  = ""
	date    = ""
)

// Info is the immutable identity of a built binary.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// Current returns the build identity embedded in this binary.
func Current() Info {
	info := Info{
		Version: clean(version, "dev"),
		Commit:  clean(commit, "unknown"),
		Date:    clean(date, "unknown"),
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "unknown" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "unknown" {
					info.Date = setting.Value
				}
			}
		}
	}

	if info.Version == "dev" && info.Commit != "unknown" && info.Date != "unknown" {
		info.Version = "dev-" + shortCommit(info.Commit)
	}

	return info
}

// String returns a stable, human-readable build identity.
func String() string {
	info := Current()
	return fmt.Sprintf("%s (commit %s, built %s)", info.Version, info.Commit, info.Date)
}

func clean(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func shortCommit(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}
