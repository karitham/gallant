package eval

import "slices"

// Provider describes a model provider and how to construct its models.
type Provider struct {
	Name string
	// KeyEnvVar names the environment variable that holds the provider's API
	// key. It is empty for providers that need no key.
	KeyEnvVar string
	// URLEnvVar names the environment variable that holds the provider's base
	// URL. It is empty for providers whose URL is fixed.
	URLEnvVar string
	Models    []string
	Default   string
	// New builds a Model from the API key and base URL read from the provider's
	// environment variables, and the selected model name. A provider ignores the
	// values it does not use.
	New func(apiKey, host, model string) Model
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
