//go:build sky_full

package main

import (
	"github.com/albertocavalcante/sky/internal/cmd/skycheck"
	"github.com/albertocavalcante/sky/internal/cmd/skycov"
	"github.com/albertocavalcante/sky/internal/cmd/skydoc"
	"github.com/albertocavalcante/sky/internal/cmd/skyfmt"
	"github.com/albertocavalcante/sky/internal/cmd/skylint"
	"github.com/albertocavalcante/sky/internal/cmd/skyls"
	"github.com/albertocavalcante/sky/internal/cmd/skyquery"
	"github.com/albertocavalcante/sky/internal/cmd/skyrepl"
	"github.com/albertocavalcante/sky/internal/cmd/skytest"
)

func init() {
	embeddedTools = map[string]EmbeddedTool{
		// Core tools - accessed via aliases (sky fmt, sky lint, etc.)
		"fmt":   skyfmt.RunWithIO,
		"lint":  skylint.RunWithIO,
		"check": skycheck.RunWithIO,
		"query": skyquery.RunWithIO,
		"repl":  skyrepl.RunWithIO,
		"test":  skytest.RunWithIO,
		"doc":   skydoc.RunWithIO,
		"cov":   skycov.RunWithIO,
		"ls":    skyls.RunWithIO,

		// Full binary names for direct access
		"skyfmt":   skyfmt.RunWithIO,
		"skylint":  skylint.RunWithIO,
		"skycheck": skycheck.RunWithIO,
		"skyquery": skyquery.RunWithIO,
		"skyrepl":  skyrepl.RunWithIO,
		"skytest":  skytest.RunWithIO,
		"skydoc":   skydoc.RunWithIO,
		"skycov":   skycov.RunWithIO,
		"skyls":    skyls.RunWithIO,
	}
}
