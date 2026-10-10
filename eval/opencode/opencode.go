// Package opencode provides an eval.Model backed by OpenCode's Zen gateway.
package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

var _ eval.Model = (*Client)(nil)

// Evaluate implements eval.Model by asking the model, once per rule, whether
// the input violates it.
func (c *Client) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	state, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	questions := make(map[string]question, len(rs))
	for _, r := range rs {
		instructions := r.Body
		if r.Type == rules.TypeNoul {
			instructions = "Does this file violate the following rule?\n\n" + r.Body
		}
		var criteria any
		switch {
		case r.Criteria.Options != nil:
			criteria = r.Criteria.Options
		case r.Criteria.Levels != nil:
			criteria = r.Criteria.Levels
		}
		questions[r.Path] = question{Type: string(r.Type), Instructions: instructions, Criteria: criteria}
	}

	body, err := json.Marshal(request{
		Model:     string(c.model),
		State:     string(state),
		Questions: questions,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode: %s: %s", c.url, resp.Status)
	}

	var res response
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var findings []eval.Finding
	for _, r := range rs {
		a, ok := res.Answers[r.Path]
		if !ok {
			continue
		}
		f := eval.Finding{Rule: r.Path, Type: r.Type}
		switch r.Type {
		case rules.TypeChoice:
			f.Result = a.Choice
			f.Confidence = a.Confidence
		case rules.TypeScore:
			f.Confidence = a.Confidence
			if i := int(a.Score); i >= 0 && i < len(r.Criteria.Levels) {
				f.Result = r.Criteria.Levels[i]
			} else {
				f.Result = fmt.Sprintf("%g", a.Score)
			}
		default:
			f.Confidence = a.Noul
		}
		findings = append(findings, f)
	}
	return findings, nil
}

type request struct {
	Model     string              `json:"model"`
	State     string              `json:"state"`
	Questions map[string]question `json:"questions"`
}

type question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type response struct {
	Answers map[string]answer `json:"answers"`
}

// answer is the response for one question. Noul is the probability that the
// question is true; Choice is the chosen option; Score is the chosen level.
type answer struct {
	Type       string  `json:"type"`
	Noul       float64 `json:"noul"`
	Choice     string  `json:"choice"`
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
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
