// Command gallant is the CLI entry point for gallant.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/madxmike/gallant/eval"
	_ "github.com/madxmike/gallant/eval/opencode"
	"github.com/madxmike/gallant/rules"
)

// defaultRulesDir is where gallant looks for lint rules, relative to the
// current working directory.
const defaultRulesDir = ".agents/gallant"

// defaultJobs is the default number of files evaluated concurrently. Evaluation
// is dominated by network round trips, so this exceeds the CPU count.
const defaultJobs = 8

func main() {
	provider := flag.String("provider", "opencode", "model provider ("+strings.Join(eval.Names(), ", ")+")")
	model := flag.String("model", "", "model name")
	rulesDir := flag.String("rules", defaultRulesDir, "directory of lint rules, relative to the current working directory")
	jobs := flag.Int("jobs", defaultJobs, "maximum number of files to evaluate concurrently")
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.DirFS(*rulesDir), flag.Args(), p.New(os.Getenv(p.EnvVar), *model), *jobs, *includePassing, *asJSON); err != nil {
		fail(err)
	}
}

func run(ctx context.Context, rulesFS fs.FS, paths []string, model eval.Model, jobs int, includePassing, asJSON bool) error {
	loaded, err := rules.Load(rulesFS)
	if err != nil {
		return err
	}

	inputs, err := resolveInputs(paths, func(name string) bool {
		return len(applicable(loaded, name)) > 0
	})
	if err != nil {
		return err
	}

	results, err := evaluateAll(ctx, inputs, loaded, model, jobs)
	if err != nil {
		return err
	}

	records := []record{}
	for i, findings := range results {
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
				File:       inputs[i],
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
		// The result is the only free-text field, so it is the only one that
		// can contain a newline or tab and need escaping.
		result := strconv.Quote(r.Result)
		fmt.Printf("%s %s %s %.0f%% %s\n", r.File, r.Rule, r.Type, r.Confidence*100, result[1:len(result)-1])
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

// evaluateAll evaluates every input, up to jobs at a time, returning each
// input's findings indexed by its position in paths. Output is assembled in
// input order regardless of the order evaluations finish. The first error, or
// ctx cancellation, stops further dispatch and cancels in-flight work.
func evaluateAll(ctx context.Context, paths []string, all []rules.Rule, model eval.Model, jobs int) ([][]eval.Finding, error) {
	if jobs < 1 {
		jobs = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]eval.Finding, len(paths))
	sem := make(chan struct{}, jobs)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

dispatch:
	for i, path := range paths {
		select {
		case <-ctx.Done():
			break dispatch
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			defer func() { <-sem }()
			findings, err := evaluate(ctx, path, all, model)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				cancel()
				return
			}
			results[i] = findings
		}(i, path)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// evaluate opens path, hands the model a reader for its contents along with the
// rules whose globs match its name, and closes it. The descriptor lives only for
// the duration of the call, so the model never sees a file and a large tree never
// holds one descriptor per file.
func evaluate(ctx context.Context, path string, all []rules.Rule, model eval.Model) ([]eval.Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return model.Evaluate(ctx, f, applicable(all, filepath.Base(path)))
}

// applicable returns the rules whose globs match name.
func applicable(all []rules.Rule, name string) []rules.Rule {
	var matched []rules.Rule
	for _, r := range all {
		if r.Applies(name) {
			matched = append(matched, r)
		}
	}
	return matched
}

// resolveInputs expands path operands into matching file paths, mirroring
// gofmt's operand handling. An explicit file operand is always included; a
// directory operand is walked and each file within it is included only when
// match reports true for its name. Files are opened lazily by evaluate, not
// here, so a large tree does not hold one descriptor per file.
func resolveInputs(paths []string, match func(name string) bool) ([]string, error) {
	var files []string
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
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func fail(err error) {
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "gallant: interrupted")
		os.Exit(130)
	}
	fmt.Fprintln(os.Stderr, "gallant:", err)
	os.Exit(1)
}
