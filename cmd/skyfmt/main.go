package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skyfmt"
)

func main() {
	os.Exit(skyfmt.Run(os.Args[1:]))
}
