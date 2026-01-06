package main

import (
	"fmt"
	"os"

	buildpb "github.com/bazelbuild/buildtools/build_proto"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <build-language.pb>\n", os.Args[0])
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var buildLang buildpb.BuildLanguage
	if err := proto.Unmarshal(data, &buildLang); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling proto: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total rules: %d\n\n", len(buildLang.Rule))

	// Show all rule names
	fmt.Println("All rules:")
	for i, rule := range buildLang.Rule {
		name := rule.GetName()
		label := rule.GetLabel()
		attrCount := len(rule.GetAttribute())
		fmt.Printf("%3d. %-40s (label: %-30s, attrs: %d)\n", i+1, name, label, attrCount)
	}
}
