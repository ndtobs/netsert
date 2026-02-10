package gnmiclient

import (
	"reflect"
	"testing"
)

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			"simple path",
			"interfaces/interface/state",
			[]string{"interfaces", "interface", "state"},
		},
		{
			"with leading slash",
			"/interfaces/interface/state",
			[]string{"interfaces", "interface", "state"},
		},
		{
			"with key",
			"interfaces/interface[name=Ethernet1]/state",
			[]string{"interfaces", "interface[name=Ethernet1]", "state"},
		},
		{
			"with multiple keys",
			"protocol[identifier=BGP][name=BGP]/neighbors",
			[]string{"protocol[identifier=BGP][name=BGP]", "neighbors"},
		},
		{
			"single element",
			"system",
			[]string{"system"},
		},
		{
			"empty path",
			"",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPath(tt.path)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePathElem(t *testing.T) {
	tests := []struct {
		name     string
		segment  string
		wantName string
		wantKeys map[string]string
		wantErr  bool
	}{
		{
			"simple",
			"interfaces",
			"interfaces",
			map[string]string{},
			false,
		},
		{
			"with one key",
			"interface[name=Ethernet1]",
			"interface",
			map[string]string{"name": "Ethernet1"},
			false,
		},
		{
			"with multiple keys",
			"protocol[identifier=BGP][name=BGP]",
			"protocol",
			map[string]string{"identifier": "BGP", "name": "BGP"},
			false,
		},
		{
			"key with special chars",
			"neighbor[neighbor-address=10.0.0.1]",
			"neighbor",
			map[string]string{"neighbor-address": "10.0.0.1"},
			false,
		},
		{
			"unclosed bracket",
			"interface[name=Ethernet1",
			"",
			nil,
			true,
		},
		{
			"missing equals",
			"interface[name]",
			"",
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem, err := parsePathElem(tt.segment)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if elem.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", elem.Name, tt.wantName)
			}
			if !reflect.DeepEqual(elem.Key, tt.wantKeys) {
				t.Errorf("Key = %v, want %v", elem.Key, tt.wantKeys)
			}
		})
	}
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantLen int
		wantErr bool
	}{
		{
			"simple path",
			"/interfaces/interface/state/oper-status",
			4,
			false,
		},
		{
			"path with keys",
			"/interfaces/interface[name=Ethernet1]/state/oper-status",
			4,
			false,
		},
		{
			"complex path",
			"/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.1]/state/session-state",
			9,
			false,
		},
		{
			"empty path",
			"",
			0,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := parsePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(path.Elem) != tt.wantLen {
				t.Errorf("got %d elements, want %d", len(path.Elem), tt.wantLen)
			}
		})
	}
}
