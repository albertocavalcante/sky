package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skyrepl"
)

func main() {
	os.Exit(skyrepl.Run(os.Args[1:]))
}
