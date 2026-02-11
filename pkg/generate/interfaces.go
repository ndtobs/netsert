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
	Register(&InterfacesGenerator{})
}

// InterfacesGenerator creates assertions for interface states
type InterfacesGenerator struct{}

func (g *InterfacesGenerator) Name() string {
	return "interfaces"
}

func (g *InterfacesGenerator) Description() string {
	return "Generate assertions for interface oper-status"
}

type interfaceState struct {
	Name        string
	OperStatus  string
	AdminStatus string
}

func (g *InterfacesGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	interfaces, err := g.getInterfaces(ctx, client, opts)
	if err != nil {
		return nil, err
	}

	var assertions []assertion.Assertion
	for _, iface := range interfaces {
		// Skip interfaces that are admin down
		if iface.AdminStatus == "DOWN" {
			continue
		}

		// Skip management and internal interfaces
		if g.isSkippedInterface(iface.Name) {
			continue
		}

		name := fmt.Sprintf("%s is %s", iface.Name, iface.OperStatus)
		// Use short path format - will be expanded at load time
		path := fmt.Sprintf("interface[%s]/state/oper-status", iface.Name)

		assertions = append(assertions, assertion.Assertion{
			Name:   name,
			Path:   path,
			Equals: strPtr(iface.OperStatus),
		})
	}

	return assertions, nil
}

func (g *InterfacesGenerator) getInterfaces(ctx context.Context, client *gnmiclient.Client, opts Options) ([]interfaceState, error) {
	// Query /interfaces to get all interfaces
	path := "/interfaces"

	value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil, nil
		}
		return nil, fmt.Errorf("query interfaces: %w", err)
	}

	if !exists || value == "" {
		return nil, nil
	}

	return g.parseInterfaces(value)
}

func (g *InterfacesGenerator) parseInterfaces(jsonData string) ([]interfaceState, error) {
	var interfaces []interfaceState

	// Try OpenConfig format: {"openconfig-interfaces:interface": [...]}
	var ocResponse struct {
		Interface []struct {
			Name  string `json:"name"`
			State struct {
				Name        string `json:"name"`
				OperStatus  string `json:"oper-status"`
				AdminStatus string `json:"admin-status"`
			} `json:"state"`
		} `json:"openconfig-interfaces:interface"`
	}

	if err := json.Unmarshal([]byte(jsonData), &ocResponse); err == nil && len(ocResponse.Interface) > 0 {
		for _, i := range ocResponse.Interface {
			// Use name from the interface object or from state
			name := i.Name
			if name == "" {
				name = i.State.Name
			}
			interfaces = append(interfaces, interfaceState{
				Name:        name,
				OperStatus:  i.State.OperStatus,
				AdminStatus: i.State.AdminStatus,
			})
		}
		return interfaces, nil
	}

	// Try generic format without prefix
	var genericResponse struct {
		Interface []struct {
			Name  string `json:"name"`
			State struct {
				OperStatus  string `json:"oper-status"`
				AdminStatus string `json:"admin-status"`
			} `json:"state"`
		} `json:"interface"`
	}

	if err := json.Unmarshal([]byte(jsonData), &genericResponse); err == nil && len(genericResponse.Interface) > 0 {
		for _, i := range genericResponse.Interface {
			interfaces = append(interfaces, interfaceState{
				Name:        i.Name,
				OperStatus:  i.State.OperStatus,
				AdminStatus: i.State.AdminStatus,
			})
		}
	}

	return interfaces, nil
}

// isSkippedInterface returns true for interfaces we typically don't monitor
func (g *InterfacesGenerator) isSkippedInterface(name string) bool {
	// Skip common internal/management interfaces
	prefixes := []string{
		"Management",
		"Loopback",
		"Null",
		"Cpu",
		"Vxlan",
		"ma", // management on some platforms
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}
