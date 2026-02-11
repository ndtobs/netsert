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
	Register(&LLDPGenerator{})
}

// LLDPGenerator creates assertions for LLDP neighbors
type LLDPGenerator struct{}

func (g *LLDPGenerator) Name() string {
	return "lldp"
}

func (g *LLDPGenerator) Description() string {
	return "Generate assertions for LLDP neighbor relationships"
}

type lldpNeighbor struct {
	LocalInterface string
	RemoteSystem   string
	RemotePort     string
}

func (g *LLDPGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	neighbors, err := g.getNeighbors(ctx, client, opts)
	if err != nil {
		return nil, err
	}

	var assertions []assertion.Assertion
	for _, n := range neighbors {
		// Assert on remote system name
		if n.RemoteSystem != "" {
			name := fmt.Sprintf("LLDP %s connects to %s", n.LocalInterface, n.RemoteSystem)
			path := fmt.Sprintf("lldp/interfaces/interface[name=%s]/neighbors/neighbor/state/system-name", n.LocalInterface)

			assertions = append(assertions, assertion.Assertion{
				Name:     name,
				Path:     path,
				Contains: strPtr(n.RemoteSystem),
			})
		}
	}

	return assertions, nil
}

func (g *LLDPGenerator) getNeighbors(ctx context.Context, client *gnmiclient.Client, opts Options) ([]lldpNeighbor, error) {
	path := "/lldp/interfaces"

	value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("query LLDP interfaces: %w", err)
	}

	if !exists || value == "" {
		return nil, nil
	}

	return g.parseNeighbors(value)
}

func (g *LLDPGenerator) parseNeighbors(jsonData string) ([]lldpNeighbor, error) {
	var neighbors []lldpNeighbor

	// Try OpenConfig format
	var ocResponse struct {
		Interface []struct {
			Name      string `json:"name"`
			Neighbors struct {
				Neighbor []struct {
					State struct {
						SystemName string `json:"system-name"`
						PortID     string `json:"port-id"`
					} `json:"state"`
				} `json:"neighbor"`
			} `json:"neighbors"`
		} `json:"openconfig-lldp:interface"`
	}

	if err := json.Unmarshal([]byte(jsonData), &ocResponse); err == nil && len(ocResponse.Interface) > 0 {
		for _, iface := range ocResponse.Interface {
			for _, n := range iface.Neighbors.Neighbor {
				if n.State.SystemName != "" {
					neighbors = append(neighbors, lldpNeighbor{
						LocalInterface: iface.Name,
						RemoteSystem:   n.State.SystemName,
						RemotePort:     n.State.PortID,
					})
				}
			}
		}
		return neighbors, nil
	}

	// Try generic format without prefix
	var genericResponse struct {
		Interface []struct {
			Name      string `json:"name"`
			Neighbors struct {
				Neighbor []struct {
					State struct {
						SystemName string `json:"system-name"`
						PortID     string `json:"port-id"`
					} `json:"state"`
				} `json:"neighbor"`
			} `json:"neighbors"`
		} `json:"interface"`
	}

	if err := json.Unmarshal([]byte(jsonData), &genericResponse); err == nil && len(genericResponse.Interface) > 0 {
		for _, iface := range genericResponse.Interface {
			for _, n := range iface.Neighbors.Neighbor {
				if n.State.SystemName != "" {
					neighbors = append(neighbors, lldpNeighbor{
						LocalInterface: iface.Name,
						RemoteSystem:   n.State.SystemName,
						RemotePort:     n.State.PortID,
					})
				}
			}
		}
	}

	return neighbors, nil
}
