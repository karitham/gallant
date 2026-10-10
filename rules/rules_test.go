package rules

import (
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

func TestParseErrors(t *testing.T) {
	tests := map[string]string{
		"not a mapping":            "---\nnot a key value line\n---\nbody\n",
		"no frontmatter":           "Just a rule body.\n",
		"unterminated":             "---\ntype: noul\nappliesTo: \"*.go\"\nno closing delimiter\n",
		"missing type":             "---\nappliesTo: \"*.go\"\n---\nbody\n",
		"unknown type":             "---\ntype: nope\n---\nbody\n",
		"choice no criteria":       "---\ntype: choice\n---\nbody\n",
		"score no criteria":        "---\ntype: score\n---\nbody\n",
		"minConfidence above one":  "---\ntype: noul\nappliesTo: \"*.go\"\nminConfidence: 1.5\n---\nbody\n",
		"minConfidence below zero": "---\ntype: noul\nappliesTo: \"*.go\"\nminConfidence: -0.1\n---\nbody\n",
		"minConfidence on choice":  "---\ntype: choice\ncriteria:\n  a: \"one\"\nminConfidence: 0.8\n---\nbody\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(src)); err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
		})
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

func TestRuleInstructions(t *testing.T) {
	tests := map[string]struct {
		rule Rule
		want string
	}{
		"noul is framed as a question": {
			rule: Rule{Type: TypeNoul, Body: "Prefer common words.\n"},
			want: "Does this file violate the following rule?\n\nPrefer common words.\n",
		},
		"choice is used verbatim": {
			rule: Rule{Type: TypeChoice, Body: "Pick one.\n"},
			want: "Pick one.\n",
		},
		"score is used verbatim": {
			rule: Rule{Type: TypeScore, Body: "Rate it.\n"},
			want: "Rate it.\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.rule.Instructions(); got != tt.want {
				t.Errorf("Instructions() = %q, want %q", got, tt.want)
			}
		})
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
