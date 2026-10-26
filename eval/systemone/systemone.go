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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
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

// finding maps an answer to every outcome the model weighed, most likely
// first. Choice and score answers carry a probability for each option or level;
// a noul carries a single probability, whose two outcomes are fail and pass.
func finding(r rules.Rule, a answer) eval.Finding {
	var outcomes []eval.Outcome
	switch r.Type {
	case rules.TypeChoice:
		outcomes = choiceOutcomes(a)
	case rules.TypeScore:
		outcomes = scoreOutcomes(r, a)
	default:
		outcomes = []eval.Outcome{
			{Result: "fail", Confidence: roundProbability(a.Noul)},
			{Result: "pass", Confidence: roundProbability(1 - a.Noul)},
		}
	}
	return eval.Finding{Rule: r.Path, Type: r.Type, Outcomes: outcomes}
}

// roundProbability trims binary floating-point noise from a reported or derived
// probability, so a noul's complement (1 - 0.9) reads as 0.1 rather than
// 0.09999999999999998, and a server's 0.9400000000000001 reads as 0.94.
func roundProbability(p float64) float64 {
	return math.Round(p*1e4) / 1e4
}

// choiceOutcomes returns a result for every option, most likely first. Equal
// probabilities keep the options' alphabetical order so a run is reproducible.
func choiceOutcomes(a answer) []eval.Outcome {
	if len(a.Probabilities) == 0 {
		return []eval.Outcome{{Result: a.Choice, Confidence: roundProbability(a.Confidence)}}
	}
	options := make([]string, 0, len(a.Probabilities))
	for option := range a.Probabilities {
		options = append(options, option)
	}
	slices.Sort(options)
	out := make([]eval.Outcome, len(options))
	for i, option := range options {
		out[i] = eval.Outcome{Result: option, Confidence: roundProbability(a.Probabilities[option])}
	}
	sortByLikelihood(out)
	return out
}

// scoreOutcomes returns a result for every level, most likely first. Levels are
// keyed by their number; a number the rubric does not name is kept as it is so
// the distribution stays complete.
func scoreOutcomes(r rules.Rule, a answer) []eval.Outcome {
	if len(a.Probabilities) == 0 {
		return []eval.Outcome{{Result: scoreLabel(r, int(a.Score)), Confidence: roundProbability(a.Confidence)}}
	}
	levels := make([]string, 0, len(a.Probabilities))
	for level := range a.Probabilities {
		levels = append(levels, level)
	}
	slices.SortFunc(levels, compareLevels)
	out := make([]eval.Outcome, len(levels))
	for i, level := range levels {
		label := level
		if n, err := strconv.Atoi(level); err == nil {
			label = scoreLabel(r, n)
		}
		out[i] = eval.Outcome{Result: label, Confidence: roundProbability(a.Probabilities[level])}
	}
	sortByLikelihood(out)
	return out
}

// scoreLabel names level n, falling back to its number when the rubric has no
// such level.
func scoreLabel(r rules.Rule, n int) string {
	if n >= 0 && n < len(r.Criteria.Levels) {
		return r.Criteria.Levels[n]
	}
	return strconv.Itoa(n)
}

// compareLevels orders level keys by number, falling back to text for a key
// that is not a number.
func compareLevels(a, b string) int {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	if aerr == nil && berr == nil {
		return cmp.Compare(ai, bi)
	}
	return strings.Compare(a, b)
}

// sortByLikelihood puts the most probable outcome first. It is stable, so
// equal probabilities keep the order they were gathered in.
func sortByLikelihood(out []eval.Outcome) {
	slices.SortStableFunc(out, func(a, b eval.Outcome) int {
		return cmp.Compare(b.Confidence, a.Confidence)
	})
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
// question is true; Choice and Score are the most likely option and level,
// Confidences summarize how spread their probabilities are, and Probabilities
// holds the full distribution (option to probability, or level number to
// probability).
type answer struct {
	Type          rules.Type         `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
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
