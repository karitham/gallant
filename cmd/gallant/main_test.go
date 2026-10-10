package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

func TestResolveInputsExplicitFileAlwaysIncluded(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "notes.txt", "x")

	paths, err := resolveInputs([]string{filepath.Join(dir, "notes.txt")}, func(string) bool { return false })
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("resolveInputs() returned %d paths, want 1", len(paths))
	}
	if got, want := paths[0].name, filepath.Join(dir, "notes.txt"); got != want {
		t.Errorf("paths[0].name = %q, want %q", got, want)
	}
}

func TestResolveInputsDirectoryFiltersByMatch(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, "b.ts", "")
	write(t, dir, "nested/c.go", "")

	paths, err := resolveInputs([]string{dir}, func(name string) bool {
		return filepath.Ext(name) == ".go"
	})
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("resolveInputs() returned %d paths, want 2", len(paths))
	}
	if got, want := paths[0].name, filepath.Join(dir, "a.go"); got != want {
		t.Errorf("paths[0].name = %q, want %q", got, want)
	}
	if got, want := paths[1].name, filepath.Join(dir, "nested", "c.go"); got != want {
		t.Errorf("paths[1].name = %q, want %q", got, want)
	}
}

func TestResolveInputsNilMatchIncludesAll(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, "b.ts", "")

	paths, err := resolveInputs([]string{dir}, nil)
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("resolveInputs() returned %d paths, want 2", len(paths))
	}
}

func TestResolveInputsMissingPath(t *testing.T) {
	if _, err := resolveInputs([]string{filepath.Join(t.TempDir(), "missing")}, nil); err == nil {
		t.Fatal("resolveInputs() error = nil, want error for missing path")
	}
}

func TestApplicableSelectsByFileName(t *testing.T) {
	all := []rules.Rule{
		{AppliesTo: []string{"*.go"}},
		{AppliesTo: []string{"*.ts"}},
	}
	got := applicable(all, "main.go")
	if len(got) != 1 || got[0].AppliesTo[0] != "*.go" {
		t.Errorf("applicable() = %#v, want only the *.go rule", got)
	}
}

func TestEvaluatePassesReaderAndSelectedRules(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.go", "package main")

	model := &fakeModel{}
	if _, err := evaluate(context.Background(), fileInput(filepath.Join(dir, "main.go")), []rules.Rule{
		{AppliesTo: []string{"*.go"}},
		{AppliesTo: []string{"*.ts"}},
	}, model); err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if len(model.rules) != 1 || model.rules[0].AppliesTo[0] != "*.go" {
		t.Errorf("model rules = %#v, want only the *.go rule", model.rules)
	}
	if model.content != "package main" {
		t.Errorf("model content = %q, want %q", model.content, "package main")
	}
}

// fakeModel reads the whole stream at evaluate time, as a real model does.
type fakeModel struct {
	content string
	rules   []rules.Rule
}

func (m *fakeModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	m.content = string(b)
	m.rules = rs
	return nil, nil
}

func TestEvaluateAllPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "slow")
	write(t, dir, "b.go", "fast")
	write(t, dir, "c.go", "medium")
	paths := []input{
		fileInput(filepath.Join(dir, "a.go")),
		fileInput(filepath.Join(dir, "b.go")),
		fileInput(filepath.Join(dir, "c.go")),
	}
	model := &delayModel{delays: map[string]time.Duration{
		"slow":   60 * time.Millisecond,
		"fast":   0,
		"medium": 20 * time.Millisecond,
	}}

	results, err := evaluateAll(context.Background(), paths, []rules.Rule{{AppliesTo: []string{"*.go"}}}, model, len(paths))
	if err != nil {
		t.Fatalf("evaluateAll() error = %v", err)
	}

	// The fast file finishes first and the slow one last, so a bug that appended
	// results in completion order would surface here.
	want := []string{"slow", "fast", "medium"}
	for i, w := range want {
		if len(results[i]) != 1 || results[i][0].Result != w {
			t.Errorf("results[%d] = %#v, want one finding with result %q", i, results[i], w)
		}
	}
}

func TestEvaluateAllStopsOnError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, "b.go", "")
	paths := []input{fileInput(filepath.Join(dir, "a.go")), fileInput(filepath.Join(dir, "b.go"))}

	if _, err := evaluateAll(context.Background(), paths, []rules.Rule{{AppliesTo: []string{"*.go"}}}, errorModel{}, len(paths)); err == nil {
		t.Fatal("evaluateAll() error = nil, want an error")
	}
}

// delayModel reads the stream, then sleeps before answering, so slower inputs
// finish after faster ones and completion order differs from input order.
type delayModel struct {
	delays map[string]time.Duration
}

func (m *delayModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	timer := time.NewTimer(m.delays[string(b)])
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	return []eval.Finding{{Result: string(b)}}, nil
}

// errorModel always fails.
type errorModel struct{}

func (errorModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	return nil, errors.New("evaluate failed")
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
