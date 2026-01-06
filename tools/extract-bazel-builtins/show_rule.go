package main

import (
	"fmt"
	"os"

	builtinspb "github.com/albertocavalcante/sky/internal/starlark/builtins/proto"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <proto-file> <rule-name>\n", os.Args[0])
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

	ruleName := os.Args[2]
	for _, val := range builtins.Values {
		if val.Name == ruleName {
			fmt.Printf("Rule: %s\n", val.Name)
			if val.Callable != nil {
				fmt.Printf("Parameters (%d):\n", len(val.Callable.Params))
				for _, param := range val.Callable.Params {
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
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Rule '%s' not found\n", ruleName)
	os.Exit(1)
}
