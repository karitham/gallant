// Package eval evaluates input files against the rules that apply to them.
package eval

import (
	"os"
	"path/filepath"

	"github.com/madxmike/gallant/rules"
)

// Finding is one rule's result for a file. For a noul, Confidence is the
// probability the rule is violated; for a choice or score, Result is the chosen
// option or level and Confidence is the model's confidence in it.
type Finding struct {
	Rule       string
	Type       rules.Type
	Confidence float64
	Result     string
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
