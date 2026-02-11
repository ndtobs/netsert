// Package inventory provides device inventory management
package inventory

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Inventory holds device groups and defaults
type Inventory struct {
	Groups   map[string][]string `yaml:"groups"`
	Defaults Defaults            `yaml:"defaults,omitempty"`
}

// Defaults for all devices in inventory
type Defaults struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Insecure bool   `yaml:"insecure,omitempty"`
	Port     int    `yaml:"port,omitempty"`
}

// DefaultPaths are the standard locations to look for inventory files
var DefaultPaths = []string{
	"inventory.yaml",
	"inventory.yml",
	"inventory.ini",
	"inventory",
	"hosts",
	"hosts.yaml",
	"hosts.yml",
}

// Discover tries to find and load an inventory file from standard locations
func Discover() (*Inventory, error) {
	for _, path := range DefaultPaths {
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	return nil, fmt.Errorf("no inventory file found (tried: %s)", strings.Join(DefaultPaths, ", "))
}

// Standard inventory file locations (checked in order)
var defaultInventoryPaths = []string{
	"inventory.yaml",
	"inventory.yml",
	"inventory.ini",
	"inventory",
	"hosts.yaml",
	"hosts.yml",
	"hosts",
}

// Load loads inventory from a file, auto-detecting format
func Load(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read inventory: %w", err)
	}

	// Try YAML first
	inv, err := ParseYAML(data)
	if err == nil && len(inv.Groups) > 0 {
		return inv, nil
	}

	// Try INI/Ansible format
	inv, err = ParseINI(path)
	if err == nil && len(inv.Groups) > 0 {
		return inv, nil
	}

	return nil, fmt.Errorf("unable to parse inventory (tried YAML and INI)")
}

// AutoDiscover tries to find and load inventory from standard locations
func AutoDiscover() (*Inventory, string, error) {
	for _, path := range defaultInventoryPaths {
		if _, err := os.Stat(path); err == nil {
			inv, err := Load(path)
			if err == nil {
				return inv, path, nil
			}
		}
	}
	return nil, "", nil // No inventory found (not an error)
}

// ParseYAML parses YAML inventory format
func ParseYAML(data []byte) (*Inventory, error) {
	var inv Inventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		return nil, err
	}

	// Expand group references (e.g., "@spines")
	inv.expandReferences()

	return &inv, nil
}

// expandReferences expands @group references in groups
func (inv *Inventory) expandReferences() {
	maxDepth := 10 // Prevent infinite loops
	for i := 0; i < maxDepth; i++ {
		changed := false
		for name, members := range inv.Groups {
			var expanded []string
			for _, member := range members {
				if strings.HasPrefix(member, "@") {
					refName := strings.TrimPrefix(member, "@")
					if refMembers, ok := inv.Groups[refName]; ok {
						expanded = append(expanded, refMembers...)
						changed = true
						continue
					}
				}
				expanded = append(expanded, member)
			}
			inv.Groups[name] = expanded
		}
		if !changed {
			break
		}
	}
}

// ParseINI parses Ansible-style INI inventory
func ParseINI(path string) (*Inventory, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	inv := &Inventory{
		Groups: make(map[string][]string),
	}

	var currentGroup string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Group header: [groupname] or [groupname:children]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentGroup = strings.Trim(line, "[]")
			// Handle :children and :vars suffixes
			if idx := strings.Index(currentGroup, ":"); idx != -1 {
				currentGroup = currentGroup[:idx]
			}
			if _, ok := inv.Groups[currentGroup]; !ok {
				inv.Groups[currentGroup] = []string{}
			}
			continue
		}

		// Host entry
		if currentGroup != "" {
			host := parseINIHost(line)
			if host != "" {
				inv.Groups[currentGroup] = append(inv.Groups[currentGroup], host)
			}
		}
	}

	return inv, scanner.Err()
}

// parseINIHost extracts host address from an INI line
func parseINIHost(line string) string {
	// Split on whitespace
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}

	host := fields[0]

	// Look for ansible_host variable
	for _, field := range fields[1:] {
		if strings.HasPrefix(field, "ansible_host=") {
			return strings.TrimPrefix(field, "ansible_host=")
		}
	}

	return host
}

// GetGroup returns all hosts in a group
func (inv *Inventory) GetGroup(name string) ([]string, bool) {
	hosts, ok := inv.Groups[name]
	return hosts, ok
}

// GetAllHosts returns all unique hosts across all groups
func (inv *Inventory) GetAllHosts() []string {
	seen := make(map[string]bool)
	var hosts []string

	for _, members := range inv.Groups {
		for _, host := range members {
			if !seen[host] {
				seen[host] = true
				hosts = append(hosts, host)
			}
		}
	}

	return hosts
}

// ListGroups returns all group names
func (inv *Inventory) ListGroups() []string {
	names := make([]string, 0, len(inv.Groups))
	for name := range inv.Groups {
		names = append(names, name)
	}
	return names
}
