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

	// Show first 3 values in detail
	fmt.Printf("Detailed view of first 3 values:\n\n")
	for i, val := range builtins.Values {
		if i >= 3 {
			break
		}

		fmt.Printf("=== %d. %s ===\n", i+1, val.Name)
		if val.Doc != "" {
			fmt.Printf("Documentation: %s\n", truncate(val.Doc, 200))
		}

		if val.Callable != nil {
			fmt.Printf("Parameters (%d):\n", len(val.Callable.Params))
			for j, param := range val.Callable.Params {
				if j >= 10 {
					fmt.Printf("  ... and %d more params\n", len(val.Callable.Params)-10)
					break
				}
				mandatory := ""
				if param.IsMandatory {
					mandatory = " [REQUIRED]"
				}
				defaultVal := ""
				if param.DefaultValue != "" {
					defaultVal = fmt.Sprintf(" = %s", param.DefaultValue)
				}
				fmt.Printf("  - %s: %s%s%s\n", param.Name, param.Type, defaultVal, mandatory)
			}
		}
		fmt.Println()
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
