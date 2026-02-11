package assertion

import (
	"testing"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Absolute paths pass through unchanged
		{
			name:     "absolute path unchanged",
			input:    "/interfaces/interface[name=Ethernet1]/state/oper-status",
			expected: "/interfaces/interface[name=Ethernet1]/state/oper-status",
		},
		{
			name:     "full bgp path unchanged",
			input:    "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state",
			expected: "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state",
		},

		// BGP short paths
		{
			name:     "bgp default instance",
			input:    "bgp[default]/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state",
			expected: "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state",
		},
		{
			name:     "bgp custom vrf",
			input:    "bgp[customer-a]/neighbors/neighbor[neighbor-address=10.1.0.1]/state/session-state",
			expected: "/network-instances/network-instance[name=customer-a]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.1.0.1]/state/session-state",
		},
		{
			name:     "bgp global config",
			input:    "bgp[default]/global/state/router-id",
			expected: "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/global/state/router-id",
		},

		// Interface short paths
		{
			name:     "interface oper-status",
			input:    "interface[Ethernet1]/state/oper-status",
			expected: "/interfaces/interface[name=Ethernet1]/state/oper-status",
		},
		{
			name:     "interface counters",
			input:    "interface[Ethernet1]/state/counters/in-octets",
			expected: "/interfaces/interface[name=Ethernet1]/state/counters/in-octets",
		},
		{
			name:     "interface with slash in name",
			input:    "interface[GigabitEthernet0/0/1]/state/oper-status",
			expected: "/interfaces/interface[name=GigabitEthernet0/0/1]/state/oper-status",
		},

		// OSPF short paths
		{
			name:     "ospf areas",
			input:    "ospf[default]/areas/area[identifier=0.0.0.0]/state",
			expected: "/network-instances/network-instance[name=default]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf/areas/area[identifier=0.0.0.0]/state",
		},

		// ISIS short paths
		{
			name:     "isis levels",
			input:    "isis[default]/levels/level[level-number=2]/state",
			expected: "/network-instances/network-instance[name=default]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/levels/level[level-number=2]/state",
		},

		// System paths
		{
			name:     "system hostname",
			input:    "system/config/hostname",
			expected: "/system/config/hostname",
		},
		{
			name:     "system state",
			input:    "system/state/boot-time",
			expected: "/system/state/boot-time",
		},

		// LLDP paths
		{
			name:     "lldp neighbors",
			input:    "lldp/interfaces/interface[name=Ethernet1]/neighbors",
			expected: "/lldp/interfaces/interface[name=Ethernet1]/neighbors",
		},

		// Network instance generic
		{
			name:     "network-instance state",
			input:    "network-instance[mgmt]/state/type",
			expected: "/network-instances/network-instance[name=mgmt]/state/type",
		},

		// Unknown prefix gets leading slash
		{
			name:     "unknown prefix",
			input:    "acl/acl-sets/acl-set[name=test]/state",
			expected: "/acl/acl-sets/acl-set[name=test]/state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandPath(%q)\n  got:  %q\n  want: %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCompactPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// BGP paths
		{
			name:     "bgp session state",
			input:    "/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state",
			expected: "bgp[default]/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state",
		},
		{
			name:     "bgp custom vrf",
			input:    "/network-instances/network-instance[name=prod]/protocols/protocol[identifier=BGP][name=BGP]/bgp/global/state/as",
			expected: "bgp[prod]/global/state/as",
		},

		// Interface paths
		{
			name:     "interface oper-status",
			input:    "/interfaces/interface[name=Ethernet1]/state/oper-status",
			expected: "interface[Ethernet1]/state/oper-status",
		},

		// OSPF paths
		{
			name:     "ospf path",
			input:    "/network-instances/network-instance[name=default]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf/areas/area[identifier=0]/state",
			expected: "ospf[default]/areas/area[identifier=0]/state",
		},

		// System paths
		{
			name:     "system path",
			input:    "/system/config/hostname",
			expected: "system/config/hostname",
		},

		// LLDP paths
		{
			name:     "lldp path",
			input:    "/lldp/state/enabled",
			expected: "lldp/state/enabled",
		},

		// Unknown paths stay as-is
		{
			name:     "unknown path unchanged",
			input:    "/acl/acl-sets/acl-set[name=test]/state",
			expected: "/acl/acl-sets/acl-set[name=test]/state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompactPath(tt.input)
			if result != tt.expected {
				t.Errorf("CompactPath(%q)\n  got:  %q\n  want: %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that Expand(Compact(path)) == path for full paths
	fullPaths := []string{
		"/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state",
		"/interfaces/interface[name=Ethernet1]/state/oper-status",
		"/system/config/hostname",
		"/lldp/interfaces/interface[name=eth0]/neighbors",
	}

	for _, path := range fullPaths {
		t.Run(path, func(t *testing.T) {
			compacted := CompactPath(path)
			expanded := ExpandPath(compacted)
			if expanded != path {
				t.Errorf("Round trip failed:\n  original:  %q\n  compacted: %q\n  expanded:  %q", path, compacted, expanded)
			}
		})
	}
}

func TestIsShortPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/interfaces/interface[name=eth0]/state", false},
		{"interface[eth0]/state", true},
		{"bgp[default]/neighbors", true},
		{"/network-instances/network-instance[name=default]", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsShortPath(tt.path); got != tt.expected {
				t.Errorf("IsShortPath(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}
