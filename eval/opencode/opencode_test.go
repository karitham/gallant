package opencode

import (
	"testing"

	"github.com/madxmike/gallant/eval"
)

func TestRegistersProvider(t *testing.T) {
	p, ok := eval.Lookup("opencode")
	if !ok {
		t.Fatal("opencode provider is not registered")
	}
	if p.KeyEnvVar != "OPENCODE_API_KEY" {
		t.Errorf("KeyEnvVar = %q, want %q", p.KeyEnvVar, "OPENCODE_API_KEY")
	}
	if len(p.Models) == 0 {
		t.Error("Models is empty")
	}
	if p.Default == "" {
		t.Error("Default is empty")
	}
}

// The free model needs no key; a paid model without one is refused up front
// rather than left to fail as a stream of 401s.
func TestNewRequiresKeyForPaidModel(t *testing.T) {
	if _, err := New("", Jev113Free); err != nil {
		t.Errorf("New(free, no key) error = %v, want nil", err)
	}
	if _, err := New("key", Jev113); err != nil {
		t.Errorf("New(paid, key) error = %v, want nil", err)
	}
	if _, err := New("", Jev113); err == nil {
		t.Error("New(paid, no key) error = nil, want an error naming the missing key")
	}
}

func TestRegisteredProviderPropagatesKeyRequirement(t *testing.T) {
	p, ok := eval.Lookup("opencode")
	if !ok {
		t.Fatal("opencode provider is not registered")
	}
	if _, err := p.New("", "", string(Jev113)); err == nil {
		t.Error("provider New(paid, no key) error = nil, want an error")
	}
}
