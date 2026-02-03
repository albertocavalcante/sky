package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skycov"
)

func main() {
	os.Exit(skycov.Run(os.Args[1:]))
}
