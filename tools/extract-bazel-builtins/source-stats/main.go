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

	withDocs := 0
	totalAttrs := 0
	attrsWithDocs := 0

	for _, rule := range buildLang.Rule {
		if rule.GetDocumentation() != "" {
			withDocs++
		}
		for _, attr := range rule.GetAttribute() {
			totalAttrs++
			if attr.GetDocumentation() != "" {
				attrsWithDocs++
			}
		}
	}

	fmt.Printf("Source Statistics:\n")
	fmt.Printf("  Total rules: %d\n", len(buildLang.Rule))
	fmt.Printf("  Rules with documentation: %d (%.1f%%)\n", withDocs, float64(withDocs)*100/float64(len(buildLang.Rule)))
	fmt.Printf("  Total attributes: %d\n", totalAttrs)
	fmt.Printf("  Attributes with documentation: %d (%.1f%%)\n", attrsWithDocs, float64(attrsWithDocs)*100/float64(totalAttrs))
}
