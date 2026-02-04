package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skyls"
)

func main() {
	os.Exit(skyls.Run(os.Args[1:]))
}
