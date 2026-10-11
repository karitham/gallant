package main

import (
	"io"
	"strings"
	"testing"
)

// FuzzDiffInputs checks the hand-rolled diff splitter: it never panics, and each
// input it yields has a single-line, non-empty name and a body that begins with
// the diff marker.
func FuzzDiffInputs(f *testing.F) {
	for _, seed := range []string{
		"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
		"commit x\n\ndiff --git a/a.go b/a.go\n",
		"diff --git \n",
		"diff --git a/a.go\n",
		"diff --git a/a.go b/a.go\n+diff --git a/b.go b/b.go\n",
		"not a diff",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		inputs, err := diffInputs(strings.NewReader(text), nil)
		if err != nil {
			return
		}
		for _, in := range inputs {
			if in.name == "" {
				t.Fatal("diffInputs returned an input with an empty name")
			}
			if strings.ContainsAny(in.name, "\n\r") {
				t.Fatalf("input name %q contains a newline", in.name)
			}
			r, err := in.open()
			if err != nil {
				t.Fatalf("open(): %v", err)
			}
			body, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if !strings.HasPrefix(string(body), "diff --git ") {
				t.Fatalf("input body does not start with the diff marker: %q", body)
			}
		}
	})
}

// FuzzRoffEscape checks the man-page escaper: no output line can be read as a
// roff request, and the escape is reversible.
func FuzzRoffEscape(f *testing.F) {
	for _, seed := range []string{
		"plain\n",
		".SH x\n",
		"'quote\n",
		"back\\slash\n",
		".\n'.\n",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, s string) {
		escaped := roffEscape(s)
		for _, line := range strings.SplitAfter(escaped, "\n") {
			if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "'") {
				t.Fatalf("escaped output has a request line: %q", line)
			}
		}
		if got := unescapeRoff(escaped); got != s {
			t.Fatalf("escape not reversible:\n in: %q\nout: %q", s, got)
		}
	})
}

// unescapeRoff reverses roffEscape: it drops a leading "\&" and halves each
// backslash.
func unescapeRoff(s string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		b.WriteString(strings.ReplaceAll(strings.TrimPrefix(line, `\&`), `\\`, `\`))
	}
	return b.String()
}
