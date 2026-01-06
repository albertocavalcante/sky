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

	fmt.Printf("File: %s\n", os.Args[1])
	fmt.Printf("Types: %d\n", len(builtins.Types))
	fmt.Printf("Values: %d\n\n", len(builtins.Values))

	// Sample first 10 values
	fmt.Println("Sample values:")
	for i, val := range builtins.Values {
		if i >= 10 {
			break
		}
		paramCount := 0
		if val.Callable != nil {
			paramCount = len(val.Callable.Params)
		}
		fmt.Printf("  %d. %s (%d params)\n", i+1, val.Name, paramCount)
	}

	if len(builtins.Values) > 10 {
		fmt.Printf("  ... and %d more\n", len(builtins.Values)-10)
	}
}
