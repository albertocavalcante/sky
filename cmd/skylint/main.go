package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skylint"
)

func main() {
	os.Exit(skylint.Run(os.Args[1:]))
}
