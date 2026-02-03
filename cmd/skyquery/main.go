package main

import (
	"os"

	"github.com/albertocavalcante/sky/internal/cmd/skyquery"
)

func main() {
	os.Exit(skyquery.Run(os.Args[1:]))
}
