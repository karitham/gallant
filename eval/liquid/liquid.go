// Package liquid provides an eval.Model backed by Liquid AI's d1 decision
// models.
//
// The hosted Liquid API serves d1.1 and d1-ultra at
// https://api.liquid.ai/decisions/v1/systemone. A llama-server with the open
// LFM3 decision weights (d1-2B and d1-Omni-2B, llama.cpp b11960 or later)
// serves the same protocol at /v1/systemone, so LIQUID_BASE_URL can point this
// provider at a local server.
package liquid

import (
	"cmp"
	"fmt"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/eval/systemone"
)

const (
	// defaultBaseURL is the hosted API root; systemone.HTTP appends the
	// System One path.
	defaultBaseURL = "https://api.liquid.ai/decisions"
	keyEnvVar      = "LIQUID_API_KEY"
	defaultModel   = "d1.1"
)

var models = []string{defaultModel, "d1-ultra", "d1-2B", "d1-Omni-2B"}

// New returns an eval.Model that evaluates with model. An empty host selects
// the hosted Liquid API, which requires key. A host selects a llama-server
// serving the same protocol locally, where key may be empty.
func New(key, host, model string) (eval.Model, error) {
	if host == "" && key == "" {
		return nil, fmt.Errorf("liquid: %s is not set", keyEnvVar)
	}
	return systemone.New(systemone.HTTP(cmp.Or(host, defaultBaseURL), key), model), nil
}

func init() {
	eval.Register(eval.Provider{
		Name:      "liquid",
		KeyEnvVar: keyEnvVar,
		URLEnvVar: "LIQUID_BASE_URL",
		Models:    models,
		Default:   defaultModel,
		New:       New,
	})
}
