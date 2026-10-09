// Command gallant is the CLI entry point for gallant.
package main

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/madxmike/gallant/rules"
)

// defaultRulesDir is where gallant looks for lint rules, relative to the
// current working directory.
const defaultRulesDir = "agents/gallant"

func main() {
	fsys, err := mountRules(defaultRulesDir)
	if err != nil {
		fail(err)
	}
	if err := run(fsys, os.Args[1:]); err != nil {
		fail(err)
	}
}

func mountRules(dir string) (fs.FS, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	return os.DirFS(dir), nil
}

func run(fsys fs.FS, args []string) error {
	loaded, err := rules.Load(fsys)
	if err != nil {
		return err
	}
	fmt.Printf("loaded %d rules\n", len(loaded))
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gallant:", err)
	os.Exit(1)
}
