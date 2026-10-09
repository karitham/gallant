package rules

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// TestLoadGolden runs Load over a fixture directory and compares a deterministic
// dump of the parsed rules against a golden file.
func TestLoadGolden(t *testing.T) {
	const fixture = "testdata/rules"

	rules, err := Load(os.DirFS(fixture))
	if err != nil {
		t.Fatalf("Load(%q) error = %v", fixture, err)
	}
	golden(t, "rules", renderRules(rules))
}

// golden compares got against testdata/<name>.golden, writing the file instead
// when the test binary is run with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()

	path := "testdata/" + name + ".golden"
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test ./rules -update)", path, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s (run: go test ./rules -update)\n\nwant:\n%s\ngot:\n%s", path, want, got)
	}
}

// renderRules dumps rules in a stable form, quoting every field so that
// whitespace and globs are unambiguous.
func renderRules(rules []Rule) string {
	var b strings.Builder
	for i, r := range rules {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "path: %s\n", r.Path)
		fmt.Fprintf(&b, "appliesTo: %s\n", formatGlobs(r.AppliesTo))
		fmt.Fprintf(&b, "body: %s\n", strconv.Quote(r.Body))
	}
	return b.String()
}

func formatGlobs(globs []string) string {
	quoted := make([]string, len(globs))
	for i, g := range globs {
		quoted[i] = strconv.Quote(g)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
