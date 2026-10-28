package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
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

// A directory walk skips hidden files and directories, as gofmt does.
func TestResolveInputsSkipsHiddenEntries(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, ".hidden.go", "")
	write(t, dir, ".git/deep.go", "")

	paths, err := resolveInputs([]string{dir}, nil)
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}
	if len(paths) != 1 || filepath.Base(paths[0].name) != "a.go" {
		t.Errorf("resolveInputs() = %#v, want only a.go", paths)
	}
}

// An explicitly named hidden file is still checked.
func TestResolveInputsExplicitHiddenFileIncluded(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".hidden.go", "")

	paths, err := resolveInputs([]string{filepath.Join(dir, ".hidden.go")}, nil)
	if err != nil {
		t.Fatalf("resolveInputs() error = %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("resolveInputs() returned %d inputs, want the explicit hidden file", len(paths))
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

func TestEvaluateSkipsWhenNoRulesApplied(t *testing.T) {
	// An explicit file operand is always included, but when no rule applies to it
	// there is nothing to check, so the model is never called.
	model := &fakeModel{}
	if _, err := evaluate(context.Background(), textInput("notes.txt", "hi"), []rules.Rule{{AppliesTo: []string{"*.go"}}}, model); err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if model.content != "" || model.rules != nil {
		t.Errorf("model called with content %q, rules %#v; want no call when no rule applies", model.content, model.rules)
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
		if len(results[i].findings) != 1 || len(results[i].findings[0].Outcomes) != 1 || results[i].findings[0].Outcomes[0].Result != w {
			t.Errorf("results[%d] = %#v, want one finding with result %q", i, results[i], w)
		}
	}
}

// A failing input is reported per file, not as a failure of the whole run.
func TestEvaluateAllReportsPerFileError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, "b.go", "")
	paths := []input{fileInput(filepath.Join(dir, "a.go")), fileInput(filepath.Join(dir, "b.go"))}

	results, err := evaluateAll(context.Background(), paths, []rules.Rule{{AppliesTo: []string{"*.go"}}}, errorModel{}, len(paths))
	if err != nil {
		t.Fatalf("evaluateAll() error = %v, want nil: a per-file failure is not a run failure", err)
	}
	for i, res := range results {
		if res.err == nil {
			t.Errorf("results[%d].err = nil, want the model's error", i)
		}
	}
}

// exampleDiff changes two files; a.go changes in two places, so it must stay a
// single evaluation rather than one per hunk.
const exampleDiff = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1,2 +1,3 @@
 package a
+// first
 func f() {}
@@ -10,2 +11,3 @@
 var x = 1
+// second
 var y = 2
diff --git a/b.go b/b.go
index 3333333..4444444 100644
--- a/b.go
+++ b/b.go
@@ -1,2 +1,3 @@
 package b
+// third
 func g() {}
`

const noulRule = "---\nappliesTo: \"*.go\"\ntype: noul\n---\nno useless comments\n"

// noulRuleConfident is noulRule with a raised minConfidence, so a finding below
// 0.8 is not reported as a failure.
const noulRuleConfident = "---\nappliesTo: \"*.go\"\ntype: noul\nminConfidence: 0.8\n---\nno useless comments\n"

const choiceRule = "---\nappliesTo: \"*.go\"\ntype: choice\ncriteria:\n  compliant: \"ok\"\n  violation: \"bad\"\n---\njudge the comments\n"

// violationRule is choiceRule with a violation condition, so a run can be shown
// to report a verdict instead of leaving the probabilities to the reader.
const violationRule = "---\nappliesTo: \"*.go\"\ntype: choice\ncriteria:\n  useful: \"explains intent\"\n  useless: \"restates the code\"\nviolation:\n  if:\n    useless: 0.5\n---\njudge the comments\n"

func TestDiffInputsSplitsPerFile(t *testing.T) {
	inputs, err := diffInputs(strings.NewReader(exampleDiff), func(name string) bool {
		return strings.HasSuffix(name, ".go")
	})
	if err != nil {
		t.Fatalf("diffInputs() error = %v", err)
	}
	if len(inputs) != 2 {
		t.Fatalf("diffInputs() returned %d inputs, want 2", len(inputs))
	}
	if got, want := inputs[0].name, "a.go"; got != want {
		t.Errorf("inputs[0].name = %q, want %q", got, want)
	}
	if got, want := inputs[1].name, "b.go"; got != want {
		t.Errorf("inputs[1].name = %q, want %q", got, want)
	}

	body := readInput(t, inputs[0])
	if !strings.Contains(body, "+// first") || !strings.Contains(body, "+// second") {
		t.Errorf("a.go content = %q, want both hunks in one input", body)
	}
	if strings.Contains(body, "b.go") {
		t.Errorf("a.go content spilled into b.go: %q", body)
	}
}

func TestRunLintsDiffFromStdin(t *testing.T) {
	stdin(t, exampleDiff)
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRule)}}
	if err := run(context.Background(), rulesFS, nil, true, "", echoModel{}, 1, false, false); !errors.Is(err, errChecksFailed) {
		t.Fatalf("run() error = %v, want errChecksFailed", err)
	}

	got := out()
	for _, want := range []string{"a.go comments.md noul 90% fail", "b.go comments.md noul 90% fail"} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want %q", got, want)
		}
	}
	if n := strings.Count(got, "a.go comments.md noul 90% fail"); n != 1 {
		t.Errorf("output = %q, want exactly one fail result for a.go, got %d", got, n)
	}
}

// echoModel answers every rule with a noul finding, so a run reports one finding
// per (file, rule) and the output names each evaluated file.
type echoModel struct{}

func (echoModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	findings := make([]eval.Finding, len(rs))
	for i, rule := range rs {
		findings[i] = noulFinding(rule.Path, 0.9)
	}
	return findings, nil
}

// noulFinding answers a noul rule with the given probability of a violation.
func noulFinding(path string, violation float64) eval.Finding {
	return eval.Finding{Rule: path, Type: rules.TypeNoul, Outcomes: []eval.Outcome{
		{Result: "fail", Confidence: violation},
		{Result: "pass", Confidence: 1 - violation},
	}}
}

// confidentModel answers every noul rule with the same confidence, so a rule's
// minConfidence can be shown to decide whether the finding is reported.
type confidentModel struct{ confidence float64 }

func (m confidentModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	findings := make([]eval.Finding, len(rs))
	for i, rule := range rs {
		findings[i] = noulFinding(rule.Path, m.confidence)
	}
	return findings, nil
}

// pickyModel answers like echoModel but fails when an input contains bad, so a
// single failure can be shown to be reported per file without abandoning the
// rest of the run.
type pickyModel struct{ bad string }

func (m pickyModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(b), m.bad) {
		return nil, errors.New("input too large")
	}
	findings := make([]eval.Finding, len(rs))
	for i, rule := range rs {
		findings[i] = noulFinding(rule.Path, 0.9)
	}
	return findings, nil
}

// stdin points os.Stdin at content for the duration of the test.
func stdin(t *testing.T, content string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	orig := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = orig
		f.Close()
	})
}

// stdout redirects os.Stdout; the returned func yields what was written.
func stdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = orig
		r.Close()
	})
	return func() string {
		w.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
}

// stderr redirects os.Stderr; the returned func yields what was written.
func stderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = orig
		r.Close()
	})
	return func() string {
		w.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
}

// readInput opens in and returns its content.
func readInput(t *testing.T, in input) string {
	t.Helper()
	r, err := in.open()
	if err != nil {
		t.Fatalf("open() error = %v", err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read %q: %v", in.name, err)
	}
	return string(b)
}

func TestRunAssumesFilenameFromStdin(t *testing.T) {
	stdin(t, "package a\n// increment i\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", echoModel{}, 1, false, false); !errors.Is(err, errChecksFailed) {
		t.Fatalf("run() error = %v, want errChecksFailed", err)
	}
	if got := out(); !strings.Contains(got, "a.go comments.md noul 90% fail") {
		t.Errorf("output = %q, want a finding named a.go", got)
	}
}

func TestRunAssumeFilenameSelectsRules(t *testing.T) {
	stdin(t, "hello")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "notes.txt", echoModel{}, 1, false, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out(); got != "" {
		t.Errorf("output = %q, want none: the *.go rule does not match notes.txt", got)
	}
}

// choiceModel answers every choice rule with two results, so a run can be shown
// to report all of them rather than only the most likely.
type choiceModel struct{}

func (choiceModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	findings := make([]eval.Finding, len(rs))
	for i, rule := range rs {
		findings[i] = eval.Finding{Rule: rule.Path, Type: rules.TypeChoice, Outcomes: []eval.Outcome{
			{Result: "violation", Confidence: 0.75},
			{Result: "compliant", Confidence: 0.25},
		}}
	}
	return findings, nil
}

// A choice reports every option, not only the most likely one.
func TestRunReportsEveryChoiceOutcome(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(choiceRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", choiceModel{}, 1, false, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	got := out()
	for _, want := range []string{
		"a.go comments.md choice 75% violation",
		"a.go comments.md choice 25% compliant",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want %q", got, want)
		}
	}
}

// predicateModel answers every choice rule with a useless probability and its
// complement, so a violation condition can be shown to decide the verdict.
type predicateModel struct{ useless float64 }

func (m predicateModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	findings := make([]eval.Finding, len(rs))
	for i, rule := range rs {
		findings[i] = eval.Finding{Rule: rule.Path, Type: rules.TypeChoice, Outcomes: []eval.Outcome{
			{Result: "useless", Confidence: m.useless},
			{Result: "useful", Confidence: 1 - m.useless},
		}}
	}
	return findings, nil
}

func TestRunReportsViolationVerdict(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(violationRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", predicateModel{0.95}, 1, false, false); !errors.Is(err, errChecksFailed) {
		t.Fatalf("run() error = %v, want errChecksFailed", err)
	}
	got := out()
	for _, want := range []string{
		"a.go comments.md choice fail (useless 0.95 > 0.5)\n",
		"a.go comments.md choice 95% useless\n",
		"a.go comments.md choice 5% useful\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want %q", got, want)
		}
	}
	if strings.Index(got, "fail (") > strings.Index(got, "95% useless") {
		t.Errorf("output = %q, want the verdict before its results", got)
	}
}

func TestRunHidesPassingViolation(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(violationRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", predicateModel{0.2}, 1, false, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out(); got != "" {
		t.Errorf("output = %q, want none: 0.2 is below the 0.5 threshold", got)
	}
}

func TestRunShowsPassingViolationWhenAsked(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(violationRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", predicateModel{0.2}, 1, true, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	got := out()
	for _, want := range []string{
		"a.go comments.md choice pass (no if result above its threshold)\n",
		"a.go comments.md choice 20% useless\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want %q", got, want)
		}
	}
}

func TestRunJSONGroupsResultsByFinding(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(violationRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", predicateModel{0.95}, 1, false, true); !errors.Is(err, errChecksFailed) {
		t.Fatalf("run() error = %v, want errChecksFailed", err)
	}

	var got []record
	if err := json.Unmarshal([]byte(out()), &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("output holds %d objects, want 1: %#v", len(got), got)
	}
	r := got[0]
	if r.Verdict != "fail" || r.Reason != "useless 0.95 > 0.5" {
		t.Errorf("Verdict, Reason = %q, %q; want fail, %q", r.Verdict, r.Reason, "useless 0.95 > 0.5")
	}
	if len(r.Results) != 2 {
		t.Fatalf("Results = %#v, want two outcomes", r.Results)
	}
	if r.Results[0] != (eval.Outcome{Result: "useless", Confidence: 0.95}) {
		t.Errorf("Results[0] = %#v, want useless at 0.95", r.Results[0])
	}
}

func TestRunMinConfidenceSuppressesLowConfidence(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRuleConfident)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", confidentModel{0.7}, 1, false, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out(); got != "" {
		t.Errorf("output = %q, want none: 0.7 is below the 0.8 threshold", got)
	}
}

func TestRunMinConfidenceReportsAtThreshold(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRuleConfident)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", confidentModel{0.9}, 1, false, false); !errors.Is(err, errChecksFailed) {
		t.Fatalf("run() error = %v, want errChecksFailed", err)
	}
	got := out()
	for _, want := range []string{"a.go comments.md noul 90% fail", "a.go comments.md noul 10% pass"} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want %q", got, want)
		}
	}
}

func TestRunMinConfidenceShowsPassWhenAsked(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRuleConfident)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", confidentModel{0.7}, 1, true, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out(); !strings.Contains(got, "a.go comments.md noul 30% pass") {
		t.Errorf("output = %q, want a pass at 30%%: below the threshold the finding is a pass", got)
	}
}

func TestRunDefaultMinConfidenceReportsAboveMidpoint(t *testing.T) {
	stdin(t, "package a\n")
	out := stdout(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRule)}}
	if err := run(context.Background(), rulesFS, nil, false, "a.go", confidentModel{0.6}, 1, false, false); !errors.Is(err, errChecksFailed) {
		t.Fatalf("run() error = %v, want errChecksFailed", err)
	}
	if got := out(); !strings.Contains(got, "a.go comments.md noul 60% fail") {
		t.Errorf("output = %q, want a fail: 0.6 is above the default 0.5", got)
	}
}

// A file that cannot be evaluated is reported on stderr and does not stop the
// other files: their findings still print and the run still fails.
func TestRunReportsFailedFileAndContinues(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "good.go", "ok")
	write(t, dir, "bad.go", "oversized")

	out := stdout(t)
	errOut := stderr(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRule)}}
	err := run(context.Background(), rulesFS, []string{dir}, false, "", pickyModel{bad: "oversized"}, 1, false, false)
	if err == nil {
		t.Error("run() error = nil, want a failure when a file could not be evaluated")
	}

	got := out()
	if !strings.Contains(got, "good.go comments.md noul 90% fail") {
		t.Errorf("output = %q, want a finding for good.go", got)
	}
	if strings.Contains(got, "bad.go") {
		t.Errorf("output = %q, want no finding for the failed file", got)
	}

	want := "gallant: " + filepath.Join(dir, "bad.go") + ": input too large"
	if text := errOut(); !strings.Contains(text, want) {
		t.Errorf("stderr = %q, want %q", text, want)
	}
}

// A provider failure is the same for every file, so it is reported once with a
// count instead of once per file.
func TestRunReportsProviderFailureOnce(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")
	write(t, dir, "b.go", "")
	write(t, dir, "c.go", "")

	_ = stdout(t)
	errOut := stderr(t)

	rulesFS := fstest.MapFS{"comments.md": {Data: []byte(noulRule)}}
	err := run(context.Background(), rulesFS, []string{dir}, false, "", providerErrorModel{}, 2, false, false)
	if err == nil {
		t.Error("run() error = nil, want a failure when files could not be evaluated")
	}

	text := errOut()
	if n := strings.Count(text, "provider unreachable"); n != 1 {
		t.Errorf("stderr = %q, want the provider failure once, got %d", text, n)
	}
	if !strings.Contains(text, "(3 files)") {
		t.Errorf("stderr = %q, want a count of the files that failed", text)
	}
}

func TestCheckRulesDir(t *testing.T) {
	dir := t.TempDir()
	if err := checkRulesDir(dir); err != nil {
		t.Errorf("checkRulesDir(existing) error = %v, want nil", err)
	}
	err := checkRulesDir(filepath.Join(dir, "missing"))
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("checkRulesDir(missing) error = %v, want a message that it does not exist", err)
	}
}

// A path that exists but is a file is not a rules directory.
func TestCheckRulesDirRejectsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := checkRulesDir(file)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("checkRulesDir(file) error = %v, want a not-a-directory message", err)
	}
}

// An empty rules directory means nothing will be checked, so say so rather than
// exit silently as if everything passed.
func TestRunWarnsWhenNoRules(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "")

	_ = stdout(t)
	errOut := stderr(t)

	if err := run(context.Background(), fstest.MapFS{}, []string{dir}, false, "", echoModel{}, 1, false, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if text := errOut(); !strings.Contains(text, "no rules found") {
		t.Errorf("stderr = %q, want a warning that no rules were found", text)
	}
}

func TestUnwrapPathDropsSyscallName(t *testing.T) {
	_, err := os.Stat(filepath.Join(t.TempDir(), "missing"))
	got := unwrapPath(err).Error()
	if strings.Contains(got, "stat ") || strings.Contains(got, "lstat ") {
		t.Errorf("unwrapPath() = %q, want no syscall name", got)
	}
	if !strings.HasSuffix(got, "no such file or directory") {
		t.Errorf("unwrapPath() = %q, want the underlying cause preserved", got)
	}
}

func TestValidateInputs(t *testing.T) {
	tests := map[string]struct {
		diff           bool
		assumeFilename string
		args           []string
		wantErr        bool
	}{
		"paths only":           {args: []string{"."}},
		"diff":                 {diff: true},
		"assume filename":      {assumeFilename: "a.go"},
		"diff and assume":      {diff: true, assumeFilename: "a.go", wantErr: true},
		"diff with operands":   {diff: true, args: []string{"."}, wantErr: true},
		"assume with operands": {assumeFilename: "a.go", args: []string{"."}, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if err := validateInputs(tt.diff, tt.assumeFilename, tt.args); (err != nil) != tt.wantErr {
				t.Errorf("validateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateInputsRejectsBareInvocation(t *testing.T) {
	if err := validateInputs(false, "", nil); !errors.Is(err, errNoInput) {
		t.Errorf("validateInputs() error = %v, want errNoInput", err)
	}
}

func TestDiffInputsIgnoresMarkerInsideHunk(t *testing.T) {
	const diff = "diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n" +
		"+++ b/a.go\n" +
		"@@ -1 +1,2 @@\n" +
		" package a\n" +
		"+diff --git a/b.go b/b.go\n"

	inputs, err := diffInputs(strings.NewReader(diff), func(string) bool { return true })
	if err != nil {
		t.Fatalf("diffInputs() error = %v", err)
	}
	if len(inputs) != 1 || inputs[0].name != "a.go" {
		t.Fatalf("diffInputs() = %#v, want only a.go", inputs)
	}
	if body := readInput(t, inputs[0]); !strings.Contains(body, "+diff --git a/b.go b/b.go") {
		t.Errorf("content = %q, want the added marker line kept in a.go", body)
	}
}

func TestDiffInputsSkipsLeadingHeader(t *testing.T) {
	const show = "commit 0123456789abcdef\n" +
		"Author: A\n" +
		"\n" +
		"    a message\n" +
		"\n" +
		"diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n" +
		"+++ b/a.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n"

	inputs, err := diffInputs(strings.NewReader(show), func(string) bool { return true })
	if err != nil {
		t.Fatalf("diffInputs() error = %v", err)
	}
	if len(inputs) != 1 || inputs[0].name != "a.go" {
		t.Fatalf("diffInputs() = %#v, want only a.go", inputs)
	}
}

func TestDiffInputsNonDiff(t *testing.T) {
	inputs, err := diffInputs(strings.NewReader("just some text\n"), func(string) bool { return true })
	if !errors.Is(err, errNoDiff) {
		t.Fatalf("diffInputs() error = %v, want errNoDiff", err)
	}
	if len(inputs) != 0 {
		t.Errorf("diffInputs() = %#v, want none for non-diff input", inputs)
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
	return []eval.Finding{{Outcomes: []eval.Outcome{{Result: string(b)}}}}, nil
}

// errorModel always fails.
type errorModel struct{}

func (errorModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	return nil, errors.New("evaluate failed")
}

// providerErrorModel fails every input with a provider-level error, as an
// unreachable or erroring server does.
type providerErrorModel struct{}

func (providerErrorModel) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	return nil, &eval.ProviderError{Err: errors.New("provider unreachable")}
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
