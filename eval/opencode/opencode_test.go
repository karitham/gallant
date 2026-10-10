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
