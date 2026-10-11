// Package ollama provides an eval.Model backed by a local Ollama server.
//
// Ollama (0.35.0+) serves System One decision models — nimble, tev1, clef,
// clef-flash, laya — through POST /v1/systemone, so the shared protocol client
// reaches them. Local requests need no API key.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/eval/systemone"
	"github.com/madxmike/gallant/rules"
)

// defaultHost is where Ollama listens when OLLAMA_HOST is unset.
const defaultHost = "http://127.0.0.1:11434"

// Model evaluates with Ollama and can list the decision models it offers.
type Model struct {
	client *systemone.Client
	host   string
}

// New returns a Model that evaluates with name on the Ollama server at host.
// An empty host falls back to defaultHost.
func New(host, name string) *Model {
	base := hostURL(host)
	return &Model{client: systemone.New(base, name, ""), host: base}
}

var (
	_ eval.Model  = (*Model)(nil)
	_ eval.Lister = (*Model)(nil)
)

// Evaluate implements eval.Model.
func (m *Model) Evaluate(ctx context.Context, r io.Reader, rs []rules.Rule) ([]eval.Finding, error) {
	return m.client.Evaluate(ctx, r, rs)
}

// List implements eval.Lister. It reads model names from GET /v1/models and
// keeps only those that POST /api/show marks with the "decision" capability. A
// server that answers /v1/models but not /api/show (llama.cpp, say) is treated
// as offering every listed model.
func (m *Model) List(ctx context.Context) ([]string, error) {
	ids, err := modelIDs(ctx, m.host)
	if err != nil {
		return nil, err
	}
	var decision []string
	for _, id := range ids {
		ok, err := isDecision(ctx, m.host, id)
		if err != nil || ok {
			// Ollama tags the default build ":latest"; report the bare name,
			// which is what the library and callers use.
			decision = append(decision, strings.TrimSuffix(id, ":latest"))
		}
	}
	return decision, nil
}

func init() {
	eval.Register(eval.Provider{
		Name:      "ollama",
		URLEnvVar: "OLLAMA_HOST",
		Default:   "nimble",
		New: func(_, host, model string) (eval.Model, error) {
			return New(host, model), nil
		},
	})
}

// hostURL resolves a base URL from a host value. Ollama's OLLAMA_HOST is
// conventionally scheme-less, such as "127.0.0.1:11434", so a missing scheme is
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

// discovery bounds the startup model listing so an unreachable or slow server
// does not stall gallant.
var discovery = &http.Client{Timeout: 5 * time.Second}

// modelIDs reads model names from the OpenAI-compatible /v1/models endpoint.
func modelIDs(ctx context.Context, base string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := discovery.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: %s/v1/models: %s", base, resp.Status)
	}

	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, err
	}
	ids := make([]string, len(models.Data))
	for i, m := range models.Data {
		ids[i] = m.ID
	}
	return ids, nil
}

// isDecision reports whether POST /api/show lists the "decision" capability. A
// non-200 response is returned as an error so the caller can fall back to
// keeping the model.
func isDecision(ctx context.Context, base, model string) (bool, error) {
	body, err := json.Marshal(map[string]string{"model": model})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/show", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := discovery.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("ollama: %s/api/show: %s", base, resp.Status)
	}

	var show struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&show); err != nil {
		return false, err
	}
	return slices.Contains(show.Capabilities, "decision"), nil
}
