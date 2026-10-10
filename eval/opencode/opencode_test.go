package opencode

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

func TestRegistersProvider(t *testing.T) {
	p, ok := eval.Lookup("opencode")
	if !ok {
		t.Fatal("opencode provider is not registered")
	}
	if p.EnvVar != "OPENCODE_API_KEY" {
		t.Errorf("EnvVar = %q, want %q", p.EnvVar, "OPENCODE_API_KEY")
	}
	if len(p.Models) == 0 {
		t.Error("Models is empty")
	}
	if p.Default == "" {
		t.Error("Default is empty")
	}
}

type captured struct {
	auth string
	req  request
}

func TestEvaluate(t *testing.T) {
	got := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		got <- captured{auth: r.Header.Get("Authorization"), req: req}
		json.NewEncoder(w).Encode(response{
			Answers: map[string]answer{
				"a.md": {Type: "noul", Noul: 0.9},
				"b.md": {Type: "noul", Noul: 0.1},
			},
		})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	c := &Client{key: "secret", model: Jev113Free, url: srv.URL, http: srv.Client()}

	findings, err := c.Evaluate(file, []rules.Rule{
		{Path: "a.md", Body: "rule A"},
		{Path: "b.md", Body: "rule B"},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !reflect.DeepEqual(findings, []string{"a.md"}) {
		t.Errorf("Evaluate() = %#v, want [a.md]", findings)
	}

	capture := <-got
	if capture.auth != "Bearer secret" {
		t.Errorf("Authorization = %q, want %q", capture.auth, "Bearer secret")
	}
	if capture.req.Model != string(Jev113Free) {
		t.Errorf("model = %q, want %q", capture.req.Model, Jev113Free)
	}
	if capture.req.State != "package main" {
		t.Errorf("state = %q, want %q", capture.req.State, "package main")
	}
	if len(capture.req.Questions) != 2 {
		t.Errorf("questions = %d, want 2", len(capture.req.Questions))
	}
}

func TestEvaluateHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	file, err := os.CreateTemp(t.TempDir(), "rule")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	c := &Client{url: srv.URL, http: srv.Client()}
	if _, err := c.Evaluate(file, []rules.Rule{{Path: "a.md", Body: "x"}}); err == nil {
		t.Fatal("Evaluate() error = nil, want an error for a non-200 response")
	}
}
