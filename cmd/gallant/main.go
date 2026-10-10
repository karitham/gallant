// Command gallant is the CLI entry point for gallant.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/madxmike/gallant/eval"
	_ "github.com/madxmike/gallant/eval/opencode"
	"github.com/madxmike/gallant/rules"
)

// defaultRulesDir is where gallant looks for lint rules, relative to the
// current working directory.
const defaultRulesDir = "agents/gallant"

func main() {
	provider := flag.String("provider", "opencode", "model provider ("+strings.Join(eval.Names(), ", ")+")")
	model := flag.String("model", "", "model name")
	flag.Parse()

	p, ok := eval.Lookup(*provider)
	if !ok {
		fail(fmt.Errorf("unknown provider %q (want one of: %s)", *provider, strings.Join(eval.Names(), ", ")))
	}
	if *model == "" {
		*model = p.Default
	}
	if !slices.Contains(p.Models, *model) {
		fail(fmt.Errorf("unknown model %q for %s (want one of: %s)", *model, p.Name, strings.Join(p.Models, ", ")))
	}

	if _, err := os.Stat(defaultRulesDir); err != nil {
		fail(err)
	}
	if err := run(os.DirFS(defaultRulesDir), flag.Args(), p.New(os.Getenv(p.EnvVar), *model)); err != nil {
		fail(err)
	}
}

func run(rulesFS fs.FS, paths []string, model eval.Model) error {
	loaded, err := rules.Load(rulesFS)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		// No operands reads standard input, as gofmt does.
		fmt.Println("<standard input>")
		return nil
	}

	files, err := resolveInputs(paths, func(name string) bool {
		for _, r := range loaded {
			if r.Applies(name) {
				return true
			}
		}
		return false
	})
	if err != nil {
		return err
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	for _, f := range files {
		findings, err := eval.File(f, loaded, model)
		if err != nil {
			return err
		}
		for _, finding := range findings {
			fmt.Printf("%s: %s\n", f.Name(), finding)
		}
	}
	return nil
}

// resolveInputs expands path operands into open files, mirroring gofmt's
// operand handling. An explicit file operand is always included; a directory
// operand is walked and each file within it is included only when match reports
// true for its name.
func resolveInputs(paths []string, match func(name string) bool) ([]*os.File, error) {
	var files []*os.File
	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if path != p && match != nil && !match(d.Name()) {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			files = append(files, f)
			return nil
		})
		if err != nil {
			for _, f := range files {
				f.Close()
			}
			return nil, err
		}
	}
	return files, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gallant:", err)
	os.Exit(1)
}
