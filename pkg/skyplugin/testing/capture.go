package testing

import (
	"bytes"
	"io"
	"os"
	"sync"
)

// CaptureResult holds captured output from a function.
type CaptureResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CaptureOutput captures stdout and stderr from a function.
// It also catches os.Exit calls and records the exit code.
//
// Note: This function is not safe for concurrent use and should
// only be used in tests.
//
// Usage:
//
//	result := testing.CaptureOutput(func() {
//		fmt.Println("Hello")
//		os.Exit(0)
//	})
//	if result.Stdout != "Hello\n" {
//		t.Error("unexpected output")
//	}
func CaptureOutput(fn func()) CaptureResult {
	result := CaptureResult{ExitCode: -1}

	// Capture stdout - errors intentionally ignored for testing utility
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Capture stderr
	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	// Capture output in goroutines
	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(&outBuf, rOut)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(&errBuf, rErr)
	}()

	// Run the function with panic recovery for os.Exit
	func() {
		defer func() {
			if r := recover(); r != nil {
				if exitCode, ok := r.(exitPanic); ok {
					result.ExitCode = int(exitCode)
				} else {
					panic(r) // Re-panic for other panics
				}
			}
		}()

		fn()
		result.ExitCode = 0
	}()

	// Restore and collect output
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	wg.Wait()

	result.Stdout = outBuf.String()
	result.Stderr = errBuf.String()

	return result
}

// exitPanic is used to catch os.Exit calls in tests.
// Note: This only works if os.Exit is replaced with ExitFunc.
type exitPanic int

// ExitFunc can be set to capture os.Exit calls in tests.
// Set this before calling the function under test.
//
// Usage:
//
//	testing.ExitFunc = func(code int) { panic(testing.ExitCode(code)) }
//	defer func() { testing.ExitFunc = nil }()
var ExitFunc func(code int)

// ExitCode is used with panic to simulate os.Exit in tests.
type ExitCode int

// CaptureOutputSimple captures stdout and stderr without exit handling.
// Use this when the function under test doesn't call os.Exit.
func CaptureOutputSimple(fn func()) (stdout, stderr string) {
	// Capture stdout - errors intentionally ignored for testing utility
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Capture stderr
	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	// Capture output in goroutines
	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(&outBuf, rOut)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(&errBuf, rErr)
	}()

	fn()

	// Restore and collect output
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	wg.Wait()

	return outBuf.String(), errBuf.String()
}
