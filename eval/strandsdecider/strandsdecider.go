// Package strandsdecider provides an eval.Model backed by a local Strands
// Decider server.
//
// Strands Decider (0.1.0+) is AWS Strands Labs' open-source decision model,
// served with `strands-decider serve`. It answers the System One contract at
// POST /v1/systemone, so the shared protocol client reaches it. The server
// binds 127.0.0.1 and needs no API key.
package strandsdecider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/eval/systemone"
	"github.com/madxmike/gallant/rules"
)

// defaultHost is where strands-decider serve listens when STRANDS_DECIDER_HOST
// is unset.
const defaultHost = "http://127.0.0.1:8000"

// Model evaluates with a Strands Decider server and can report the checkpoint
// it serves.
type Model struct {
	client *systemone.Client
	host   string

	// mu serialises evaluations. A Strands Decider server runs one request at a
	// time (a single worker owns the accelerator), and its Metal/CUDA backend
	// faults on concurrent inference, so requests must not overlap however many
	// files gallant evaluates at once.
	mu sync.Mutex
}

// New returns a Model that evaluates with name on the Strands Decider server at
// host. An empty host falls back to defaultHost. A Strands Decider server serves
// the single checkpoint it was started with and ignores name; name is sent for
// compatibility with the System One contract and, in practice, is the name List
// reports.
func New(host, name string) *Model {
	base := hostURL(host)
	return &Model{client: systemone.New(base, name, ""), host: base}
}

var (
	_ eval.Model  = (*Model)(nil)
	_ eval.Lister = (*Model)(nil)
)

// Evaluate implements eval.Model. It serialises requests to the server.
func (m *Model) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.Evaluate(ctx, r, rs)
}

// List implements eval.Lister. A Strands Decider server serves exactly one
// checkpoint, named in GET /health, so the set has at most one member.
func (m *Model) List(ctx context.Context) ([]string, error) {
	name, err := health(ctx, m.host)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, nil
	}
	return []string{name}, nil
}

func init() {
	eval.Register(eval.Provider{
		Name:      "strands-decider",
		URLEnvVar: "STRANDS_DECIDER_HOST",
		Default:   "strands-decider-2B-qwen3.5-v1-2610",
		New: func(_, host, model string) eval.Model {
			return New(host, model)
		},
	})
}

// hostURL resolves a base URL from a host value. STRANDS_DECIDER_HOST is
// conventionally scheme-less, such as "127.0.0.1:8000", so a missing scheme is
// filled in; an empty value selects the default host.
func hostURL(host string) string {
	if host == "" {
		return defaultHost
	}
	if !strings.Contains(host, "://") {
		return "http://" + host
	}
	return host
}

// discovery bounds the startup health check so an unreachable or slow server
// does not stall gallant.
var discovery = &http.Client{Timeout: 5 * time.Second}

// health reads the served model name from GET /health.
func health(ctx context.Context, base string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
	if err != nil {
		return "", err
	}
	resp, err := discovery.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("strands-decider: %s/health: %s", base, resp.Status)
	}

	var h struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return "", err
	}
	return h.Model, nil
}
