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
			src:       "---\nappliesTo: \"**/*.go\"\n---\nCheck for things.\n",
			appliesTo: []string{"**/*.go"},
			body:      "Check for things.\n",
		},
		{
			name:      "unquoted scalar appliesTo",
			src:       "---\nappliesTo: src/**\n---\nbody\n",
			appliesTo: []string{"src/**"},
			body:      "body\n",
		},
		{
			name:      "inline list",
			src:       "---\nappliesTo: [\"**/*.go\", '**/*.ts']\n---\nbody\n",
			appliesTo: []string{"**/*.go", "**/*.ts"},
			body:      "body\n",
		},
		{
			name:      "block list",
			src:       "---\nappliesTo:\n  - \"**/*.go\"\n  - \"**/*.ts\"\n---\nbody\n",
			appliesTo: []string{"**/*.go", "**/*.ts"},
			body:      "body\n",
		},
		{
			name:      "other fields and comments ignored",
			src:       "---\n# a comment\nseverity: error\nappliesTo: \"*.go\"\n---\nbody\n",
			appliesTo: []string{"*.go"},
			body:      "body\n",
		},
		{
			name:      "no frontmatter keeps whole input as body",
			src:       "Just a rule body.\n",
			appliesTo: nil,
			body:      "Just a rule body.\n",
		},
		{
			name:      "unterminated frontmatter is treated as body",
			src:       "---\nappliesTo: \"*.go\"\nno closing delimiter\n",
			appliesTo: nil,
			body:      "---\nappliesTo: \"*.go\"\nno closing delimiter\n",
		},
		{
			name:      "body preserved verbatim including dashes and blank lines",
			src:       "---\nappliesTo: \"a\"\n---\n\nline one\n---\nline two\n",
			appliesTo: []string{"a"},
			body:      "\nline one\n---\nline two\n",
		},
		{
			name:      "quoted value containing a colon",
			src:       "---\nappliesTo: \"**/*.{go,ts}\"\n---\nbody\n",
			appliesTo: []string{"**/*.{go,ts}"},
			body:      "body\n",
		},
		{
			name:      "empty appliesTo yields no patterns",
			src:       "---\nappliesTo:\n---\nbody\n",
			appliesTo: nil,
			body:      "body\n",
		},
		{
			name:      "CRLF line endings",
			src:       "---\r\nappliesTo: \"*.go\"\r\n---\r\nbody\r\n",
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

func TestParserDefaultsToNoul(t *testing.T) {
	rule, err := Parse([]byte("---\nappliesTo: \"*.go\"\n---\nbody\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if rule.Type != TypeNoul {
		t.Errorf("Type = %q, want %q", rule.Type, TypeNoul)
	}
}

func TestParseInvalidFrontmatter(t *testing.T) {
	tests := map[string]string{
		"not a mapping": "---\nnot a key value line\n---\nbody\n",
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

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeRule(t, dir, "a.md", "---\nappliesTo: \"*.go\"\n---\nfirst\n")
	writeRule(t, dir, "nested/b.markdown", "---\nappliesTo:\n  - \"*.ts\"\n---\nsecond\n")
	writeRule(t, dir, "notes.txt", "not a rule\n")
	writeRule(t, dir, ".hidden/c.md", "---\nappliesTo: \"*.py\"\n---\nhidden\n")

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
