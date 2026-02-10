// Package generate provides assertion generation from live device state
package generate

import (
	"context"

	"github.com/ndtobs/netsert/pkg/assertion"
	"github.com/ndtobs/netsert/pkg/gnmiclient"
)

// Generator creates assertions from device state
type Generator interface {
	// Name returns the generator name (e.g., "bgp", "interfaces")
	Name() string

	// Description returns a human-readable description
	Description() string

	// Generate queries the device and returns assertions
	Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error)
}

// Options controls what gets generated
type Options struct {
	// Target address for output
	Target string

	// Credentials (passed through for context)
	Username string
	Password string
}

// Registry holds all available generators
var Registry = make(map[string]Generator)

// Register adds a generator to the registry
func Register(g Generator) {
	Registry[g.Name()] = g
}

// Get returns a generator by name
func Get(name string) (Generator, bool) {
	g, ok := Registry[name]
	return g, ok
}

// List returns all registered generator names
func List() []string {
	names := make([]string, 0, len(Registry))
	for name := range Registry {
		names = append(names, name)
	}
	return names
}

// GenerateFile creates a complete assertion file from multiple generators
func GenerateFile(ctx context.Context, client *gnmiclient.Client, generators []string, opts Options) (*assertion.AssertionFile, error) {
	var allAssertions []assertion.Assertion

	for _, name := range generators {
		gen, ok := Get(name)
		if !ok {
			continue
		}

		assertions, err := gen.Generate(ctx, client, opts)
		if err != nil {
			return nil, err
		}
		allAssertions = append(allAssertions, assertions...)
	}

	return &assertion.AssertionFile{
		Targets: []assertion.Target{
			{
				Host:       opts.Target,
				Assertions: allAssertions,
			},
		},
	}, nil
}
