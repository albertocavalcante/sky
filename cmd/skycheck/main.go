package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skycheck"
)

func main() {
	os.Exit(skycheck.Run(os.Args[1:]))
}
