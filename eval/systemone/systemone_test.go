package systemone

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

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

	c := New(srv.URL, "jev-1.13-free", "secret")
	c.http = srv.Client()

	findings, err := c.Evaluate(context.Background(), strings.NewReader("package main"), []rules.Rule{
		{Path: "a.md", Type: rules.TypeNoul, Body: "rule A"},
		{Path: "b.md", Type: rules.TypeNoul, Body: "rule B"},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	want := []eval.Finding{
		{Rule: "a.md", Type: rules.TypeNoul, Confidence: 0.9},
		{Rule: "b.md", Type: rules.TypeNoul, Confidence: 0.1},
	}
	if !reflect.DeepEqual(findings, want) {
		t.Errorf("Evaluate() = %#v, want %#v", findings, want)
	}

	capture := <-got
	if capture.auth != "Bearer secret" {
		t.Errorf("Authorization = %q, want %q", capture.auth, "Bearer secret")
	}
	if capture.req.Model != "jev-1.13-free" {
		t.Errorf("model = %q, want %q", capture.req.Model, "jev-1.13-free")
	}
	if capture.req.State != "package main" {
		t.Errorf("state = %q, want %q", capture.req.State, "package main")
	}
	if len(capture.req.Questions) != 2 {
		t.Errorf("questions = %d, want 2", len(capture.req.Questions))
	}
}

// A client built without a key must not send an Authorization header, so local
// servers that ignore authentication are not handed an empty Bearer token.
func TestEvaluateWithoutKeyOmitsAuth(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(response{})
	}))
	defer srv.Close()

	c := New(srv.URL, "nimble", "")
	c.http = srv.Client()
	if _, err := c.Evaluate(context.Background(), strings.NewReader(""), nil); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if auth := <-got; auth != "" {
		t.Errorf("Authorization = %q, want none", auth)
	}
}

func TestEvaluateOtherTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(response{
			Answers: map[string]answer{
				"s.md": {Type: "choice", Choice: "violation", Confidence: 0.82},
				"r.md": {Type: "score", Score: 1, Confidence: 0.9},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "nimble", "")
	c.http = srv.Client()
	findings, err := c.Evaluate(context.Background(), strings.NewReader(""), []rules.Rule{
		{Path: "s.md", Type: rules.TypeChoice, Criteria: rules.Criteria{Options: map[string]string{"compliant": "ok", "violation": "bad"}}},
		{Path: "r.md", Type: rules.TypeScore, Criteria: rules.Criteria{Levels: []string{"low", "mid", "high"}}},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	want := []eval.Finding{
		{Rule: "s.md", Type: rules.TypeChoice, Result: "violation", Confidence: 0.82},
		{Rule: "r.md", Type: rules.TypeScore, Result: "mid", Confidence: 0.9},
	}
	if !reflect.DeepEqual(findings, want) {
		t.Errorf("Evaluate() = %#v, want %#v", findings, want)
	}
}

func TestEvaluateHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, "nimble", "")
	c.http = srv.Client()
	if _, err := c.Evaluate(context.Background(), strings.NewReader(""), []rules.Rule{{Path: "a.md", Body: "x"}}); err == nil {
		t.Fatal("Evaluate() error = nil, want an error for a non-200 response")
	}
}
