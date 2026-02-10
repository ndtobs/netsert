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
	return "Generate assertions for BGP neighbor states"
}

// bgpNeighborState represents the relevant BGP neighbor state
type bgpNeighborState struct {
	NeighborAddress string `json:"neighbor-address"`
	SessionState    string `json:"session-state"`
	PeerAS          uint32 `json:"peer-as"`
	LocalAS         uint32 `json:"local-as"`
	PeerType        string `json:"peer-type"`
}

func (g *BGPGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	// Try OpenConfig path first (works on Arista, Nokia, etc.)
	neighbors, err := g.getOpenConfigNeighbors(ctx, client, opts)
	if err != nil {
		return nil, err
	}

	var assertions []assertion.Assertion
	for _, n := range neighbors {
		// Only generate assertions for established sessions
		// Users can edit the file to add assertions for non-established peers
		name := fmt.Sprintf("BGP peer %s is %s", n.NeighborAddress, n.SessionState)

		path := fmt.Sprintf("/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=%s]/state/session-state", n.NeighborAddress)

		assertions = append(assertions, assertion.Assertion{
			Name:   name,
			Path:   path,
			Equals: strPtr(n.SessionState),
		})
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
	// The response structure varies, try to handle common formats
	var neighbors []bgpNeighborState

	// Try parsing as OpenConfig structure
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
		} `json:"openconfig-network-instance:neighbor"`
	}

	if err := json.Unmarshal([]byte(jsonData), &ocResponse); err == nil && len(ocResponse.Neighbor) > 0 {
		for _, n := range ocResponse.Neighbor {
			neighbors = append(neighbors, bgpNeighborState{
				NeighborAddress: n.State.NeighborAddress,
				SessionState:    n.State.SessionState,
				PeerAS:          n.State.PeerAS,
				LocalAS:         n.State.LocalAS,
				PeerType:        n.State.PeerType,
			})
		}
		return neighbors, nil
	}

	// Try Arista-specific format
	var aristaResponse struct {
		Neighbor []struct {
			NeighborAddress string `json:"neighbor-address"`
			State           struct {
				NeighborAddress string `json:"neighbor-address"`
				SessionState    string `json:"session-state"`
				PeerAS          uint32 `json:"peer-as"`
			} `json:"state"`
		} `json:"arista-bgp:neighbor"`
	}

	if err := json.Unmarshal([]byte(jsonData), &aristaResponse); err == nil && len(aristaResponse.Neighbor) > 0 {
		for _, n := range aristaResponse.Neighbor {
			neighbors = append(neighbors, bgpNeighborState{
				NeighborAddress: n.State.NeighborAddress,
				SessionState:    n.State.SessionState,
				PeerAS:          n.State.PeerAS,
			})
		}
		return neighbors, nil
	}

	// Try generic neighbor array
	var genericResponse struct {
		Neighbor []json.RawMessage `json:"neighbor"`
	}

	if err := json.Unmarshal([]byte(jsonData), &genericResponse); err == nil {
		for _, raw := range genericResponse.Neighbor {
			var n struct {
				NeighborAddress string `json:"neighbor-address"`
				State           struct {
					SessionState string `json:"session-state"`
					PeerAS       uint32 `json:"peer-as"`
				} `json:"state"`
			}
			if err := json.Unmarshal(raw, &n); err == nil && n.NeighborAddress != "" {
				neighbors = append(neighbors, bgpNeighborState{
					NeighborAddress: n.NeighborAddress,
					SessionState:    n.State.SessionState,
					PeerAS:          n.State.PeerAS,
				})
			}
		}
	}

	return neighbors, nil
}

func strPtr(s string) *string {
	return &s
}
