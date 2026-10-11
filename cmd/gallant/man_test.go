package main

import "testing"

func TestRoffEscape(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain\n", "plain\n"},
		{`back\slash` + "\n", `back\\slash` + "\n"},
		{".SH x\n", `\&.SH x` + "\n"},
		{"'quote\n", `\&'quote` + "\n"},
	}
	for _, tt := range tests {
		if got := roffEscape(tt.in); got != tt.want {
			t.Errorf("roffEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
