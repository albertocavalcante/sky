package typemode

import "testing"

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{Disabled, "disabled"},
		{ParseOnly, "parse_only"},
		{Enabled, "enabled"},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("Mode.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		want    Mode
		wantErr bool
	}{
		{"", Disabled, false},
		{"disabled", Disabled, false},
		{"parse_only", ParseOnly, false},
		{"parse-only", ParseOnly, false},
		{"parseonly", ParseOnly, false},
		{"enabled", Enabled, false},
		{"invalid", "", true},
		{"ENABLED", "", true}, // case sensitive
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMode_IsEnabled(t *testing.T) {
	tests := []struct {
		mode Mode
		want bool
	}{
		{Disabled, false},
		{ParseOnly, true},
		{Enabled, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.IsEnabled(); got != tt.want {
				t.Errorf("Mode.IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMode_ShouldCheck(t *testing.T) {
	tests := []struct {
		mode Mode
		want bool
	}{
		{Disabled, false},
		{ParseOnly, false},
		{Enabled, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.ShouldCheck(); got != tt.want {
				t.Errorf("Mode.ShouldCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllModes(t *testing.T) {
	modes := AllModes()
	if len(modes) != 3 {
		t.Errorf("AllModes() returned %d modes, want 3", len(modes))
	}
}
