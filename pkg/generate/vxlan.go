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
	Register(&VXLANGenerator{})
}

// VXLANGenerator creates assertions for VXLAN interface state
type VXLANGenerator struct{}

func (g *VXLANGenerator) Name() string {
	return "vxlan"
}

func (g *VXLANGenerator) Description() string {
	return "Generate assertions for VXLAN interface states"
}

// vxlanInterface represents VXLAN interface state
type vxlanInterface struct {
	Name       string
	OperStatus string
	AdminStatus string
}

func (g *VXLANGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	// Find VXLAN interfaces
	vxlanIntfs, err := g.getVxlanInterfaces(ctx, client, opts)
	if err != nil {
		return nil, err
	}

	var assertions []assertion.Assertion
	for _, vx := range vxlanIntfs {
		// Oper status assertion
		name := fmt.Sprintf("VXLAN interface %s is %s", vx.Name, vx.OperStatus)
		path := fmt.Sprintf("interfaces/interface[name=%s]/state/oper-status", vx.Name)

		assertions = append(assertions, assertion.Assertion{
			Name:   name,
			Path:   path,
			Equals: strPtr(vx.OperStatus),
		})
	}

	return assertions, nil
}

func (g *VXLANGenerator) getVxlanInterfaces(ctx context.Context, client *gnmiclient.Client, opts Options) ([]vxlanInterface, error) {
	// Query all interfaces and filter for VXLAN
	path := "/interfaces/interface"

	value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("query interfaces: %w", err)
	}

	if !exists || value == "" {
		return nil, nil
	}

	return g.parseVxlanInterfaces(value)
}

func (g *VXLANGenerator) parseVxlanInterfaces(jsonData string) ([]vxlanInterface, error) {
	var vxlanIntfs []vxlanInterface

	// Try OpenConfig format
	var ocResponse struct {
		Interface []struct {
			Name  string `json:"name"`
			State struct {
				Name        string `json:"name"`
				Type        string `json:"type"`
				OperStatus  string `json:"oper-status"`
				AdminStatus string `json:"admin-status"`
			} `json:"state"`
		} `json:"openconfig-interfaces:interface"`
	}

	if err := json.Unmarshal([]byte(jsonData), &ocResponse); err == nil && len(ocResponse.Interface) > 0 {
		for _, intf := range ocResponse.Interface {
			// Check if it's a VXLAN interface (by name or type)
			if isVxlanInterface(intf.Name, intf.State.Type) {
				vxlanIntfs = append(vxlanIntfs, vxlanInterface{
					Name:        intf.Name,
					OperStatus:  intf.State.OperStatus,
					AdminStatus: intf.State.AdminStatus,
				})
			}
		}
		return vxlanIntfs, nil
	}

	// Try generic format
	var genericResponse struct {
		Interface []struct {
			Name  string `json:"name"`
			State struct {
				Name        string `json:"name"`
				Type        string `json:"type"`
				OperStatus  string `json:"oper-status"`
				AdminStatus string `json:"admin-status"`
			} `json:"state"`
		} `json:"interface"`
	}

	if err := json.Unmarshal([]byte(jsonData), &genericResponse); err == nil {
		for _, intf := range genericResponse.Interface {
			if isVxlanInterface(intf.Name, intf.State.Type) {
				vxlanIntfs = append(vxlanIntfs, vxlanInterface{
					Name:        intf.Name,
					OperStatus:  intf.State.OperStatus,
					AdminStatus: intf.State.AdminStatus,
				})
			}
		}
	}

	return vxlanIntfs, nil
}

// isVxlanInterface checks if interface is a VXLAN tunnel interface
func isVxlanInterface(name, ifType string) bool {
	nameLower := strings.ToLower(name)
	typeLower := strings.ToLower(ifType)

	// Check name patterns
	if strings.HasPrefix(nameLower, "vxlan") || strings.HasPrefix(nameLower, "nve") {
		return true
	}

	// Check type
	if strings.Contains(typeLower, "vxlan") || strings.Contains(typeLower, "tunnel") {
		return true
	}

	return false
}
