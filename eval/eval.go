// Package eval evaluates input streams against the rules that apply to them.
package eval

import (
	"context"
	"io"

	"github.com/madxmike/gallant/rules"
)

// Finding is one rule's result for an input. For a noul, Confidence is the
// probability the rule is violated; for a choice or score, Result is the chosen
// option or level and Confidence is the model's confidence in it.
type Finding struct {
	Rule       string
	Type       rules.Type
	Confidence float64
	Result     string
}

// Model evaluates an input stream against the rules it is given, returning its
// findings. Evaluate must abort and return when ctx is done.
type Model interface {
	Evaluate(ctx context.Context, r io.Reader, rules []rules.Rule) ([]Finding, error)
}
