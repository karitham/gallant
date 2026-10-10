package strandsdecider

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
	p, ok := eval.Lookup("strands-decider")
	if !ok {
		t.Fatal("strands-decider provider is not registered")
	}
	if p.URLEnvVar != "STRANDS_DECIDER_HOST" {
		t.Errorf("URLEnvVar = %q, want %q", p.URLEnvVar, "STRANDS_DECIDER_HOST")
	}
	if p.Default == "" {
		t.Error("Default is empty")
	}
}

func TestHostURL(t *testing.T) {
	tests := map[string]string{
		"":                defaultHost,
		"127.0.0.1:8000":  "http://127.0.0.1:8000",
		"http://box:9000": "http://box:9000",
		"https://box:443": "https://box:443",
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
	got := make(chan struct {
		path string
		auth string
	}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- struct {
			path string
			auth string
		}{r.URL.Path, r.Header.Get("Authorization")}
		json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{}})
	}))
	defer srv.Close()

	m := New(srv.URL, "strands-decider-2B-qwen3.5-v1-2610")
	if _, err := m.Evaluate(context.Background(), strings.NewReader("package main"), nil); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	capture := <-got
	if capture.path != "/v1/systemone" {
		t.Errorf("path = %q, want %q", capture.path, "/v1/systemone")
	}
	if capture.auth != "" {
		t.Errorf("Authorization = %q, want none", capture.auth)
	}
}

// List reads the single served checkpoint from GET /health.
func TestListReadsHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "model": "hobson-2b-recipe"})
	}))
	defer srv.Close()

	got, err := New(srv.URL, "").List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if want := []string{"hobson-2b-recipe"}; !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func TestListFailsWhenHealthUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "").List(context.Background()); err == nil {
		t.Fatal("List() error = nil, want an error when /health is unavailable")
	}
}
