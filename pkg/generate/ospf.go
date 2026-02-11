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
	Register(&OSPFGenerator{})
}

// OSPFGenerator creates assertions for OSPF neighbor states
type OSPFGenerator struct{}

func (g *OSPFGenerator) Name() string {
	return "ospf"
}

func (g *OSPFGenerator) Description() string {
	return "Generate assertions for OSPF neighbor states"
}

type ospfNeighbor struct {
	NeighborID string
	State      string
	Area       string
	Interface  string
}

func (g *OSPFGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	neighbors, err := g.getNeighbors(ctx, client, opts)
	if err != nil {
		return nil, err
	}

	var assertions []assertion.Assertion
	for _, n := range neighbors {
		name := fmt.Sprintf("OSPF neighbor %s is %s", n.NeighborID, n.State)

		// Use short path format
		path := fmt.Sprintf("ospf[default]/areas/area[identifier=%s]/interfaces/interface[id=%s]/neighbors/neighbor[neighbor-id=%s]/state/adjacency-state",
			n.Area, n.Interface, n.NeighborID)

		assertions = append(assertions, assertion.Assertion{
			Name:   name,
			Path:   path,
			Equals: strPtr(n.State),
		})
	}

	return assertions, nil
}

func (g *OSPFGenerator) getNeighbors(ctx context.Context, client *gnmiclient.Client, opts Options) ([]ospfNeighbor, error) {
	// Query OSPF areas to find neighbors
	path := "/network-instances/network-instance[name=default]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf/areas"

	value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
	if err != nil {
		// OSPF might not be configured - that's okay, return empty
		if strings.Contains(err.Error(), "NotFound") || 
		   strings.Contains(err.Error(), "not found") ||
		   strings.Contains(err.Error(), "path invalid") ||
		   strings.Contains(err.Error(), "InvalidArgument") {
			return nil, nil
		}
		return nil, fmt.Errorf("query OSPF areas: %w", err)
	}

	if !exists || value == "" {
		return nil, nil
	}

	return g.parseNeighbors(value)
}

func (g *OSPFGenerator) parseNeighbors(jsonData string) ([]ospfNeighbor, error) {
	var neighbors []ospfNeighbor

	// Try OpenConfig format
	var ocResponse struct {
		Area []struct {
			Identifier string `json:"identifier"`
			Interfaces struct {
				Interface []struct {
					ID        string `json:"id"`
					Neighbors struct {
						Neighbor []struct {
							NeighborID string `json:"neighbor-id"`
							State      struct {
								NeighborID     string `json:"neighbor-id"`
								AdjacencyState string `json:"adjacency-state"`
							} `json:"state"`
						} `json:"neighbor"`
					} `json:"neighbors"`
				} `json:"interface"`
			} `json:"interfaces"`
		} `json:"openconfig-network-instance:area"`
	}

	if err := json.Unmarshal([]byte(jsonData), &ocResponse); err == nil && len(ocResponse.Area) > 0 {
		for _, area := range ocResponse.Area {
			for _, iface := range area.Interfaces.Interface {
				for _, n := range iface.Neighbors.Neighbor {
					if n.State.AdjacencyState != "" {
						neighbors = append(neighbors, ospfNeighbor{
							NeighborID: n.State.NeighborID,
							State:      n.State.AdjacencyState,
							Area:       area.Identifier,
							Interface:  iface.ID,
						})
					}
				}
			}
		}
		return neighbors, nil
	}

	// Try generic format without prefix
	var genericResponse struct {
		Area []struct {
			Identifier string `json:"identifier"`
			Interfaces struct {
				Interface []struct {
					ID        string `json:"id"`
					Neighbors struct {
						Neighbor []struct {
							NeighborID string `json:"neighbor-id"`
							State      struct {
								NeighborID     string `json:"neighbor-id"`
								AdjacencyState string `json:"adjacency-state"`
							} `json:"state"`
						} `json:"neighbor"`
					} `json:"neighbors"`
				} `json:"interface"`
			} `json:"interfaces"`
		} `json:"area"`
	}

	if err := json.Unmarshal([]byte(jsonData), &genericResponse); err == nil && len(genericResponse.Area) > 0 {
		for _, area := range genericResponse.Area {
			for _, iface := range area.Interfaces.Interface {
				for _, n := range iface.Neighbors.Neighbor {
					if n.State.AdjacencyState != "" {
						neighbors = append(neighbors, ospfNeighbor{
							NeighborID: n.State.NeighborID,
							State:      n.State.AdjacencyState,
							Area:       area.Identifier,
							Interface:  iface.ID,
						})
					}
				}
			}
		}
	}

	return neighbors, nil
}
