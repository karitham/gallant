// Package opencode provides an eval.Model backed by OpenCode Zen's System One
// gateway, which serves TypeSafe's Jev.
package opencode

import (
	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/eval/systemone"
)

// baseURL is OpenCode Zen; the System One client appends its own path.
const baseURL = "https://opencode.ai/zen"

// Model identifies a model available on OpenCode Zen.
type Model string

const (
	Jev113     Model = "jev-1.13"
	Jev113Free Model = "jev-1.13-free"
)

// New returns an eval.Model that evaluates with model, authenticating with the
// given OpenCode Zen API key.
func New(key string, model Model) eval.Model {
	return systemone.New(baseURL, string(model), key)
}

func init() {
	eval.Register(eval.Provider{
		Name:      "opencode",
		KeyEnvVar: "OPENCODE_API_KEY",
		Models:    []string{string(Jev113), string(Jev113Free)},
		Default:   string(Jev113Free),
		New: func(apiKey, _ string, model string) eval.Model {
			return New(apiKey, Model(model))
		},
	})
}
