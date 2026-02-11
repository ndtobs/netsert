package assertion

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFile loads assertions from a YAML file
func LoadFile(path string) (*AssertionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return Parse(data)
}

// Parse parses assertion YAML data
func Parse(data []byte) (*AssertionFile, error) {
	var af AssertionFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	// Validate and expand paths
	for i, target := range af.Targets {
		if target.GetHost() == "" {
			return nil, fmt.Errorf("target %d: host is required", i)
		}
		for j, assertion := range target.Assertions {
			if assertion.Path == "" {
				return nil, fmt.Errorf("target %d, assertion %d: path is required", i, j)
			}
			// Expand short paths to full OpenConfig paths
			af.Targets[i].Assertions[j].Path = ExpandPath(assertion.Path)
		}
	}

	return &af, nil
}
