// Package eval evaluates input files against the rules that apply to them.
package eval

import (
	"os"
	"path/filepath"

	"github.com/madxmike/gallant/rules"
)

// Finding is one rule's result for a file. Confidence is the model's
// probability that the rule is violated, from 0 to 1.
type Finding struct {
	Rule       string
	Confidence float64
}

// Model evaluates a file against the rules that apply to it, returning its
// findings.
type Model interface {
	Evaluate(file *os.File, rules []rules.Rule) ([]Finding, error)
}

// File returns the findings from evaluating file against the rules that apply
// to it.
func File(file *os.File, all []rules.Rule, model Model) ([]Finding, error) {
	var applicable []rules.Rule
	for _, r := range all {
		if r.Applies(filepath.Base(file.Name())) {
			applicable = append(applicable, r)
		}
	}
	return model.Evaluate(file, applicable)
}
