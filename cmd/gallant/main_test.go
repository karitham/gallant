package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInputsExplicitFileAlwaysIncluded(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "notes.txt", "x")

	files, err := resolveInputs([]string{filepath.Join(dir, "notes.txt")}, func(string) bool { return false })
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	if len(files) != 1 {
		t.Fatalf("resolveInputs() returned %d files, want 1", len(files))
	}
	if got, want := files[0].Name(), filepath.Join(dir, "notes.txt"); got != want {
		t.Errorf("files[0].Name() = %q, want %q", got, want)
	}
}

func TestResolveInputsDirectoryFiltersByMatch(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, "b.ts", "")
	write(t, dir, "nested/c.go", "")

	files, err := resolveInputs([]string{dir}, func(name string) bool {
		return filepath.Ext(name) == ".go"
	})
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	if len(files) != 2 {
		t.Fatalf("resolveInputs() returned %d files, want 2", len(files))
	}
	if got, want := files[0].Name(), filepath.Join(dir, "a.go"); got != want {
		t.Errorf("files[0].Name() = %q, want %q", got, want)
	}
	if got, want := files[1].Name(), filepath.Join(dir, "nested", "c.go"); got != want {
		t.Errorf("files[1].Name() = %q, want %q", got, want)
	}
}

func TestResolveInputsNilMatchIncludesAll(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, "b.ts", "")

	files, err := resolveInputs([]string{dir}, nil)
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	if len(files) != 2 {
		t.Errorf("resolveInputs() returned %d files, want 2", len(files))
	}
}

func TestResolveInputsMissingPath(t *testing.T) {
	if _, err := resolveInputs([]string{filepath.Join(t.TempDir(), "missing")}, nil); err == nil {
		t.Fatal("resolveInputs() error = nil, want error for missing path")
	}
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
