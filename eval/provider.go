package eval

import "slices"

// Provider describes a model provider and how to construct its models.
type Provider struct {
	Name    string
	EnvVar  string
	Models  []string
	Default string
	New     func(apiKey, model string) Model
}

var providers = map[string]Provider{}

// Register adds p to the provider registry. Provider packages call it from
// their init function.
func Register(p Provider) {
	providers[p.Name] = p
}

// Lookup returns the provider registered under name.
func Lookup(name string) (Provider, bool) {
	p, ok := providers[name]
	return p, ok
}

// Names returns the names of all registered providers, sorted.
func Names() []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
