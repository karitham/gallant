// Package systemone implements the System One decision protocol.
//
// A System One server answers a state against typed questions and returns
// typed answers and probabilities instead of generated text. OpenCode Zen
// (Jev), TypeSafe, Ollama, llama.cpp, Ollaya and Strands Decider expose it at
// POST /v1/systemone, and HTTP posts to that endpoint. Cloudflare Workers AI
// serves the same body at its own endpoint and envelope, so eval/cloudflare
// supplies its own Post. See also eval/opencode, eval/ollama and
// eval/strandsdecider.
package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

// endpoint is the path, appended to a server's base URL, that answers
// questions about a state.
const endpoint = "/v1/systemone"

// Post sends a System One request body for model and returns the response
// body, the object holding "answers". It reports a failure of the server or
// the transport as an *eval.ProviderError that names the endpoint.
type Post func(ctx context.Context, model string, body []byte) ([]byte, error)

// New returns an eval.Model that asks model its questions through post.
func New(post Post, model string) eval.Model {
	return &evaluator{post: post, model: model}
}

type evaluator struct {
	post  Post
	model string
}

// Evaluate implements eval.Model by asking the model about every rule in one
// request.
func (e *evaluator) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	state, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(newRequest(e.model, string(state), rs))
	if err != nil {
		return nil, err
	}

	raw, err := e.post(ctx, e.model, body)
	if err != nil {
		return nil, err
	}

	var res response
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, &eval.ProviderError{Err: fmt.Errorf("systemone: unexpected response: %w", err)}
	}
	return findings(res, rs)
}

// questionID names the question asked for the rule at index i. Rule paths may
// hold characters a server rejects in ids, such as "/", so ids are positional.
func questionID(i int) string {
	return "q" + strconv.Itoa(i)
}

func newRequest(model, state string, rs []rules.Rule) request {
	questions := make(map[string]question, len(rs))
	for i, r := range rs {
		var criteria any
		switch {
		case r.Criteria.Options != nil:
			criteria = r.Criteria.Options
		case r.Criteria.Levels != nil:
			criteria = r.Criteria.Levels
		}
		questions[questionID(i)] = question{Type: r.Type, Instructions: r.Body, Criteria: criteria}
	}
	return request{Model: model, State: state, Questions: questions}
}

// findings maps each rule to the answer given for its question.
func findings(res response, rs []rules.Rule) ([]eval.Finding, error) {
	out := make([]eval.Finding, len(rs))
	for i, r := range rs {
		a, ok := res.Answers[questionID(i)]
		if !ok {
			// A question the server did not answer would otherwise look like a
			// clean pass, so report it rather than dropping the finding.
			return nil, &eval.ProviderError{Err: fmt.Errorf("systemone: no answer for rule %q", r.Path)}
		}
		out[i] = finding(r, a)
	}
	return out, nil
}

func finding(r rules.Rule, a answer) eval.Finding {
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
	return f
}

// HTTP returns a Post that sends requests to the System One endpoint under
// baseURL, such as "http://localhost:11434". apiKey is optional and sent as a
// Bearer token only when non-empty. The returned Post ignores its model
// argument, because the body already names the model.
func HTTP(baseURL, apiKey string) Post {
	u := strings.TrimRight(baseURL, "/") + endpoint
	client := &http.Client{Timeout: 30 * time.Second}

	return func(ctx context.Context, _ string, body []byte) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
		if err != nil {
			return nil, &eval.ProviderError{Err: fmt.Errorf("systemone: %s: %w", u, err)}
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, &eval.ProviderError{Err: fmt.Errorf("systemone: %s: %w", u, requestCause(err))}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, &eval.ProviderError{Err: fmt.Errorf("systemone: %s: %s%s", u, resp.Status, bodySuffix(resp.Body))}
		}

		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, &eval.ProviderError{Err: fmt.Errorf("systemone: %s: read response: %w", u, err)}
		}
		return raw, nil
	}
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

// requestCause strips the method-and-URL wrapper net/http puts around a
// transport failure, so the diagnostic does not name the URL twice.
func requestCause(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// bodySuffix reads a short, single-line snippet of an error response so the
// server's own explanation reaches the user. It is bounded so a large or HTML
// error page cannot flood the terminal.
func bodySuffix(body io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(body, 512))
	s := strings.Join(strings.Fields(string(b)), " ")
	if s == "" {
		return ""
	}
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return ": " + s
}
