// Command gallant is the CLI entry point for gallant.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	diff := flag.Bool("diff", false, "lint a unified diff read from stdin")
	assumeFilename := flag.String("assume-filename", "", "lint a single file read from stdin under this name")
	flag.Parse()

	if err := validateInputs(*diff, *assumeFilename, flag.Args()); err != nil {
		if errors.Is(err, errNoInput) {
			flag.Usage()
			os.Exit(2)
		}
		fail(err)
	}

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

	if err := run(ctx, os.DirFS(*rulesDir), flag.Args(), *diff, *assumeFilename, p.New(os.Getenv(p.EnvVar), *model), *jobs, *includePassing, *asJSON); err != nil {
		fail(err)
	}
}

func run(ctx context.Context, rulesFS fs.FS, paths []string, diff bool, assumeFilename string, model eval.Model, jobs int, includePassing, asJSON bool) error {
	loaded, err := rules.Load(rulesFS)
	if err != nil {
		return err
	}

	match := func(name string) bool {
		return len(applicable(loaded, name)) > 0
	}

	var inputs []input
	switch {
	case diff:
		inputs, err = diffInputs(os.Stdin, match)
	case assumeFilename != "":
		inputs, err = namedInput(assumeFilename, os.Stdin)
	default:
		inputs, err = resolveInputs(paths, match)
	}
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
				File:       inputs[i].name,
				Rule:       finding.Rule,
				Type:       finding.Type,
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
	File       string     `json:"file"`
	Rule       string     `json:"rule"`
	Type       rules.Type `json:"type"`
	Result     string     `json:"result"`
	Confidence float64    `json:"confidence"`
}

// evaluateAll evaluates every input, up to jobs at a time, returning each
// input's findings indexed by its position in inputs. Output is assembled in
// input order regardless of the order evaluations finish. The first error, or
// ctx cancellation, stops further dispatch and cancels in-flight work.
func evaluateAll(ctx context.Context, inputs []input, all []rules.Rule, model eval.Model, jobs int) ([][]eval.Finding, error) {
	if jobs < 1 {
		jobs = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]eval.Finding, len(inputs))
	sem := make(chan struct{}, jobs)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

dispatch:
	for i, in := range inputs {
		select {
		case <-ctx.Done():
			break dispatch
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(i int, in input) {
			defer wg.Done()
			defer func() { <-sem }()
			findings, err := evaluate(ctx, in, all, model)
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
		}(i, in)
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

// evaluate opens in and hands the model a reader for its contents along with the
// rules whose globs match its name, then closes it. Content is opened only for
// the duration of the call, so the model never sees a file and a large tree
// never holds one descriptor per file.
func evaluate(ctx context.Context, in input, all []rules.Rule, model eval.Model) ([]eval.Finding, error) {
	r, err := in.open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return model.Evaluate(ctx, r, applicable(all, filepath.Base(in.name)))
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

// input is one unit of evaluation: the name used to match rules and to report
// the finding, and a function that opens its content. Opening is deferred until
// evaluate runs, so a large tree never holds one descriptor per file.
type input struct {
	name string
	open func() (io.ReadCloser, error)
}

// fileInput is an input whose content is read from path on disk.
func fileInput(path string) input {
	return input{
		name: path,
		open: func() (io.ReadCloser, error) { return os.Open(path) },
	}
}

// textInput is an input whose content is already in memory, as for a file's
// slice of a unified diff.
func textInput(name, content string) input {
	return input{
		name: name,
		open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(content)), nil },
	}
}

// resolveInputs expands path operands into matching file inputs, mirroring
// gofmt's operand handling. An explicit file operand is always included; a
// directory operand is walked and each file within it is included only when
// match reports true for its name. Files are opened lazily by evaluate, not
// here, so a large tree does not hold one descriptor per file.
func resolveInputs(paths []string, match func(name string) bool) ([]input, error) {
	var inputs []input
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
			inputs = append(inputs, fileInput(path))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return inputs, nil
}

// diffInputs reads a unified diff and returns one input per changed file whose
// name matches, so the model reviews only the change rather than whole files.
// A section starts at a line-initial "diff --git"; the name is the destination
// path with git's b/ prefix stripped, and the content is the file's whole
// section, preserved verbatim so the model sees the file's name with its hunks.
// Splitting only on line-initial markers keeps a hunk that adds a line
// containing "diff --git" from starting a phantom file.
func diffInputs(r io.Reader, match func(name string) bool) ([]input, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if !strings.HasPrefix(text, "diff --git ") {
		// Skip anything before the first section, such as the commit header of
		// git show output.
		i := strings.Index(text, "\ndiff --git ")
		if i < 0 {
			return nil, nil
		}
		text = text[i+1:]
	}
	text = strings.TrimPrefix(text, "diff --git ")

	var inputs []input
	for _, section := range strings.Split(text, "\ndiff --git ") {
		header, _, _ := strings.Cut(section, "\n")
		if !strings.HasPrefix(header, "a/") {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "b/")
		if name == "" || (match != nil && !match(filepath.Base(name))) {
			continue
		}
		inputs = append(inputs, textInput(name, "diff --git "+section))
	}
	return inputs, nil
}

// namedInput reads one input's content from r and names it name, so a stream
// that is not a diff can still be linted as a named file.
func namedInput(name string, r io.Reader) ([]input, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return []input{textInput(name, string(data))}, nil
}

// errNoInput reports a bare invocation: no path operands and no stdin mode.
var errNoInput = errors.New("no input")

// validateInputs rejects flag and operand combinations that name no input or
// more than one source of input.
func validateInputs(diff bool, assumeFilename string, args []string) error {
	switch {
	case diff && assumeFilename != "":
		return errors.New("--diff and --assume-filename are mutually exclusive")
	case diff && len(args) > 0:
		return errors.New("--diff reads a diff from stdin and takes no path operands")
	case assumeFilename != "" && len(args) > 0:
		return errors.New("--assume-filename reads from stdin and takes no path operands")
	case !diff && assumeFilename == "" && len(args) == 0:
		return errNoInput
	}
	return nil
}

func fail(err error) {
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "gallant: interrupted")
		os.Exit(130)
	}
	fmt.Fprintln(os.Stderr, "gallant:", err)
	os.Exit(1)
}
