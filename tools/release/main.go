// Command release manages version tags for sky releases.
//
// Usage:
//
//	release              Show latest tags and help
//	release rc 0.1.0     Create release candidate (v0.1.0-rc.0, rc.1, etc.)
//	release final 0.1.0  Create final release (v0.1.0)
//	release delete TAG   Delete a tag locally and remotely
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		showHelp()
		return
	}

	switch os.Args[1] {
	case "rc":
		if len(os.Args) < 3 {
			fatal("usage: release rc VERSION (e.g., release rc 0.1.0)")
		}
		createRC(os.Args[2])
	case "final":
		if len(os.Args) < 3 {
			fatal("usage: release final VERSION (e.g., release final 0.1.0)")
		}
		createFinal(os.Args[2])
	case "delete":
		if len(os.Args) < 3 {
			fatal("usage: release delete TAG (e.g., release delete v0.1.0-rc.0)")
		}
		deleteTag(os.Args[2])
	default:
		showHelp()
	}
}

func showHelp() {
	fmt.Println("Latest tags:")
	tags := getLatestTags(5)
	if len(tags) == 0 {
		fmt.Println("  (no tags yet)")
	} else {
		for _, tag := range tags {
			fmt.Printf("  %s\n", tag)
		}
	}
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  release rc 0.1.0     Create v0.1.0-rc.0 (or increment rc)")
	fmt.Println("  release final 0.1.0  Create v0.1.0")
	fmt.Println("  release delete TAG   Delete a tag")
}

func createRC(version string) {
	version = strings.TrimPrefix(version, "v")

	// Find latest RC for this version
	pattern := regexp.MustCompile(`^v` + regexp.QuoteMeta(version) + `-rc\.(\d+)$`)
	tags := getLatestTags(100)

	var latestRC int = -1
	for _, tag := range tags {
		if m := pattern.FindStringSubmatch(tag); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > latestRC {
				latestRC = n
			}
		}
	}

	newTag := fmt.Sprintf("v%s-rc.%d", version, latestRC+1)
	createAndPushTag(newTag, tags)
}

func createFinal(version string) {
	version = strings.TrimPrefix(version, "v")
	newTag := fmt.Sprintf("v%s", version)

	// Check if tag exists
	tags := getLatestTags(100)
	for _, tag := range tags {
		if tag == newTag {
			fatal("tag %s already exists", newTag)
		}
	}

	createAndPushTag(newTag, tags)
}

func createAndPushTag(newTag string, existingTags []string) {
	fmt.Printf("Creating: %s\n\n", newTag)

	// Show changes
	var baseTag string
	if len(existingTags) > 0 {
		baseTag = existingTags[0]
		fmt.Printf("Changes since %s:\n", baseTag)
		out, _ := exec.Command("git", "log", "--oneline", baseTag+"..HEAD").Output()
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for i, line := range lines {
			if i >= 15 {
				fmt.Printf("  ... and %d more commits\n", len(lines)-15)
				break
			}
			if line != "" {
				fmt.Printf("  %s\n", line)
			}
		}
	} else {
		fmt.Println("Changes (first release):")
		out, _ := exec.Command("git", "log", "--oneline", "-15").Output()
		fmt.Print(string(out))
	}

	fmt.Println()
	if !confirm("Create and push " + newTag + "?") {
		fmt.Println("Aborted.")
		return
	}

	// Create tag
	if err := exec.Command("git", "tag", "-a", newTag, "-m", "Release "+newTag).Run(); err != nil {
		fatal("failed to create tag: %v", err)
	}

	// Push tag
	if err := exec.Command("git", "push", "origin", newTag).Run(); err != nil {
		fatal("failed to push tag: %v", err)
	}

	fmt.Printf("\nTag %s pushed! GitHub Actions will create the release.\n", newTag)
	fmt.Println("View at: https://github.com/albertocavalcante/sky/releases")
}

func deleteTag(tag string) {
	fmt.Printf("Delete tag: %s\n", tag)
	if !confirm("Are you sure?") {
		fmt.Println("Aborted.")
		return
	}

	if err := exec.Command("git", "tag", "-d", tag).Run(); err != nil {
		fmt.Printf("Warning: failed to delete local tag: %v\n", err)
	}
	if err := exec.Command("git", "push", "origin", "--delete", tag).Run(); err != nil {
		fmt.Printf("Warning: failed to delete remote tag: %v\n", err)
	}
	fmt.Printf("Tag %s deleted.\n", tag)
}

func getLatestTags(n int) []string {
	out, err := exec.Command("git", "tag", "--sort=-v:refname").Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	if len(lines) > n {
		return lines[:n]
	}
	return lines
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
