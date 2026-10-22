package systemone

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

// fakePost answers every request with reply and records the request it was
// sent in *sent.
func fakePost(t *testing.T, reply string, sent *request) Post {
	t.Helper()
	return func(_ context.Context, _ string, body []byte) ([]byte, error) {
		if err := json.Unmarshal(body, sent); err != nil {
			t.Errorf("decode request: %v", err)
		}
		return []byte(reply), nil
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name  string
		rules []rules.Rule
		reply string
		want  []eval.Finding
	}{
		{
			name: "noul",
			rules: []rules.Rule{
				{Path: "a.md", Type: rules.TypeNoul, Body: "rule A"},
				{Path: "b.md", Type: rules.TypeNoul, Body: "rule B"},
			},
			reply: `{"answers":{"q0":{"type":"noul","noul":0.9},"q1":{"type":"noul","noul":0.1}}}`,
			want: []eval.Finding{
				{Rule: "a.md", Type: rules.TypeNoul, Confidence: 0.9},
				{Rule: "b.md", Type: rules.TypeNoul, Confidence: 0.1},
			},
		},
		{
			name: "choice and score",
			rules: []rules.Rule{
				{Path: "s.md", Type: rules.TypeChoice, Criteria: rules.Criteria{Options: map[string]string{"compliant": "ok", "violation": "bad"}}},
				{Path: "r.md", Type: rules.TypeScore, Criteria: rules.Criteria{Levels: []string{"low", "mid", "high"}}},
			},
			reply: `{"answers":{"q0":{"type":"choice","choice":"violation","confidence":0.82},"q1":{"type":"score","score":1,"confidence":0.9}}}`,
			want: []eval.Finding{
				{Rule: "s.md", Type: rules.TypeChoice, Result: "violation", Confidence: 0.82},
				{Rule: "r.md", Type: rules.TypeScore, Result: "mid", Confidence: 0.9},
			},
		},
		{
			// A nested path is not a valid question id on every server, so it
			// never reaches the wire; the finding still names the rule.
			name:  "nested path",
			rules: []rules.Rule{{Path: "go/style.md", Type: rules.TypeNoul, Body: "A"}},
			reply: `{"answers":{"q0":{"type":"noul","noul":0.2}}}`,
			want:  []eval.Finding{{Rule: "go/style.md", Type: rules.TypeNoul, Confidence: 0.2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sent request
			got, err := New(fakePost(t, tt.reply, &sent), "nimble").Evaluate(context.Background(), strings.NewReader("package main"), tt.rules)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Evaluate() = %#v, want %#v", got, tt.want)
			}

			if sent.Model != "nimble" || sent.State != "package main" {
				t.Errorf("request model, state = %q, %q", sent.Model, sent.State)
			}
			for i, r := range tt.rules {
				q, ok := sent.Questions[questionID(i)]
				if !ok || q.Instructions != r.Instructions() {
					t.Errorf("question %s = %#v, want the instructions of %s", questionID(i), q, r.Path)
				}
			}
		})
	}
}

// A malformed answer, or a question the server did not answer, is a provider
// fault, never a clean pass.
func TestEvaluateReportsProviderFaults(t *testing.T) {
	tests := []struct {
		name  string
		reply string
	}{
		{name: "malformed body", reply: "not json"},
		{name: "missing answer", reply: `{"answers":{"q0":{"type":"noul","noul":0.9}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sent request
			_, err := New(fakePost(t, tt.reply, &sent), "nimble").Evaluate(context.Background(), strings.NewReader(""), []rules.Rule{
				{Path: "a.md", Type: rules.TypeNoul, Body: "A"},
				{Path: "b.md", Type: rules.TypeNoul, Body: "B"},
			})
			var pe *eval.ProviderError
			if !errors.As(err, &pe) {
				t.Fatalf("Evaluate() error = %v, want a ProviderError", err)
			}
		})
	}
}

func TestEvaluatePassesPostErrorThrough(t *testing.T) {
	want := &eval.ProviderError{Err: errors.New("down")}
	post := func(context.Context, string, []byte) ([]byte, error) { return nil, want }
	_, err := New(post, "nimble").Evaluate(context.Background(), strings.NewReader(""), []rules.Rule{{Path: "a.md"}})
	if err != want {
		t.Fatalf("Evaluate() error = %v, want %v", err, want)
	}
}

// HTTP is the one transport test: it posts to the System One path, sends the
// key only when there is one, and reports a non-200 status as a provider fault.
func TestHTTP(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		status   int
		wantAuth string
		wantErr  bool
	}{
		{name: "with key", key: "secret", status: http.StatusOK, wantAuth: "Bearer secret"},
		{name: "without key", status: http.StatusOK},
		{name: "error status", status: http.StatusUnauthorized, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != endpoint {
					t.Errorf("path = %q, want %q", r.URL.Path, endpoint)
				}
				if got := r.Header.Get("Authorization"); got != tt.wantAuth {
					t.Errorf("Authorization = %q, want %q", got, tt.wantAuth)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte("{}"))
			}))
			defer srv.Close()

			_, err := HTTP(srv.URL, tt.key)(context.Background(), "nimble", []byte("{}"))
			var pe *eval.ProviderError
			if tt.wantErr != errors.As(err, &pe) {
				t.Fatalf("post() error = %v, want provider error: %v", err, tt.wantErr)
			}
		})
	}
}

func TestBodySuffix(t *testing.T) {
	tests := map[string]string{
		"":                       "",
		"  \n\t ":                "",
		"boom":                   ": boom",
		"a\nb":                   ": a b",
		"  spaced   out  ":       ": spaced out",
		strings.Repeat("x", 300): ": " + strings.Repeat("x", 200) + "...",
	}
	for in, want := range tests {
		if got := bodySuffix(strings.NewReader(in)); got != want {
			t.Errorf("bodySuffix(%q) = %q, want %q", in, got, want)
		}
	}
}
