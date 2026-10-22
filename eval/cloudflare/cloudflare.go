// Package cloudflare provides an eval.Model backed by Cloudflare Workers AI,
// which serves Cloudflare's Clef decision models.
//
// Workers AI takes the System One request body at a per-model endpoint under
// the account. It wraps the answers in Cloudflare's {"result", "success",
// "errors"} envelope, so this package supplies its own systemone.Post.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/eval/systemone"
	"github.com/madxmike/gallant/rules"
)

// baseURL is the Cloudflare API; the account and model are appended per request.
const baseURL = "https://api.cloudflare.com/client/v4"

const (
	// keyEnvVar names the environment variable that holds a Workers AI API token.
	keyEnvVar = "CLOUDFLARE_API_TOKEN"
	// accountEnvVar names the environment variable that holds the account id.
	// The provider registry has no slot for it, so New's caller reads it here.
	accountEnvVar = "CLOUDFLARE_ACCOUNT_ID"
)

// maxQuestions is the most questions Workers AI accepts in one request.
const maxQuestions = 64

// maxResponseBytes bounds how much of a response is read, so a misbehaving
// endpoint cannot exhaust memory.
const maxResponseBytes = 4 << 20

// Model identifies a decision model served by Workers AI.
type Model string

const (
	Clef      Model = "clef"
	ClefFlash Model = "clef-flash"
)

// New returns an eval.Model that evaluates with model on Workers AI, billed to
// account and authenticated with token. Both are required, so a missing one is
// reported here rather than as a stream of rejected requests.
func New(token, account string, model Model) (eval.Model, error) {
	if token == "" {
		return nil, fmt.Errorf("cloudflare: %s is not set", keyEnvVar)
	}
	if account == "" {
		return nil, fmt.Errorf("cloudflare: %s is not set", accountEnvVar)
	}
	post := newPost(baseURL+"/accounts/"+url.PathEscape(account), token)
	return capped{systemone.New(post, string(model))}, nil
}

// capped refuses a file with more applicable rules than one request carries,
// before the file is sent.
type capped struct{ eval.Model }

func (c capped) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	if len(rs) > maxQuestions {
		return nil, fmt.Errorf("cloudflare: %d rules apply, more than the %d Workers AI accepts in one request", len(rs), maxQuestions)
	}
	return c.Model.Evaluate(ctx, r, rs)
}

func init() {
	eval.Register(eval.Provider{
		Name:      "cloudflare",
		KeyEnvVar: keyEnvVar,
		Models:    []string{string(Clef), string(ClefFlash)},
		Default:   string(Clef),
		New: func(apiKey, _ string, model string) (eval.Model, error) {
			return New(apiKey, os.Getenv(accountEnvVar), Model(model))
		},
	})
}

// newPost returns a Post that runs models under accountURL, the account's API
// root, and unwraps Cloudflare's envelope.
func newPost(accountURL, token string) systemone.Post {
	client := &http.Client{Timeout: 30 * time.Second}

	return func(ctx context.Context, model string, body []byte) ([]byte, error) {
		u := accountURL + "/ai/run/@cf/cloudflare/" + url.PathEscape(model)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
		if err != nil {
			return nil, &eval.ProviderError{Err: fmt.Errorf("cloudflare: %s: %w", u, err)}
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, &eval.ProviderError{Err: fmt.Errorf("cloudflare: %s: %w", u, requestCause(err))}
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		if err != nil {
			return nil, &eval.ProviderError{Err: fmt.Errorf("cloudflare: %s: read response: %w", u, err)}
		}

		result, err := unwrap(resp.StatusCode, raw)
		if err != nil {
			return nil, &eval.ProviderError{Err: fmt.Errorf("cloudflare: %s: %w", u, err)}
		}
		return result, nil
	}
}

// envelope is the wrapper the Cloudflare API puts around every response.
type envelope struct {
	Result  json.RawMessage `json:"result"`
	Success bool            `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// unwrap returns the result of a successful response, or an error carrying
// Cloudflare's own messages, falling back to the status when it sent none.
func unwrap(status int, raw []byte) (json.RawMessage, error) {
	var env envelope
	decodeErr := json.Unmarshal(raw, &env)

	if status == http.StatusOK && decodeErr == nil && env.Success {
		return env.Result, nil
	}
	if len(env.Errors) > 0 {
		msgs := make([]string, len(env.Errors))
		for i, e := range env.Errors {
			msgs[i] = fmt.Sprintf("%s (code %d)", e.Message, e.Code)
		}
		return nil, fmt.Errorf("%d %s: %s", status, http.StatusText(status), strings.Join(msgs, "; "))
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%d %s", status, http.StatusText(status))
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("unexpected response: %w", decodeErr)
	}
	return nil, errors.New("request failed without an error message")
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
