package eval

import (
	"slices"
	"testing"
)

func TestRegisterAndLookup(t *testing.T) {
	Register(Provider{Name: "test-provider", EnvVar: "TEST_API_KEY"})

	p, ok := Lookup("test-provider")
	if !ok {
		t.Fatal("Lookup() did not find a registered provider")
	}
	if p.EnvVar != "TEST_API_KEY" {
		t.Errorf("EnvVar = %q, want %q", p.EnvVar, "TEST_API_KEY")
	}
}

func TestLookupUnknown(t *testing.T) {
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup() found an unregistered provider")
	}
}

func TestNames(t *testing.T) {
	Register(Provider{Name: "test-names"})
	if !slices.Contains(Names(), "test-names") {
		t.Error("Names() is missing a registered provider")
	}
}
