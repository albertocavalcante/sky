package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestExitCodes(t *testing.T) {
	// Verify exit code values match expected Unix conventions
	if ExitOK != 0 {
		t.Errorf("ExitOK = %d, want 0", ExitOK)
	}
	if ExitError != 1 {
		t.Errorf("ExitError = %d, want 1", ExitError)
	}
	if ExitWarning != 2 {
		t.Errorf("ExitWarning = %d, want 2", ExitWarning)
	}
}

func TestWritef(t *testing.T) {
	var buf bytes.Buffer
	Writef(&buf, "hello %s, count=%d", "world", 42)

	got := buf.String()
	want := "hello world, count=42"
	if got != want {
		t.Errorf("Writef() = %q, want %q", got, want)
	}
}

func TestWriteln(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want string
	}{
		{
			name: "no args",
			args: nil,
			want: "\n",
		},
		{
			name: "single arg",
			args: []any{"hello"},
			want: "hello\n",
		},
		{
			name: "multiple args",
			args: []any{"hello", "world", 42},
			want: "hello world 42\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			Writeln(&buf, tc.args...)

			got := buf.String()
			if got != tc.want {
				t.Errorf("Writeln() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	var buf bytes.Buffer
	Write(&buf, "hello world")

	got := buf.String()
	want := "hello world"
	if got != want {
		t.Errorf("Write() = %q, want %q", got, want)
	}
}

func TestWriteBytes(t *testing.T) {
	var buf bytes.Buffer
	WriteBytes(&buf, []byte("hello world"))

	got := buf.String()
	want := "hello world"
	if got != want {
		t.Errorf("WriteBytes() = %q, want %q", got, want)
	}
}

func TestExitCodeError(t *testing.T) {
	err := ExitCodeError(42)

	if !strings.Contains(err.Error(), "42") {
		t.Errorf("ExitCodeError.Error() = %q, want to contain '42'", err.Error())
	}
}

func TestExecute_Version(t *testing.T) {
	cmd := Command{
		Name:    "testcmd",
		Summary: "test command",
		Run:     func(args []string, stdout, stderr io.Writer) error { return nil },
	}

	var stdout, stderr bytes.Buffer
	code := Execute(cmd, []string{"--version"}, &stdout, &stderr)

	if code != ExitOK {
		t.Errorf("Execute() with --version returned %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout.String(), "testcmd") {
		t.Errorf("Execute() version output = %q, want to contain 'testcmd'", stdout.String())
	}
}

func TestExecute_Help(t *testing.T) {
	cmd := Command{
		Name:    "testcmd",
		Summary: "test command",
		Run:     func(args []string, stdout, stderr io.Writer) error { return nil },
	}

	var stdout, stderr bytes.Buffer
	code := Execute(cmd, []string{"--help"}, &stdout, &stderr)

	if code != ExitOK {
		t.Errorf("Execute() with --help returned %d, want %d", code, ExitOK)
	}
}
