package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ndtobs/netsert/pkg/assertion"
	"github.com/ndtobs/netsert/pkg/gnmiclient"
)

func init() {
	Register(&VXLANGenerator{})
}

// VXLANGenerator creates assertions for VXLAN/EVPN state
type VXLANGenerator struct{}

func (g *VXLANGenerator) Name() string {
	return "vxlan"
}

func (g *VXLANGenerator) Description() string {
	return "Generate assertions for VXLAN interface, VTEP, and VNI mappings"
}

// vxlanState represents VXLAN interface and EVPN state
type vxlanState struct {
	Name       string
	OperStatus string
	VTEPSource string
	UDPPort    int
	VLANVNIs   []vlanVNI
	VRFVNIs    []vrfVNI
}

type vlanVNI struct {
	VLAN int
	VNI  int
}

type vrfVNI struct {
	VRF string
	VNI int
}

func (g *VXLANGenerator) Generate(ctx context.Context, client *gnmiclient.Client, opts Options) ([]assertion.Assertion, error) {
	// Get VXLAN interface state
	vxlan, err := g.getVxlanState(ctx, client, opts)
	if err != nil {
		return nil, err
	}

	if vxlan == nil {
		return nil, nil // No VXLAN configured
	}

	var assertions []assertion.Assertion

	// Note: Arista doesn't expose oper-status for Vxlan interfaces via OpenConfig
	// so we skip that assertion and focus on config validation

	// VTEP source interface
	if vxlan.VTEPSource != "" {
		assertions = append(assertions, assertion.Assertion{
			Name:   fmt.Sprintf("VXLAN VTEP source is %s", vxlan.VTEPSource),
			Path:   fmt.Sprintf("interfaces/interface[name=%s]/arista-vxlan/state/src-ip-intf", vxlan.Name),
			Equals: strPtr(vxlan.VTEPSource),
		})
	}

	// VLAN to VNI mappings
	for _, mapping := range vxlan.VLANVNIs {
		assertions = append(assertions, assertion.Assertion{
			Name:   fmt.Sprintf("VLAN %d maps to VNI %d", mapping.VLAN, mapping.VNI),
			Path:   fmt.Sprintf("interfaces/interface[name=%s]/arista-vxlan/vlan-to-vnis/vlan-to-vni[vlan=%d]/state/vni", vxlan.Name, mapping.VLAN),
			Equals: strPtr(fmt.Sprintf("%d", mapping.VNI)),
		})
	}

	// VRF to VNI mappings (L3 VNI)
	for _, mapping := range vxlan.VRFVNIs {
		assertions = append(assertions, assertion.Assertion{
			Name:   fmt.Sprintf("VRF %s maps to L3VNI %d", mapping.VRF, mapping.VNI),
			Path:   fmt.Sprintf("interfaces/interface[name=%s]/arista-vxlan/vrf-to-vnis/vrf-to-vni[vrf=%s]/state/vni", vxlan.Name, mapping.VRF),
			Equals: strPtr(fmt.Sprintf("%d", mapping.VNI)),
		})
	}

	return assertions, nil
}

func (g *VXLANGenerator) getVxlanState(ctx context.Context, client *gnmiclient.Client, opts Options) (*vxlanState, error) {
	// Query Vxlan1 interface (standard Arista naming)
	path := "/interfaces/interface[name=Vxlan1]"

	value, exists, err := client.Get(ctx, path, opts.Username, opts.Password)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("query VXLAN interface: %w", err)
	}

	if !exists || value == "" {
		return nil, nil
	}

	return g.parseVxlanState(value)
}

func (g *VXLANGenerator) parseVxlanState(jsonData string) (*vxlanState, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("parse VXLAN JSON: %w", err)
	}

	vxlan := &vxlanState{
		Name: "Vxlan1",
	}

	// Get oper-status from OpenConfig state
	if state := getNestedMap(data, "openconfig-interfaces:state"); state != nil {
		if oper, ok := state["oper-status"].(string); ok {
			vxlan.OperStatus = oper
		}
	}

	// Get Arista VXLAN extensions
	aristaVxlan := getNestedMap(data, "arista-exp-eos-vxlan:arista-vxlan")
	if aristaVxlan == nil {
		return vxlan, nil
	}

	// Get VTEP source and UDP port from state
	if state := getNestedMap(aristaVxlan, "state"); state != nil {
		if src, ok := state["src-ip-intf"].(string); ok && src != "" {
			vxlan.VTEPSource = src
		}
		if port, ok := state["udp-port"].(float64); ok {
			vxlan.UDPPort = int(port)
		}
	}

	// Parse VLAN to VNI mappings
	if vlanVniMap := getNestedMap(aristaVxlan, "vlan-to-vnis"); vlanVniMap != nil {
		if vlanVnis, ok := vlanVniMap["vlan-to-vni"].([]interface{}); ok {
			for _, vv := range vlanVnis {
				vvMap, ok := vv.(map[string]interface{})
				if !ok {
					continue
				}

				var vlan, vni int

				// Get VLAN from top level
				if v, ok := vvMap["vlan"].(float64); ok {
					vlan = int(v)
				}

				// Get VNI from state
				if state := getNestedMap(vvMap, "state"); state != nil {
					if v, ok := state["vni"].(float64); ok {
						vni = int(v)
					}
				}

				if vlan > 0 && vni > 0 {
					vxlan.VLANVNIs = append(vxlan.VLANVNIs, vlanVNI{
						VLAN: vlan,
						VNI:  vni,
					})
				}
			}
		}
	}

	// Sort VLAN VNIs for consistent output
	sort.Slice(vxlan.VLANVNIs, func(i, j int) bool {
		return vxlan.VLANVNIs[i].VLAN < vxlan.VLANVNIs[j].VLAN
	})

	// Parse VRF to VNI mappings
	if vrfVniMap := getNestedMap(aristaVxlan, "vrf-to-vnis"); vrfVniMap != nil {
		if vrfVnis, ok := vrfVniMap["vrf-to-vni"].([]interface{}); ok {
			for _, vv := range vrfVnis {
				vvMap, ok := vv.(map[string]interface{})
				if !ok {
					continue
				}

				var vrf string
				var vni int

				// Get VRF from top level
				if v, ok := vvMap["vrf"].(string); ok {
					vrf = v
				}

				// Get VNI from state
				if state := getNestedMap(vvMap, "state"); state != nil {
					if v, ok := state["vni"].(float64); ok {
						vni = int(v)
					}
				}

				if vrf != "" && vni > 0 {
					vxlan.VRFVNIs = append(vxlan.VRFVNIs, vrfVNI{
						VRF: vrf,
						VNI: vni,
					})
				}
			}
		}
	}

	// Sort VRF VNIs for consistent output
	sort.Slice(vxlan.VRFVNIs, func(i, j int) bool {
		return vxlan.VRFVNIs[i].VRF < vxlan.VRFVNIs[j].VRF
	})

	return vxlan, nil
}

// getNestedMap safely retrieves a nested map from a parent map
func getNestedMap(data map[string]interface{}, key string) map[string]interface{} {
	if data == nil {
		return nil
	}
	if v, ok := data[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}
