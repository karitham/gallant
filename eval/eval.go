// Package eval evaluates input streams against the rules that apply to them.
package eval

import (
	"context"
	"io"

	"github.com/madxmike/gallant/rules"
)

// Outcome is one possible result for a rule and the probability the model gives
// it. Confidence means the same thing for every rule type: the probability that
// this result is the answer.
type Outcome struct {
	Result     string
	Confidence float64
}

// Finding is one rule's answer for an input: every result the model weighed,
// most likely first. A noul has two outcomes (fail and pass), a choice one per
// option, and a score one per level.
type Finding struct {
	Rule     string
	Type     rules.Type
	Outcomes []Outcome
}

// Model evaluates an input stream against the rules it is given, returning its
// findings. Evaluate must abort and return when ctx is done.
type Model interface {
	Evaluate(ctx context.Context, r io.Reader, rules []rules.Rule) ([]Finding, error)
}

// Lister is implemented by models that can report the model names they offer,
// so a provider's set can be discovered at startup rather than declared.
type Lister interface {
	List(ctx context.Context) ([]string, error)
}

// ProviderError reports a failure of the model provider itself — an unreachable
// server, a rejected request, or a malformed response — as opposed to a problem
// with one input. Because it is the same for every file, a run reports it once
// rather than repeating it per file.
type ProviderError struct {
	Err error
}

func (e *ProviderError) Error() string { return e.Err.Error() }

func (e *ProviderError) Unwrap() error { return e.Err }
