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
	_ "github.com/madxmike/gallant/eval/cloudflare"
	_ "github.com/madxmike/gallant/eval/ollama"
	_ "github.com/madxmike/gallant/eval/opencode"
	_ "github.com/madxmike/gallant/eval/strandsdecider"
	"github.com/madxmike/gallant/rules"
)

// defaultRulesDir is where gallant looks for lint rules, relative to the
// current working directory.
const defaultRulesDir = ".agents/gallant"

// defaultJobs is the default number of files evaluated concurrently. Evaluation
// is dominated by network round trips, so this exceeds the CPU count.
const defaultJobs = 8

// defaultProvider is the provider used when neither --provider nor
// GALLANT_PROVIDER names one.
const defaultProvider = "opencode"

// Environment variables that supply defaults a flag can override.
const (
	envProvider = "GALLANT_PROVIDER"
	envModel    = "GALLANT_MODEL"
	envRules    = "GALLANT_RULES"
)

func main() {
	s := streams{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	args := os.Args[1:]
	if printDocRequest(args, s.stdout) {
		return
	}

	cfg, err := resolveConfig(args, os.Getenv)
	if err != nil {
		exitWith(err, s.stderr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	m, err := newModel(ctx, cfg.model, os.Getenv, s.stderr)
	if err != nil {
		exitWith(err, s.stderr)
	}

	if err := checkRulesDir(cfg.rulesDir); err != nil {
		exitWith(err, s.stderr)
	}

	if err := run(ctx, os.DirFS(cfg.rulesDir), cfg.run, m, s); err != nil {
		exitWith(err, s.stderr)
	}
}

// config is one invocation after flags, environment defaults, and built-in
// defaults are resolved. Its model names a registered provider and a model
// scoped to it.
type config struct {
	model    modelSpec
	rulesDir string
	run      runOptions
}

// modelSpec is the model a run uses: a registered provider and a model name
// scoped to it. resolveModel is its only constructor.
type modelSpec struct {
	provider eval.Provider
	name     string
}

// runOptions are the settings a lint pass consumes: how to obtain its inputs and
// how to report them. They travel together and carry no provider or
// configuration knowledge.
type runOptions struct {
	read inputReader
	// jobs is at least 1; resolveConfig rejects smaller values.
	jobs           int
	includePassing bool
	asJSON         bool
}

// streams are the process streams a run reads and writes. They are injected
// rather than read from the process so run has no global state.
type streams struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// flagError is a command line the flag parser rejected. main prints the usage
// text with it, unlike invalidCommand, which the parser accepted but the
// command cannot act on.
type flagError struct{ err error }

func (e flagError) Error() string { return e.err.Error() }
func (e flagError) Unwrap() error { return e.err }

// invalidCommand is a parsed command line gallant cannot act on: an unknown
// provider or model, conflicting input modes, or a jobs count below one. main
// reports it without the usage text.
type invalidCommand struct{ err error }

func (e invalidCommand) Error() string { return e.err.Error() }
func (e invalidCommand) Unwrap() error { return e.err }

// disposition is how main reports an error: the exit status, the line for
// standard error (empty for none), and whether to print the usage text.
type disposition struct {
	code    int
	message string
	usage   bool
}

// resolveConfig parses args and gives each setting the highest-precedence value
// available: an explicit flag, then its environment default, then the built-in
// default. getenv is injected so the precedence is testable without mutating the
// process environment. Every rejection is tagged with the error kind main uses
// to choose its exit status.
func resolveConfig(args []string, getenv func(string) string) (config, error) {
	flags := flag.NewFlagSet("gallant", flag.ContinueOnError)
	// main owns the diagnostics and the exit status, so the parser writes
	// neither and only reports the error.
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}

	// Each flag declares the environment value as its default, so the flag
	// package's own precedence carries the rule: an explicit flag, then the
	// environment, then the built-in default. An empty environment value falls
	// through to the built-in default.
	provider := flags.String("provider", envOr(getenv, envProvider, defaultProvider), "model provider ("+strings.Join(eval.Names(), ", ")+")")
	model := flags.String("model", envOr(getenv, envModel, ""), "model name")
	rulesDir := flags.String("rules", envOr(getenv, envRules, defaultRulesDir), "directory of lint rules, relative to the current working directory")
	jobs := flags.Int("jobs", defaultJobs, "maximum number of files to evaluate concurrently")
	includePassing := flags.Bool("include-passing", false, "also show rules the file passes")
	asJSON := flags.Bool("json", false, "emit findings as JSON")
	diff := flags.Bool("diff", false, "lint a unified diff read from stdin")
	assumeFilename := flags.String("assume-filename", "", "lint a single file read from stdin under this name")
	if err := flags.Parse(args); err != nil {
		return config{}, flagError{err}
	}

	if *jobs < 1 {
		return config{}, invalidCommand{fmt.Errorf("--jobs must be at least 1, got %d", *jobs)}
	}

	read, err := parseInputSource(flags.Args(), *diff, *assumeFilename)
	if err != nil {
		if errors.Is(err, errNoInput) {
			// The bare invocation prints usage alone; wrapping it in
			// invalidCommand would add a message.
			return config{}, err
		}
		return config{}, invalidCommand{err}
	}

	p, ok := eval.Lookup(*provider)
	if !ok {
		return config{}, invalidCommand{fmt.Errorf("unknown provider %q (want one of: %s)", *provider, strings.Join(eval.Names(), ", "))}
	}
	spec, err := resolveModel(p, *model)
	if err != nil {
		return config{}, err
	}

	cfg := config{
		model:    spec,
		rulesDir: *rulesDir,
		run: runOptions{
			read:           read,
			jobs:           *jobs,
			includePassing: *includePassing,
			asJSON:         *asJSON,
		},
	}
	return cfg, nil
}

// resolveModel applies the provider's default when model names none and checks
// the result against the set the provider declares. A provider that declares no
// models discovers them at construction, so its names are checked by newModel.
func resolveModel(p eval.Provider, model string) (modelSpec, error) {
	if model == "" {
		model = p.Default
	}
	if model == "" {
		return modelSpec{}, invalidCommand{fmt.Errorf("provider %s declares no default model; pass --model", p.Name)}
	}
	if len(p.Models) > 0 && !slices.Contains(p.Models, model) {
		return modelSpec{}, unknownModel(p.Name, model, p.Models)
	}
	return modelSpec{provider: p, name: model}, nil
}

// unknownModel is the diagnostic for a model name absent from the set the
// provider declares or discovers.
func unknownModel(provider, model string, known []string) error {
	return invalidCommand{fmt.Errorf("unknown model %q for %s (want one of: %s)", model, provider, strings.Join(known, ", "))}
}

// envOr returns the environment value named name, or def when the variable is
// unset or set to the empty string.
func envOr(getenv func(string) string, name, def string) string {
	if value := getenv(name); value != "" {
		return value
	}
	return def
}

// newModel constructs the selected model and, when the provider declares no
// models of its own, checks the name against the set the provider discovers.
// Discovery failure is not fatal: the name is passed through and the server
// reports per-file problems, with a warning to warn. Construction precedes
// discovery because the lister is the constructed model, and it also reports a
// missing API key first.
func newModel(ctx context.Context, spec modelSpec, getenv func(string) string, warn io.Writer) (eval.Model, error) {
	p := spec.provider
	m, err := p.New(getenv(p.KeyEnvVar), getenv(p.URLEnvVar), spec.name)
	if err != nil {
		return nil, err
	}

	if len(p.Models) == 0 {
		if lister, ok := m.(eval.Lister); ok {
			names, err := lister.List(ctx)
			switch {
			case err != nil:
				fmt.Fprintf(warn, "gallant: %s: cannot list models: %v; using model %q\n", p.Name, err, spec.name)
			case len(names) > 0 && !slices.Contains(names, spec.name):
				return nil, unknownModel(p.Name, spec.name, names)
			}
		}
	}
	return m, nil
}

// printDocRequest writes the documentation an argument asks for and reports
// whether it did. It runs before flag parsing because an explicit help request
// prints to standard output and exits 0, unlike the flag package's own help.
func printDocRequest(args []string, w io.Writer) bool {
	for i, arg := range args {
		switch arg {
		case "-h", "-help", "--help":
			var topic string
			if i+1 < len(args) {
				topic = args[i+1]
			}
			switch topic {
			case "rules":
				printRulesHelp(w)
			case "providers":
				printProvidersHelp(w)
			default:
				printUsage(w)
			}
			return true
		case "--man":
			printMan(w)
			return true
		}
	}
	return false
}

func run(ctx context.Context, rulesFS fs.FS, opts runOptions, model eval.Model, s streams) error {
	loaded, err := rules.Load(rulesFS)
	if err != nil {
		return err
	}
	if len(loaded) == 0 {
		fmt.Fprintln(s.stderr, "gallant: no rules found; nothing will be checked")
	}

	// minConfidence is indexed by rule path so a finding can recover the
	// threshold its rule declared.
	minConfidence := make(map[string]float64, len(loaded))
	for _, r := range loaded {
		minConfidence[r.Path] = r.MinConfidence
	}

	match := func(name string) bool {
		return len(applicable(loaded, name)) > 0
	}

	inputs, err := opts.read(match, s.stdin)
	if err != nil {
		return err
	}

	results, err := evaluateAll(ctx, inputs, loaded, model, opts.jobs)
	if err != nil {
		return err
	}

	records := []record{}
	var failed, violations int
	// A provider failure is the same for every file, so report each distinct
	// message once with a count rather than once per file.
	providerFaults := map[string]int{}
	var providerOrder []string
	for i, res := range results {
		if res.err != nil {
			failed++
			var pe *eval.ProviderError
			if errors.As(res.err, &pe) {
				msg := res.err.Error()
				if _, seen := providerFaults[msg]; !seen {
					providerOrder = append(providerOrder, msg)
				}
				providerFaults[msg]++
				continue
			}
			fmt.Fprintf(s.stderr, "gallant: %s: %v\n", inputs[i].name, res.err)
			continue
		}
		for _, finding := range res.findings {
			result, confidence := finding.Result, finding.Confidence
			if finding.Type == rules.TypeNoul {
				// A model can name a rule that was not loaded, so a missing
				// entry falls back to the shared default.
				min := rules.DefaultMinConfidence
				if declared, ok := minConfidence[finding.Rule]; ok {
					min = declared
				}
				if finding.Confidence <= min {
					if !opts.includePassing {
						continue
					}
					result, confidence = "pass", 1-finding.Confidence
				} else {
					result, confidence = "fail", finding.Confidence
					violations++
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
	for _, msg := range providerOrder {
		if n := providerFaults[msg]; n > 1 {
			fmt.Fprintf(s.stderr, "gallant: %s (%d files)\n", msg, n)
		} else {
			fmt.Fprintf(s.stderr, "gallant: %s\n", msg)
		}
	}

	if opts.asJSON {
		enc := json.NewEncoder(s.stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(records); err != nil {
			return err
		}
	} else {
		for _, r := range records {
			// The result is the only free-text field, so it is the only one that
			// can contain a newline or tab and need escaping.
			result := strconv.Quote(r.Result)
			fmt.Fprintf(s.stdout, "%s %s %s %.0f%% %s\n", r.File, r.Rule, r.Type, r.Confidence*100, result[1:len(result)-1])
		}
	}

	if failed > 0 {
		return fmt.Errorf("could not evaluate %d of %d files", failed, len(inputs))
	}
	if violations > 0 {
		return fmt.Errorf("%w: %d", errChecksFailed, violations)
	}
	return nil
}

// errChecksFailed reports that the run completed and at least one noul rule
// failed, so a script or CI job can gate on the exit status.
var errChecksFailed = errors.New("checks failed")

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

// result is one input's evaluation: its findings, or the error that stopped it.
// A failed input does not affect the others, so one unreadable or oversized file
// does not abandon the rest of the run.
type result struct {
	findings []eval.Finding
	err      error
}

// evaluateAll evaluates every input, up to jobs at a time, returning each
// input's result indexed by its position in inputs. jobs must be at least 1.
// Output is assembled in input order regardless of the order evaluations finish.
// ctx cancellation stops further dispatch and cancels in-flight work.
func evaluateAll(ctx context.Context, inputs []input, all []rules.Rule, model eval.Model, jobs int) ([]result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]result, len(inputs))
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup

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
			results[i] = result{findings: findings, err: err}
		}(i, in)
	}
	wg.Wait()

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
	matched := applicable(all, filepath.Base(in.name))
	if len(matched) == 0 {
		// Nothing to check, so do not open the file or bother the model. An
		// explicit file operand is always included, even when no rule matches it.
		return nil, nil
	}
	r, err := in.open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return model.Evaluate(ctx, r, matched)
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
// match reports true for its name, and hidden files and directories are skipped
// during the walk. Files are opened lazily by evaluate, not here, so a large tree
// does not hold one descriptor per file.
func resolveInputs(paths []string, match func(name string) bool) ([]input, error) {
	var inputs []input
	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Skip hidden directories, as gofmt does, so a broad glob does
				// not descend into .git and the like.
				if path != p && strings.HasPrefix(d.Name(), ".") {
					return fs.SkipDir
				}
				return nil
			}
			// Hidden files are skipped during a walk but still honoured when named
			// explicitly, as with gofmt.
			if path != p && strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			if path != p && match != nil && !match(d.Name()) {
				return nil
			}
			inputs = append(inputs, fileInput(path))
			return nil
		})
		if err != nil {
			return nil, unwrapPath(err)
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
			return nil, errNoDiff
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

// errNoDiff reports stdin that carried no unified diff at all, as opposed to a
// diff in which no file matched a rule.
var errNoDiff = errors.New("no diff found on stdin (--diff expects unified diff output)")

// inputReader expands a parsed input source into the inputs to evaluate. It is
// the only thing run knows about the source, so a source that names no input or
// more than one cannot reach it.
type inputReader func(match func(string) bool, stdin io.Reader) ([]input, error)

// parseInputSource parses the input flags and operands into the one way this run
// reads its inputs. Every accepted combination returns a reader; a combination
// that names no source or more than one is rejected here, so no later stage has
// to check it.
func parseInputSource(args []string, diff bool, assumeFilename string) (inputReader, error) {
	switch {
	case diff && assumeFilename != "":
		return nil, errors.New("--diff and --assume-filename are mutually exclusive")
	case diff && len(args) > 0:
		return nil, errors.New("--diff reads a diff from stdin and takes no path operands")
	case assumeFilename != "" && len(args) > 0:
		return nil, errors.New("--assume-filename reads from stdin and takes no path operands")
	case !diff && assumeFilename == "" && len(args) == 0:
		return nil, errNoInput
	case diff:
		return diffReader(), nil
	case assumeFilename != "":
		return namedReader(assumeFilename), nil
	default:
		return pathsReader(args), nil
	}
}

// pathsReader reads the files and directories named on the command line.
func pathsReader(paths []string) inputReader {
	return func(match func(string) bool, _ io.Reader) ([]input, error) {
		return resolveInputs(paths, match)
	}
}

// diffReader reads a unified diff from stdin.
func diffReader() inputReader {
	return func(match func(string) bool, stdin io.Reader) ([]input, error) {
		return diffInputs(stdin, match)
	}
}

// namedReader reads one file from stdin under name.
func namedReader(name string) inputReader {
	return func(_ func(string) bool, stdin io.Reader) ([]input, error) {
		return namedInput(name, stdin)
	}
}

// checkRulesDir reports a missing, unreadable, or non-directory rules path in
// terms a user can act on, rather than the raw stat error.
func checkRulesDir(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("rules directory %q is not a directory", dir)
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("rules directory %q does not exist (create it, or pass --rules)", dir)
	default:
		return unwrapPath(err)
	}
}

// unwrapPath renders a PathError without the syscall name it carries, so a
// diagnostic reads "<path>: no such file or directory" rather than the
// internal "lstat <path>: no such file or directory".
func unwrapPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %w", pe.Path, pe.Err)
	}
	return err
}

// exitWith reports err on w and exits with its disposition. Diagnostics never
// go to stdout, so a consumer reading findings from a pipe sees only findings.
func exitWith(err error, w io.Writer) {
	d := dispositionOf(err)
	if d.message != "" {
		fmt.Fprintln(w, d.message)
	}
	if d.usage {
		printUsage(w)
	}
	os.Exit(d.code)
}

// dispositionOf maps an error to how main reports it. The error kind is assigned
// where the error is found, so this table does not need to know which stage
// failed: a command line gallant cannot act on exits 2, a failed run exits 1,
// and an interrupt keeps the shell's conventional 130.
func dispositionOf(err error) disposition {
	if errors.Is(err, errNoInput) {
		return disposition{code: 2, usage: true}
	}
	var flagErr flagError
	if errors.As(err, &flagErr) {
		return disposition{code: 2, message: "gallant: " + err.Error(), usage: true}
	}
	var invalid invalidCommand
	if errors.As(err, &invalid) {
		return disposition{code: 2, message: "gallant: " + err.Error()}
	}
	if errors.Is(err, context.Canceled) {
		return disposition{code: 130, message: "gallant: interrupted"}
	}
	return disposition{code: 1, message: "gallant: " + err.Error()}
}
