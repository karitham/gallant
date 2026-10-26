package rules

import (
	"path"
	"strings"
	"testing"
)

// FuzzParse checks that parsing arbitrary bytes never panics and that anything
// Parse accepts is usable and self-consistent.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"---\ntype: noul\nappliesTo: \"*.go\"\n---\nbody\n",
		"---\ntype: choice\ncriteria:\n  a: one\n  b: two\n---\nbody\n",
		"---\ntype: score\ncriteria:\n  - low\n  - high\n---\nbody\n",
		"---\r\ntype: noul\r\nappliesTo: \"*.go\"\r\n---\r\nbody\r\n",
		"---\n",
		"---",
		"no frontmatter\n",
		"",
		"---\ntype: noul\nminConfidence: 2\n---\n",
		"---\nappliesTo: \"[\"\ntype: noul\n---\n",
		"---\ntype: noul\nappliesTo: [\"*.go\", '*.ts']\n---\n---\n",
		"---\ntype: choice\ncriteria:\n  a: one\nviolation:\n  if:\n    a: 0.5\n---\nbody\n",
		"---\ntype: score\ncriteria:\n  - low\n  - high\nviolation:\n  if:\n    high: 0.5\n  unless:\n    low: 0.5\n---\nbody\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		rule, err := Parse(data)
		if err != nil {
			return
		}
		switch rule.Type {
		case TypeNoul, TypeChoice, TypeScore:
		default:
			t.Fatalf("Parse returned unknown type %q", rule.Type)
		}
		if rule.Type == TypeNoul && (rule.MinConfidence < 0 || rule.MinConfidence > 1) {
			t.Fatalf("noul minConfidence out of range: %v", rule.MinConfidence)
		}
		// A violation condition must be decidable: it only belongs to a choice or
		// score, names at least one trigger, and every threshold is a probability.
		if v := rule.Violation; v != nil {
			if rule.Type == TypeNoul {
				t.Fatal("noul parsed with a violation condition")
			}
			if len(v.If) == 0 {
				t.Fatal("violation parsed with no if entries")
			}
			for _, thresholds := range []map[string]float64{v.If, v.Unless} {
				for name, p := range thresholds {
					if p < 0 || p > 1 {
						t.Fatalf("violation threshold for %q out of range: %v", name, p)
					}
				}
			}
			_, _ = rule.Violation.Fails(map[string]float64{"a": 1})
		}
		// Every accepted glob must be a valid pattern, or the rule could never
		// match and the parse would have been misleading.
		for _, glob := range rule.AppliesTo {
			if _, err := path.Match(glob, ""); err != nil {
				t.Fatalf("Parse accepted invalid glob %q", glob)
			}
		}
		// A parsed rule must be safe to use.
		for _, name := range []string{"", "a.go", "main.go", "a/b/c.md", "."} {
			_ = rule.Applies(name)
		}
	})
}

// FuzzSplitFrontmatter checks the hand-rolled splitter directly: it never panics
// and, when it reports success, the body is a suffix of the input and the
// captured frontmatter holds no delimiter line of its own.
func FuzzSplitFrontmatter(f *testing.F) {
	for _, seed := range []string{
		"---\na: b\n---\nbody\n",
		"---\n---\n",
		"---\n",
		"",
		"---\r\n---\r\nbody",
		"---\n---\n---\n",
		"no\n---\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		fm, body, err := splitFrontmatter(src)
		if err != nil {
			return
		}
		if !strings.HasSuffix(src, body) {
			t.Fatalf("body %q is not a suffix of %q", body, src)
		}
		for _, line := range strings.Split(fm, "\n") {
			if strings.TrimRight(line, "\r") == frontmatterDelimiter {
				t.Fatalf("frontmatter %q contains a delimiter line", fm)
			}
		}
	})
}
