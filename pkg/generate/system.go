package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ndtobs/netsert/pkg/assertion"
	"github.com/ndtobs/netsert/pkg/gnmiclient"
)

func init() {
	Register(&SystemGenerator{})
}

// SystemGenerator creates assertions for system state
type SystemGenerator struct{}

func (g *SystemGenerator) Name() string {
	return "system"
}

func (g *SystemGenerator) Description() string {
	return "Generate assertions for system hostname and software version"
}

func (g *SystemGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	var assertions []assertion.Assertion

	// Get hostname
	hostname, err := g.getHostname(ctx, client, opts)
	if err == nil && hostname != "" {
		assertions = append(assertions, assertion.Assertion{
			Name:   fmt.Sprintf("Hostname is %s", hostname),
			Path:   "system/state/hostname",
			Equals: strPtr(hostname),
		})
	}

	// Get software version
	version, err := g.getSoftwareVersion(ctx, client, opts)
	if err == nil && version != "" {
		assertions = append(assertions, assertion.Assertion{
			Name:   fmt.Sprintf("Software version is %s", version),
			Path:   "system/state/software-version",
			Equals: strPtr(version),
		})
	}

	return assertions, nil
}

func (g *SystemGenerator) getHostname(ctx context.Context, client *gnmiclient.Client, opts Options) (string, error) {
	// Try state path first, then config
	paths := []string{
		"/system/state/hostname",
		"/system/config/hostname",
	}

	for _, path := range paths {
		value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
		if err != nil {
			continue
		}
		if exists && value != "" {
			// Handle JSON-encoded string
			var hostname string
			if err := json.Unmarshal([]byte(value), &hostname); err == nil {
				return hostname, nil
			}
			// Return raw value if not JSON
			return strings.Trim(value, "\""), nil
		}
	}

	return "", fmt.Errorf("hostname not found")
}

func (g *SystemGenerator) getSoftwareVersion(ctx context.Context, client *gnmiclient.Client, opts Options) (string, error) {
	path := "/system/state/software-version"

	value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
	if err != nil {
		return "", err
	}
	if !exists || value == "" {
		return "", fmt.Errorf("software version not found")
	}

	// Handle JSON-encoded string
	var version string
	if err := json.Unmarshal([]byte(value), &version); err == nil {
		return version, nil
	}

	return strings.Trim(value, "\""), nil
}
