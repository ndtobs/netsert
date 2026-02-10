package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds netsert configuration
type Config struct {
	Defaults Defaults          `yaml:"defaults,omitempty"`
	Targets  map[string]Target `yaml:"targets,omitempty"`
}

// Defaults holds default settings
type Defaults struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Insecure bool   `yaml:"insecure,omitempty"`
	Timeout  string `yaml:"timeout,omitempty"`
}

// Target holds per-target settings (keyed by address or pattern)
type Target struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Insecure *bool  `yaml:"insecure,omitempty"`
}

// Load loads config from standard locations
// Priority: ./netsert.yaml > ~/.netsert.yaml > ~/.config/netsert/config.yaml
func Load() (*Config, error) {
	paths := []string{
		"netsert.yaml",
		".netsert.yaml",
	}

	// Add home directory paths
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".netsert.yaml"),
			filepath.Join(home, ".config", "netsert", "config.yaml"),
		)
	}

	for _, path := range paths {
		cfg, err := LoadFile(path)
		if err == nil {
			return cfg, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// No config file found, return empty config
	return &Config{}, nil
}

// LoadFile loads config from a specific file
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetCredentials returns username/password for a target address
// Checks target-specific config first, then defaults
func (c *Config) GetCredentials(address string) (username, password string, insecure bool) {
	// Check target-specific config
	if target, ok := c.Targets[address]; ok {
		username = target.Username
		password = target.Password
		if target.Insecure != nil {
			insecure = *target.Insecure
		}
	}

	// Fall back to defaults for empty values
	if username == "" {
		username = c.Defaults.Username
	}
	if password == "" {
		password = c.Defaults.Password
	}
	if !insecure {
		insecure = c.Defaults.Insecure
	}

	return username, password, insecure
}
