package rules

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		appliesTo []string
		body      string
	}{
		{
			name:      "scalar appliesTo",
			src:       "---\ntype: noul\nappliesTo: \"**/*.go\"\n---\nCheck for things.\n",
			appliesTo: []string{"**/*.go"},
			body:      "Check for things.\n",
		},
		{
			name:      "unquoted scalar appliesTo",
			src:       "---\ntype: noul\nappliesTo: src/**\n---\nbody\n",
			appliesTo: []string{"src/**"},
			body:      "body\n",
		},
		{
			name:      "inline list",
			src:       "---\ntype: noul\nappliesTo: [\"**/*.go\", '**/*.ts']\n---\nbody\n",
			appliesTo: []string{"**/*.go", "**/*.ts"},
			body:      "body\n",
		},
		{
			name:      "block list",
			src:       "---\ntype: noul\nappliesTo:\n  - \"**/*.go\"\n  - \"**/*.ts\"\n---\nbody\n",
			appliesTo: []string{"**/*.go", "**/*.ts"},
			body:      "body\n",
		},
		{
			name:      "other fields and comments ignored",
			src:       "---\n# a comment\nseverity: error\ntype: noul\nappliesTo: \"*.go\"\n---\nbody\n",
			appliesTo: []string{"*.go"},
			body:      "body\n",
		},
		{
			name:      "body preserved verbatim including dashes and blank lines",
			src:       "---\ntype: noul\nappliesTo: \"a\"\n---\n\nline one\n---\nline two\n",
			appliesTo: []string{"a"},
			body:      "\nline one\n---\nline two\n",
		},
		{
			name:      "quoted value containing a colon",
			src:       "---\ntype: noul\nappliesTo: \"**/*.{go,ts}\"\n---\nbody\n",
			appliesTo: []string{"**/*.{go,ts}"},
			body:      "body\n",
		},
		{
			name:      "empty appliesTo yields no patterns",
			src:       "---\ntype: noul\nappliesTo:\n---\nbody\n",
			appliesTo: nil,
			body:      "body\n",
		},
		{
			name:      "CRLF line endings",
			src:       "---\r\ntype: noul\r\nappliesTo: \"*.go\"\r\n---\r\nbody\r\n",
			appliesTo: []string{"*.go"},
			body:      "body\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := Parse([]byte(tt.src))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if rule.Type != TypeNoul {
				t.Errorf("Type = %q, want %q", rule.Type, TypeNoul)
			}
			if !reflect.DeepEqual(rule.AppliesTo, tt.appliesTo) {
				t.Errorf("AppliesTo = %#v, want %#v", rule.AppliesTo, tt.appliesTo)
			}
			if rule.Body != tt.body {
				t.Errorf("Body = %q, want %q", rule.Body, tt.body)
			}
		})
	}
}

func TestParseChoice(t *testing.T) {
	src := "---\ntype: choice\ncriteria:\n  a: \"one\"\n  b: \"two\"\n---\nbody\n"
	rule, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if rule.Type != TypeChoice {
		t.Errorf("Type = %q, want %q", rule.Type, TypeChoice)
	}
	want := map[string]string{"a": "one", "b": "two"}
	if !reflect.DeepEqual(rule.Criteria.Options, want) {
		t.Errorf("Criteria.Options = %#v, want %#v", rule.Criteria.Options, want)
	}
}

func TestParseScore(t *testing.T) {
	src := "---\ntype: score\ncriteria:\n  - \"low\"\n  - \"high\"\n---\nbody\n"
	rule, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if rule.Type != TypeScore {
		t.Errorf("Type = %q, want %q", rule.Type, TypeScore)
	}
	want := []string{"low", "high"}
	if !reflect.DeepEqual(rule.Criteria.Levels, want) {
		t.Errorf("Criteria.Levels = %#v, want %#v", rule.Criteria.Levels, want)
	}
}

func TestParseMinConfidence(t *testing.T) {
	tests := map[string]struct {
		src  string
		want float64
	}{
		"noul defaults to 0.5": {
			src:  "---\ntype: noul\nappliesTo: \"*.go\"\n---\nbody\n",
			want: 0.5,
		},
		"noul reads the declared value": {
			src:  "---\ntype: noul\nappliesTo: \"*.go\"\nminConfidence: 0.8\n---\nbody\n",
			want: 0.8,
		},
		"choice leaves it unset": {
			src:  "---\ntype: choice\ncriteria:\n  a: \"one\"\n---\nbody\n",
			want: 0,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rule, err := Parse([]byte(tt.src))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if rule.MinConfidence != tt.want {
				t.Errorf("MinConfidence = %v, want %v", rule.MinConfidence, tt.want)
			}
		})
	}
}

func TestParseViolation(t *testing.T) {
	tests := map[string]struct {
		src  string
		want *Violation
	}{
		"choice": {
			src: "---\ntype: choice\ncriteria:\n  useful: \"explains intent\"\n  useless: \"restates the code\"\nviolation:\n  if:\n    useless: 0.5\n  unless:\n    useful: 0.7\n---\nbody\n",
			want: &Violation{
				If:     map[string]float64{"useless": 0.5},
				Unless: map[string]float64{"useful": 0.7},
			},
		},
		"score": {
			src:  "---\ntype: score\ncriteria:\n  - \"Easy\"\n  - \"Hard\"\nviolation:\n  if:\n    Hard: 0.6\n---\nbody\n",
			want: &Violation{If: map[string]float64{"Hard": 0.6}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rule, err := Parse([]byte(tt.src))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(rule.Violation, tt.want) {
				t.Errorf("Violation = %#v, want %#v", rule.Violation, tt.want)
			}
		})
	}
}

func TestParseViolationAbsent(t *testing.T) {
	rule, err := Parse([]byte("---\ntype: choice\ncriteria:\n  a: \"one\"\n---\nbody\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if rule.Violation != nil {
		t.Errorf("Violation = %#v, want nil when the field is absent", rule.Violation)
	}
}

func TestViolationFails(t *testing.T) {
	v := &Violation{
		If:     map[string]float64{"useless": 0.5},
		Unless: map[string]float64{"useful": 0.7},
	}
	tests := map[string]struct {
		probability map[string]float64
		fail        bool
		reason      string
	}{
		"above the threshold":              {map[string]float64{"useless": 0.6}, true, "useless 0.6 > 0.5"},
		"at the threshold":                 {map[string]float64{"useless": 0.5}, false, "no if result above its threshold"},
		"below the threshold":              {map[string]float64{"useless": 0.2}, false, "no if result above its threshold"},
		"unless cancels":                   {map[string]float64{"useless": 0.6, "useful": 0.8}, false, "unless useful 0.8 > 0.7"},
		"unless at the threshold is inert": {map[string]float64{"useless": 0.6, "useful": 0.7}, true, "useless 0.6 > 0.5"},
		"missing result counts as zero":    {map[string]float64{}, false, "no if result above its threshold"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fail, reason := v.Fails(tt.probability)
			if fail != tt.fail {
				t.Errorf("Fails() = %v, want %v", fail, tt.fail)
			}
			if reason != tt.reason {
				t.Errorf("reason = %q, want %q", reason, tt.reason)
			}
		})
	}
}

func TestViolationFailsAnyTrigger(t *testing.T) {
	v := &Violation{If: map[string]float64{"alpha": 0.5, "beta": 0.7}}

	// Either entry alone is enough to fail, not only the one named first.
	if fail, _ := v.Fails(map[string]float64{"beta": 0.8}); !fail {
		t.Error("Fails() = false for beta above its threshold, want true")
	}
	// The reason names the first firing entry in name order, so it is stable.
	fail, reason := v.Fails(map[string]float64{"beta": 0.8, "alpha": 0.6})
	if !fail || reason != "alpha 0.6 > 0.5" {
		t.Errorf("Fails() = %v, %q; want true, %q", fail, reason, "alpha 0.6 > 0.5")
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]string{
		"not a mapping":             "---\nnot a key value line\n---\nbody\n",
		"no frontmatter":            "Just a rule body.\n",
		"unterminated":              "---\ntype: noul\nappliesTo: \"*.go\"\nno closing delimiter\n",
		"missing type":              "---\nappliesTo: \"*.go\"\n---\nbody\n",
		"unknown type":              "---\ntype: nope\n---\nbody\n",
		"choice no criteria":        "---\ntype: choice\n---\nbody\n",
		"score no criteria":         "---\ntype: score\n---\nbody\n",
		"minConfidence above one":   "---\ntype: noul\nappliesTo: \"*.go\"\nminConfidence: 1.5\n---\nbody\n",
		"minConfidence below zero":  "---\ntype: noul\nappliesTo: \"*.go\"\nminConfidence: -0.1\n---\nbody\n",
		"minConfidence on choice":   "---\ntype: choice\ncriteria:\n  a: \"one\"\nminConfidence: 0.8\n---\nbody\n",
		"violation on noul":         "---\ntype: noul\nviolation:\n  if:\n    fail: 0.5\n---\nbody\n",
		"violation without if":      "---\ntype: choice\ncriteria:\n  a: \"one\"\nviolation:\n  unless:\n    a: 0.5\n---\nbody\n",
		"violation unknown result":  "---\ntype: choice\ncriteria:\n  a: \"one\"\nviolation:\n  if:\n    b: 0.5\n---\nbody\n",
		"violation misspelled key":  "---\ntype: choice\ncriteria:\n  a: \"one\"\nviolation:\n  iff:\n    a: 0.5\n---\nbody\n",
		"violation above one":       "---\ntype: choice\ncriteria:\n  a: \"one\"\nviolation:\n  if:\n    a: 1.5\n---\nbody\n",
		"violation below zero":      "---\ntype: choice\ncriteria:\n  a: \"one\"\nviolation:\n  if:\n    a: -0.1\n---\nbody\n",
		"violation duplicate level": "---\ntype: score\ncriteria:\n  - \"Easy\"\n  - \"Easy\"\nviolation:\n  if:\n    Easy: 0.5\n---\nbody\n",
		"bad appliesTo glob":        "---\ntype: noul\nappliesTo: \"[\"\n---\nbody\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(src)); err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
		})
	}
}

func TestParseFrontmatterErrors(t *testing.T) {
	// A rule with no leading delimiter and one whose delimiter is never closed
	// are different mistakes, so they report different errors.
	if _, err := Parse([]byte("Just a rule body.\n")); !errors.Is(err, errNoFrontmatter) {
		t.Errorf("Parse(no frontmatter) error = %v, want errNoFrontmatter", err)
	}
	if _, err := Parse([]byte("---\ntype: noul\n")); !errors.Is(err, errUnclosedFrontmatter) {
		t.Errorf("Parse(unclosed) error = %v, want errUnclosedFrontmatter", err)
	}
}

func TestRuleApplies(t *testing.T) {
	rule := Rule{AppliesTo: []string{"*.go", "*.ts"}}
	tests := map[string]bool{
		"main.go":     true,
		"a.ts":        true,
		"a.py":        false,
		"main.go.bak": false,
	}
	for name, want := range tests {
		if got := rule.Applies(name); got != want {
			t.Errorf("Applies(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestRuleAppliesNoGlobs(t *testing.T) {
	if (Rule{}).Applies("main.go") {
		t.Error("Applies() = true for a rule with no globs, want false")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeRule(t, dir, "a.md", "---\ntype: noul\nappliesTo: \"*.go\"\n---\nfirst\n")
	writeRule(t, dir, "nested/b.markdown", "---\ntype: noul\nappliesTo:\n  - \"*.ts\"\n---\nsecond\n")
	writeRule(t, dir, "notes.txt", "not a rule\n")
	writeRule(t, dir, ".hidden/c.md", "---\ntype: noul\nappliesTo: \"*.py\"\n---\nhidden\n")

	rules, err := Load(os.DirFS(dir))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("Load() returned %d rules, want 2", len(rules))
	}

	if got, want := rules[0].Path, "a.md"; got != want {
		t.Errorf("rules[0].Path = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(rules[0].AppliesTo, []string{"*.go"}) {
		t.Errorf("rules[0].AppliesTo = %#v, want [*.go]", rules[0].AppliesTo)
	}
	if rules[0].Body != "first\n" {
		t.Errorf("rules[0].Body = %q, want %q", rules[0].Body, "first\n")
	}

	if got, want := rules[1].Path, "nested/b.markdown"; got != want {
		t.Errorf("rules[1].Path = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(rules[1].AppliesTo, []string{"*.ts"}) {
		t.Errorf("rules[1].AppliesTo = %#v, want [*.ts]", rules[1].AppliesTo)
	}
}

func TestLoadMissingDir(t *testing.T) {
	if _, err := Load(os.DirFS(filepath.Join(t.TempDir(), "does-not-exist"))); err == nil {
		t.Fatal("Load() error = nil, want error for missing directory")
	}
}

func writeRule(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
