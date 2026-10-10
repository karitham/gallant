package eval

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/madxmike/gallant/rules"
)

type fakeModel struct {
	file  *os.File
	rules []rules.Rule
	out   []string
}

func (m *fakeModel) Evaluate(file *os.File, rs []rules.Rule) ([]string, error) {
	m.file = file
	m.rules = rs
	return m.out, nil
}

func TestFileFiltersRulesByFileName(t *testing.T) {
	file := openFile(t, "main.go", "package main")

	all := []rules.Rule{
		{AppliesTo: []string{"*.go"}},
		{AppliesTo: []string{"*.ts"}},
	}
	model := &fakeModel{out: []string{"finding"}}

	got, err := File(file, all, model)
	if err != nil {
		t.Fatalf("File() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"finding"}) {
		t.Errorf("File() = %#v, want [finding]", got)
	}
	if len(model.rules) != 1 || model.rules[0].AppliesTo[0] != "*.go" {
		t.Errorf("model rules = %#v, want only the *.go rule", model.rules)
	}

	text, err := io.ReadAll(model.file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(text) != "package main" {
		t.Errorf("file text = %q, want %q", text, "package main")
	}
}

func TestFileNoApplicableRules(t *testing.T) {
	file := openFile(t, "readme.md", "text")

	all := []rules.Rule{{AppliesTo: []string{"*.go"}}}
	model := &fakeModel{out: []string{"x"}}

	if _, err := File(file, all, model); err != nil {
		t.Fatalf("File() error = %v", err)
	}
	if len(model.rules) != 0 {
		t.Errorf("model rules = %#v, want none", model.rules)
	}
}

func openFile(t *testing.T, name, content string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
