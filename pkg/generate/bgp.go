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
	Register(&BGPGenerator{})
}

// BGPGenerator creates assertions for BGP neighbors
type BGPGenerator struct{}

func (g *BGPGenerator) Name() string {
	return "bgp"
}

func (g *BGPGenerator) Description() string {
	return "Generate assertions for BGP neighbor states and AFI-SAFI"
}

// bgpNeighborState represents the relevant BGP neighbor state
type bgpNeighborState struct {
	NeighborAddress string
	SessionState    string
	PeerAS          uint32
	LocalAS         uint32
	PeerType        string
	AfiSafis        []afiSafiState
}

// afiSafiState represents AFI-SAFI state for a neighbor
type afiSafiState struct {
	Name   string
	Active bool
}

func (g *BGPGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	// Get neighbors with AFI-SAFI info
	neighbors, err := g.getOpenConfigNeighbors(ctx, client, opts)
	if err != nil {
		return nil, err
	}

	var assertions []assertion.Assertion
	for _, n := range neighbors {
		// Session state assertion
		name := fmt.Sprintf("BGP peer %s is %s", n.NeighborAddress, n.SessionState)
		path := fmt.Sprintf("bgp[default]/neighbors/neighbor[neighbor-address=%s]/state/session-state", n.NeighborAddress)

		assertions = append(assertions, assertion.Assertion{
			Name:   name,
			Path:   path,
			Equals: strPtr(n.SessionState),
		})

		// AFI-SAFI assertions for active address families
		for _, afi := range n.AfiSafis {
			if afi.Active {
				afiName := fmt.Sprintf("BGP peer %s AFI %s is active", n.NeighborAddress, afi.Name)
				afiPath := fmt.Sprintf("bgp[default]/neighbors/neighbor[neighbor-address=%s]/afi-safis/afi-safi[afi-safi-name=%s]/state/active", n.NeighborAddress, afi.Name)

				assertions = append(assertions, assertion.Assertion{
					Name:   afiName,
					Path:   afiPath,
					Equals: strPtr("true"),
				})
			}
		}
	}

	return assertions, nil
}

func (g *BGPGenerator) getOpenConfigNeighbors(ctx context.Context, client *gnmiclient.Client, opts Options) ([]bgpNeighborState, error) {
	// Query BGP neighbors path
	path := "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors"

	value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
	if err != nil {
		// BGP might not be configured
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("query BGP neighbors: %w", err)
	}

	if !exists || value == "" {
		return nil, nil
	}

	// Parse the JSON response
	return g.parseNeighbors(value)
}

func (g *BGPGenerator) parseNeighbors(jsonData string) ([]bgpNeighborState, error) {
	var neighbors []bgpNeighborState

	// Try parsing as OpenConfig structure with AFI-SAFI
	var ocResponse struct {
		Neighbor []struct {
			NeighborAddress string `json:"neighbor-address"`
			State           struct {
				NeighborAddress string `json:"neighbor-address"`
				SessionState    string `json:"session-state"`
				PeerAS          uint32 `json:"peer-as"`
				LocalAS         uint32 `json:"local-as"`
				PeerType        string `json:"peer-type"`
			} `json:"state"`
			AfiSafis struct {
				AfiSafi []struct {
					AfiSafiName string `json:"afi-safi-name"`
					State       struct {
						AfiSafiName string `json:"afi-safi-name"`
						Active      bool   `json:"active"`
						Enabled     bool   `json:"enabled"`
						Prefixes    struct {
							Received  uint32 `json:"received"`
							Sent      uint32 `json:"sent"`
							Installed uint32 `json:"installed"`
						} `json:"prefixes"`
					} `json:"state"`
				} `json:"afi-safi"`
			} `json:"afi-safis"`
		} `json:"openconfig-network-instance:neighbor"`
	}

	if err := json.Unmarshal([]byte(jsonData), &ocResponse); err == nil && len(ocResponse.Neighbor) > 0 {
		for _, n := range ocResponse.Neighbor {
			neighbor := bgpNeighborState{
				NeighborAddress: n.State.NeighborAddress,
				SessionState:    n.State.SessionState,
				PeerAS:          n.State.PeerAS,
				LocalAS:         n.State.LocalAS,
				PeerType:        n.State.PeerType,
			}

			// Parse AFI-SAFIs
			for _, afi := range n.AfiSafis.AfiSafi {
				afiName := normalizeAfiSafiName(afi.AfiSafiName)
				if afiName == "" {
					afiName = normalizeAfiSafiName(afi.State.AfiSafiName)
				}
				if afiName != "" {
					neighbor.AfiSafis = append(neighbor.AfiSafis, afiSafiState{
						Name:   afiName,
						Active: afi.State.Active,
					})
				}
			}

			neighbors = append(neighbors, neighbor)
		}
		return neighbors, nil
	}

	// Try generic neighbor array format
	var genericResponse struct {
		Neighbor []json.RawMessage `json:"neighbor"`
	}

	if err := json.Unmarshal([]byte(jsonData), &genericResponse); err == nil {
		for _, raw := range genericResponse.Neighbor {
			var n struct {
				NeighborAddress string `json:"neighbor-address"`
				State           struct {
					NeighborAddress string `json:"neighbor-address"`
					SessionState    string `json:"session-state"`
					PeerAS          uint32 `json:"peer-as"`
				} `json:"state"`
				AfiSafis struct {
					AfiSafi []struct {
						AfiSafiName string `json:"afi-safi-name"`
						State       struct {
							AfiSafiName string `json:"afi-safi-name"`
							Active      bool   `json:"active"`
						} `json:"state"`
					} `json:"afi-safi"`
				} `json:"afi-safis"`
			}
			if err := json.Unmarshal(raw, &n); err == nil && n.NeighborAddress != "" {
				neighbor := bgpNeighborState{
					NeighborAddress: n.NeighborAddress,
					SessionState:    n.State.SessionState,
					PeerAS:          n.State.PeerAS,
				}
				if neighbor.NeighborAddress == "" {
					neighbor.NeighborAddress = n.State.NeighborAddress
				}

				for _, afi := range n.AfiSafis.AfiSafi {
					afiName := normalizeAfiSafiName(afi.AfiSafiName)
					if afiName == "" {
						afiName = normalizeAfiSafiName(afi.State.AfiSafiName)
					}
					if afiName != "" {
						neighbor.AfiSafis = append(neighbor.AfiSafis, afiSafiState{
							Name:   afiName,
							Active: afi.State.Active,
						})
					}
				}

				neighbors = append(neighbors, neighbor)
			}
		}
	}

	return neighbors, nil
}

// normalizeAfiSafiName strips namespace prefixes and returns canonical name
func normalizeAfiSafiName(name string) string {
	// Strip common prefixes like "openconfig-bgp-types:" or "oc-bgp-types:"
	if idx := strings.LastIndex(name, ":"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

func strPtr(s string) *string {
	return &s
}
