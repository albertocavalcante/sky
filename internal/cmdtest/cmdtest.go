// Package cmdtest provides a testscript-based test harness for Sky CLI tools.
//
// It uses txtar format test files to specify input files and expected outputs,
// making it easy to write comprehensive CLI tests.
//
// Example test file (testdata/skylint/unused_load.txtar):
//
//	# Test that skylint catches unused loads
//	exec skylint test.star
//	stdout 'unused-load'
//	! stdout 'error'
//
//	-- test.star --
//	load("//lib:foo.bzl", "unused_symbol")
//
//	def main():
//	    pass
package cmdtest

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/albertocavalcante/sky/internal/cmd/skycheck"
	"github.com/albertocavalcante/sky/internal/cmd/skydoc"
	"github.com/albertocavalcante/sky/internal/cmd/skyfmt"
	"github.com/albertocavalcante/sky/internal/cmd/skylint"
	"github.com/albertocavalcante/sky/internal/cmd/skyls"
	"github.com/albertocavalcante/sky/internal/cmd/skyquery"
)

// Run executes the testscript tests in the given directory.
func Run(t *testing.T, dir string) {
	testscript.Run(t, testscript.Params{
		Dir: dir,
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// Custom commands can be added here if needed
		},
		Setup: func(env *testscript.Env) error {
			// Set up environment variables if needed
			return nil
		},
	})
}

// Main is the TestMain function that should be called from test files.
// It sets up the CLI tools as testscript commands.
func Main(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"skylint":  wrapRun(skylint.Run),
		"skyfmt":   wrapRun(skyfmt.Run),
		"skycheck": wrapRun(skycheck.Run),
		"skyquery": wrapRun(skyquery.Run),
		"skydoc":   wrapRun(skydoc.Run),
		"skyls":    wrapRun(skyls.Run),
	}))
}

// wrapRun wraps a Run(args []string) int function to func() int for testscript.
// The args are taken from os.Args[1:].
func wrapRun(run func(args []string) int) func() int {
	return func() int {
		return run(os.Args[1:])
	}
}
