package systemone

import (
	"strings"
	"testing"
)

// FuzzBodySuffix checks the error-body snippet: it never panics, never yields a
// multi-line or unbounded string, and stays empty for blank input.
func FuzzBodySuffix(f *testing.F) {
	for _, seed := range []string{
		"",
		"  \n\t ",
		"boom",
		"a\nb\nc",
		strings.Repeat("x", 5000),
		"<html><body>nope</body></html>",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		got := bodySuffix(strings.NewReader(body))
		if strings.ContainsAny(got, "\n\r") {
			t.Fatalf("bodySuffix(%q) = %q, want a single line", body, got)
		}
		if len(got) > 210 {
			t.Fatalf("bodySuffix(%q) = %q, want it bounded", body, got)
		}
		if strings.TrimSpace(body) == "" && got != "" {
			t.Fatalf("bodySuffix(%q) = %q, want empty for blank input", body, got)
		}
		if got != "" && !strings.HasPrefix(got, ": ") {
			t.Fatalf("bodySuffix(%q) = %q, want a ': ' prefix", body, got)
		}
	})
}
