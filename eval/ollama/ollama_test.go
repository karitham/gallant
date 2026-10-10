package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/madxmike/gallant/eval"
)

func TestRegistersProvider(t *testing.T) {
	p, ok := eval.Lookup("ollama")
	if !ok {
		t.Fatal("ollama provider is not registered")
	}
	if p.URLEnvVar != "OLLAMA_HOST" {
		t.Errorf("URLEnvVar = %q, want %q", p.URLEnvVar, "OLLAMA_HOST")
	}
	if p.Default == "" {
		t.Error("Default is empty")
	}
}

func TestHostURL(t *testing.T) {
	tests := map[string]string{
		"":                 defaultHost,
		"127.0.0.1:11434":  "http://127.0.0.1:11434",
		"http://box:1234":  "http://box:1234",
		"https://box:1234": "https://box:1234",
	}
	for host, want := range tests {
		if got := hostURL(host); got != want {
			t.Errorf("hostURL(%q) = %q, want %q", host, got, want)
		}
	}
}

// Evaluate must go through the shared System One client, reaching /v1/systemone
// without an Authorization header.
func TestEvaluateTargetsSystemOne(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.URL.Path
		json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{}})
	}))
	defer srv.Close()

	m := New(srv.URL, "nimble")
	if _, err := m.Evaluate(context.Background(), strings.NewReader("package main"), nil); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if path := <-got; path != "/v1/systemone" {
		t.Errorf("path = %q, want %q", path, "/v1/systemone")
	}
}

func TestListKeepsDecisionModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "nimble"}, {"id": "llama3"}},
			})
		case "/api/show":
			var req struct {
				Model string `json:"model"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			caps := map[string][]string{
				"nimble": {"completion", "decision"},
				"llama3": {"completion"},
			}
			json.NewEncoder(w).Encode(map[string]any{"capabilities": caps[req.Model]})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got, err := New(srv.URL, "nimble").List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if want := []string{"nimble"}; !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

// A server that answers /v1/models but not /api/show (llama.cpp, say) is treated
// as offering every listed model.
func TestListKeepsAllWhenShowUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "kev"}, {"id": "laya"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	got, err := New(srv.URL, "kev").List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if want := []string{"kev", "laya"}; !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func TestListFailsWhenModelsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "nimble").List(context.Background()); err == nil {
		t.Fatal("List() error = nil, want an error when /v1/models is unavailable")
	}
}
