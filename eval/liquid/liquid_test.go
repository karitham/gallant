package liquid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The hosted API requires a key; a local llama-server does not.
func TestNewRequiresKeyOnlyForHostedAPI(t *testing.T) {
	if _, err := New("", "", defaultModel); err == nil {
		t.Error("New(hosted, no key) error = nil, want an error naming the missing key")
	}
	if _, err := New("key", "", defaultModel); err != nil {
		t.Errorf("New(hosted, key) error = %v, want nil", err)
	}
	if _, err := New("", "http://127.0.0.1:8080", "d1-2B"); err != nil {
		t.Errorf("New(local, no key) error = %v, want nil", err)
	}
}

func TestEvaluateTargetsSystemOneWithBearerToken(t *testing.T) {
	var path, auth, model string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		model = req.Model
		json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{}})
	}))
	defer srv.Close()

	m, err := New("test-key", srv.URL, defaultModel)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := m.Evaluate(context.Background(), strings.NewReader("package main"), nil); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if path != "/v1/systemone" {
		t.Errorf("path = %q, want %q", path, "/v1/systemone")
	}
	if auth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer test-key")
	}
	if model != defaultModel {
		t.Errorf("model = %q, want %q", model, defaultModel)
	}
}
