// Command gallant is the CLI entry point for gallant.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/madxmike/gallant/eval"
	_ "github.com/madxmike/gallant/eval/opencode"
	"github.com/madxmike/gallant/rules"
)

// defaultRulesDir is where gallant looks for lint rules, relative to the
// current working directory.
const defaultRulesDir = ".agents/gallant"

func main() {
	provider := flag.String("provider", "opencode", "model provider ("+strings.Join(eval.Names(), ", ")+")")
	model := flag.String("model", "", "model name")
	rulesDir := flag.String("rules", defaultRulesDir, "directory of lint rules, relative to the current working directory")
	includePassing := flag.Bool("include-passing", false, "also show rules the file passes")
	asJSON := flag.Bool("json", false, "emit findings as JSON")
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

	if _, err := os.Stat(*rulesDir); err != nil {
		fail(err)
	}
	if err := run(os.DirFS(*rulesDir), flag.Args(), p.New(os.Getenv(p.EnvVar), *model), *includePassing, *asJSON); err != nil {
		fail(err)
	}
}

func run(rulesFS fs.FS, paths []string, model eval.Model, includePassing, asJSON bool) error {
	loaded, err := rules.Load(rulesFS)
	if err != nil {
		return err
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

	records := []record{}
	for _, f := range files {
		findings, err := eval.File(f, loaded, model)
		if err != nil {
			return err
		}
		for _, finding := range findings {
			result, confidence := finding.Result, finding.Confidence
			if finding.Type == rules.TypeNoul {
				if finding.Confidence <= 0.5 {
					if !includePassing {
						continue
					}
					result, confidence = "pass", 1-finding.Confidence
				} else {
					result, confidence = "fail", finding.Confidence
				}
			}
			records = append(records, record{
				File:       f.Name(),
				Rule:       finding.Rule,
				Type:       string(finding.Type),
				Result:     result,
				Confidence: confidence,
			})
		}
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}
	for _, r := range records {
		fmt.Printf("%s %s %s %.0f%% %s\n", escape(r.File), escape(r.Rule), r.Type, r.Confidence*100, escape(r.Result))
	}
	return nil
}

// record is one finding as it is emitted. Confidence is the confidence in
// Result: for a noul it is the probability of the reported pass or fail, and for
// a choice or score it is the model's confidence in the chosen option or level.
type record struct {
	File       string  `json:"file"`
	Rule       string  `json:"rule"`
	Type       string  `json:"type"`
	Result     string  `json:"result"`
	Confidence float64 `json:"confidence"`
}

// escape makes s safe to print on one line. It renders s as a quoted Go string
// with the surrounding quotes removed, so backslashes, newlines, tabs, and other
// control characters are escaped and the field never spans a line.
func escape(s string) string {
	quoted := strconv.Quote(s)
	return quoted[1 : len(quoted)-1]
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
