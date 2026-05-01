package version

import "testing"

func TestClean(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		want     string
	}{
		{name: "value", value: " v0.0.0-20260206000000-abcdef123456 ", fallback: "dev", want: "v0.0.0-20260206000000-abcdef123456"},
		{name: "fallback", value: " ", fallback: "dev", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clean(tt.value, tt.fallback); got != tt.want {
				t.Fatalf("clean(%q, %q) = %q, want %q", tt.value, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestShortCommit(t *testing.T) {
	if got := shortCommit("abcdef1234567890"); got != "abcdef123456" {
		t.Fatalf("shortCommit() = %q, want %q", got, "abcdef123456")
	}
	if got := shortCommit("abc"); got != "abc" {
		t.Fatalf("shortCommit() = %q, want %q", got, "abc")
	}
}
