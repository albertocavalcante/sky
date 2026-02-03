package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skydoc"
)

func main() {
	os.Exit(skydoc.Run(os.Args[1:]))
}
