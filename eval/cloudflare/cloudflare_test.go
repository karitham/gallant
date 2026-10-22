package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madxmike/gallant/eval"
	"github.com/madxmike/gallant/rules"
)

func TestRegistersProvider(t *testing.T) {
	p, ok := eval.Lookup("cloudflare")
	if !ok {
		t.Fatal("cloudflare provider is not registered")
	}
	if p.KeyEnvVar != keyEnvVar {
		t.Errorf("KeyEnvVar = %q, want %q", p.KeyEnvVar, keyEnvVar)
	}
	if p.Default != string(Clef) {
		t.Errorf("Default = %q, want %q", p.Default, Clef)
	}

	t.Setenv(accountEnvVar, "")
	if _, err := p.New("tok", "", string(Clef)); err == nil || !strings.Contains(err.Error(), accountEnvVar) {
		t.Errorf("New() without account error = %v, want it to name %s", err, accountEnvVar)
	}
	t.Setenv(accountEnvVar, "acct")
	if _, err := p.New("tok", "", string(Clef)); err != nil {
		t.Errorf("New() with account error = %v", err)
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	tests := []struct {
		name, token, account, want string
	}{
		{name: "no token", account: "acct", want: keyEnvVar},
		{name: "no account", token: "tok", want: accountEnvVar},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.token, tt.account, Clef)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("New() error = %v, want it to name %s", err, tt.want)
			}
		})
	}
}

// More rules than Workers AI accepts are refused before anything is sent.
func TestEvaluateRejectsTooManyRules(t *testing.T) {
	m, err := New("tok", "acct", Clef)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = m.Evaluate(context.Background(), strings.NewReader(""), make([]rules.Rule, maxQuestions+1))
	if err == nil || !strings.Contains(err.Error(), "65 rules") {
		t.Fatalf("Evaluate() error = %v, want it to name the rule count", err)
	}
}

func TestUnwrap(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr string
	}{
		{name: "success", status: http.StatusOK, body: `{"success":true,"errors":[],"result":{"answers":{}}}`, want: `{"answers":{}}`},
		{name: "api errors", status: http.StatusBadRequest, body: `{"success":false,"errors":[{"code":5006,"message":"bad questions"}]}`, wantErr: "400 Bad Request: bad questions (code 5006)"},
		{name: "unsuccessful 200", status: http.StatusOK, body: `{"success":false,"errors":[{"code":1,"message":"nope"}]}`, wantErr: "nope"},
		{name: "unsuccessful without message", status: http.StatusOK, body: `{"success":false}`, wantErr: "without an error message"},
		{name: "bare status", status: http.StatusBadGateway, body: "<html>", wantErr: "502 Bad Gateway"},
		{name: "malformed body", status: http.StatusOK, body: "not json", wantErr: "unexpected response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unwrap(tt.status, []byte(tt.body))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("unwrap() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unwrap() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("unwrap() = %s, want %s", got, tt.want)
			}
		})
	}
}

// newPost is the one transport test: it runs the model's endpoint under the
// account with the token, and returns the unwrapped result.
func TestPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/acct/ai/run/@cf/cloudflare/clef-flash" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer tok")
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{"answers": map[string]any{}}})
	}))
	defer srv.Close()

	got, err := newPost(srv.URL+"/accounts/acct", "tok")(context.Background(), string(ClefFlash), []byte("{}"))
	if err != nil {
		t.Fatalf("post() error = %v", err)
	}
	if string(got) != `{"answers":{}}` {
		t.Errorf("post() = %s, want the unwrapped result", got)
	}
}
