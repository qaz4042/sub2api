package dto

import (
	"testing"
)

func TestSafeRawJSONArray(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "valid array", raw: `[{"type":"email"}]`, want: `[{"type":"email"}]`},
		{name: "empty", raw: "", want: "[]"},
		{name: "object", raw: `{"type":"email"}`, want: "[]"},
		{name: "null", raw: "null", want: "[]"},
		{name: "invalid", raw: "[", want: "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(SafeRawJSONArray(tt.raw)); got != tt.want {
				t.Fatalf("SafeRawJSONArray(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
