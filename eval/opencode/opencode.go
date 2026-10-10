// Package opencode provides an eval.Model backed by OpenCode's Zen gateway.
package opencode

import (
	"errors"
	"net/http"
	"os"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

const systemOneURL = "https://opencode.ai/zen/v1/systemone"

// Model identifies a model available on OpenCode Zen.
type Model string

const (
	Jev113     Model = "jev-1.13"
	Jev113Free Model = "jev-1.13-free"
)

// Client evaluates files with an OpenCode Zen model.
type Client struct {
	key   string
	model Model
	url   string
	http  *http.Client
}

// New returns a Client that evaluates with model, authenticating with the given
// OpenCode Zen API key.
func New(key string, model Model) *Client {
	return &Client{
		key:   key,
		model: model,
		url:   systemOneURL,
		http:  http.DefaultClient,
	}
}

var _ eval.Model = (*Client)(nil)

// Evaluate implements eval.Model. The request and response handling are not
// implemented yet.
func (c *Client) Evaluate(file *os.File, rules []rules.Rule) ([]string, error) {
	return nil, errors.New("opencode: Evaluate is not implemented")
}

func init() {
	eval.Register(eval.Provider{
		Name:    "opencode",
		EnvVar:  "OPENCODE_API_KEY",
		Models:  []string{string(Jev113), string(Jev113Free)},
		Default: string(Jev113Free),
		New: func(apiKey, model string) eval.Model {
			return New(apiKey, Model(model))
		},
	})
}
