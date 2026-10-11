// Package opencode provides an eval.Model backed by OpenCode Zen's System One
// gateway, which serves TypeSafe's Jev.
package opencode

import (
	"fmt"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/eval/systemone"
)

// baseURL is OpenCode Zen; the System One client appends its own path.
const baseURL = "https://opencode.ai/zen"

// keyEnvVar names the environment variable that holds the OpenCode Zen API key.
const keyEnvVar = "OPENCODE_API_KEY"

// Model identifies a model available on OpenCode Zen.
type Model string

const (
	Jev113     Model = "jev-1.13"
	Jev113Free Model = "jev-1.13-free"
)

// New returns an eval.Model that evaluates with model, authenticating with the
// given OpenCode Zen API key. The free model needs no key; any other model
// requires one, so a missing key is reported here rather than as a stream of
// rejected requests.
func New(key string, model Model) (eval.Model, error) {
	if key == "" && model != Jev113Free {
		return nil, fmt.Errorf("opencode: %s requires %s, which is not set", model, keyEnvVar)
	}
	return systemone.New(baseURL, string(model), key), nil
}

func init() {
	eval.Register(eval.Provider{
		Name:      "opencode",
		KeyEnvVar: keyEnvVar,
		Models:    []string{string(Jev113), string(Jev113Free)},
		Default:   string(Jev113Free),
		New: func(apiKey, _ string, model string) (eval.Model, error) {
			return New(apiKey, Model(model))
		},
	})
}
