package main

import (
	"fmt"
	"os"

	builtinspb "github.com/albertocavalcante/sky/internal/starlark/builtins/proto"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <proto-file>\n", os.Args[0])
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var builtins builtinspb.Builtins
	if err := proto.Unmarshal(data, &builtins); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling proto: %v\n", err)
		os.Exit(1)
	}

	totalParams := 0
	mandatoryParams := 0
	withDocs := 0
	withDefaults := 0

	for _, val := range builtins.Values {
		if val.Doc != "" {
			withDocs++
		}
		if val.Callable != nil {
			for _, param := range val.Callable.Params {
				totalParams++
				if param.IsMandatory {
					mandatoryParams++
				}
				if param.DefaultValue != "" {
					withDefaults++
				}
			}
		}
	}

	fmt.Printf("Statistics:\n")
	fmt.Printf("  Total values: %d\n", len(builtins.Values))
	fmt.Printf("  Values with documentation: %d (%.1f%%)\n", withDocs, float64(withDocs)*100/float64(len(builtins.Values)))
	fmt.Printf("  Total parameters: %d\n", totalParams)
	fmt.Printf("  Mandatory parameters: %d (%.1f%%)\n", mandatoryParams, float64(mandatoryParams)*100/float64(totalParams))
	fmt.Printf("  Parameters with defaults: %d (%.1f%%)\n", withDefaults, float64(withDefaults)*100/float64(totalParams))
	if len(builtins.Values) > 0 {
		fmt.Printf("  Avg params per value: %.1f\n", float64(totalParams)/float64(len(builtins.Values)))
	}
}
