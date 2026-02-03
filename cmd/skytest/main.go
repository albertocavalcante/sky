package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skytest"
)

func main() {
	os.Exit(skytest.Run(os.Args[1:]))
}
