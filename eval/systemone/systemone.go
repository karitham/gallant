// Package systemone implements the System One decision protocol over HTTP.
//
// A System One server answers a state against typed questions and returns
// typed answers and probabilities instead of generated text. OpenCode Zen
// (Jev), TypeSafe, Ollama, llama.cpp and Ollaya all expose the same
// POST /v1/systemone contract, so one client reaches them all. Providers
// supply the base URL and any API key; see eval/opencode and eval/ollama.
package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

// endpoint is the path, appended to a server's base URL, that answers
// questions about a state.
const endpoint = "/v1/systemone"

// Client evaluates files with a System One server.
type Client struct {
	url   string
	model string
	key   string
	http  *http.Client
}

// New returns a Client that evaluates with model at baseURL, such as
// "http://localhost:11434". The System One path is appended to baseURL. apiKey
// is optional and sent as a Bearer token only when non-empty.
func New(baseURL, model, apiKey string) *Client {
	return &Client{
		url:   strings.TrimRight(baseURL, "/") + endpoint,
		model: model,
		key:   apiKey,
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
		var criteria any
		switch {
		case r.Criteria.Options != nil:
			criteria = r.Criteria.Options
		case r.Criteria.Levels != nil:
			criteria = r.Criteria.Levels
		}
		questions[r.Path] = question{Type: r.Type, Instructions: r.Instructions(), Criteria: criteria}
	}

	body, err := json.Marshal(request{
		Model:     c.model,
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
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("systemone: %s: %s", c.url, resp.Status)
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
	Type         rules.Type `json:"type"`
	Instructions string     `json:"instructions"`
	Criteria     any        `json:"criteria,omitempty"`
}

type response struct {
	Answers map[string]answer `json:"answers"`
}

// answer is the response for one question. Noul is the probability that the
// question is true; Choice is the chosen option; Score is the chosen level.
type answer struct {
	Type       rules.Type `json:"type"`
	Noul       float64    `json:"noul"`
	Choice     string     `json:"choice"`
	Score      float64    `json:"score"`
	Confidence float64    `json:"confidence"`
}
