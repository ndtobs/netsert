package assertion

import (
	"testing"
)

func ptr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func TestValidate_Equals(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		exists   bool
		want     bool
	}{
		{"exact match", "UP", "UP", true, true},
		{"mismatch", "UP", "DOWN", true, false},
		{"case sensitive", "up", "UP", true, false},
		{"empty match", "", "", true, true},
		{"not exists", "UP", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Assertion{Path: "/test", Equals: ptr(tt.expected)}
			result := a.Validate(tt.actual, tt.exists)
			if result.Passed != tt.want {
				t.Errorf("Validate() = %v, want %v", result.Passed, tt.want)
			}
		})
	}
}

func TestValidate_Contains(t *testing.T) {
	tests := []struct {
		name     string
		contains string
		actual   string
		want     bool
	}{
		{"substring found", "UP", "LINK_UP", true},
		{"exact match", "UP", "UP", true},
		{"not found", "UP", "DOWN", false},
		{"case sensitive", "up", "UP", false},
		{"empty contains empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Assertion{Path: "/test", Contains: ptr(tt.contains)}
			result := a.Validate(tt.actual, true)
			if result.Passed != tt.want {
				t.Errorf("Validate() = %v, want %v", result.Passed, tt.want)
			}
		})
	}
}

func TestValidate_Matches(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		actual  string
		want    bool
		wantErr bool
	}{
		{"simple match", "UP", "UP", true, false},
		{"regex match", "^[A-Z]+$", "UP", true, false},
		{"regex no match", "^[a-z]+$", "UP", false, false},
		{"invalid regex", "[", "", false, true},
		{"partial match", "UP", "LINKUP", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Assertion{Path: "/test", Matches: ptr(tt.pattern)}
			result := a.Validate(tt.actual, true)
			if tt.wantErr && result.Error == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && result.Passed != tt.want {
				t.Errorf("Validate() = %v, want %v", result.Passed, tt.want)
			}
		})
	}
}

func TestValidate_Exists(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
		want   bool
	}{
		{"path exists", true, true},
		{"path missing", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Assertion{Path: "/test", Exists: boolPtr(true)}
			result := a.Validate("anything", tt.exists)
			if result.Passed != tt.want {
				t.Errorf("Validate() = %v, want %v", result.Passed, tt.want)
			}
		})
	}
}

func TestValidate_Absent(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
		want   bool
	}{
		{"path exists", true, false},
		{"path missing", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Assertion{Path: "/test", Absent: boolPtr(true)}
			result := a.Validate("anything", tt.exists)
			if result.Passed != tt.want {
				t.Errorf("Validate() = %v, want %v", result.Passed, tt.want)
			}
		})
	}
}

func TestValidate_NumericComparisons(t *testing.T) {
	tests := []struct {
		name   string
		assert Assertion
		actual string
		want   bool
	}{
		{"gt pass", Assertion{Path: "/test", GT: ptr("10")}, "15", true},
		{"gt fail", Assertion{Path: "/test", GT: ptr("10")}, "5", false},
		{"gt equal fail", Assertion{Path: "/test", GT: ptr("10")}, "10", false},
		{"lt pass", Assertion{Path: "/test", LT: ptr("10")}, "5", true},
		{"lt fail", Assertion{Path: "/test", LT: ptr("10")}, "15", false},
		{"gte pass equal", Assertion{Path: "/test", GTE: ptr("10")}, "10", true},
		{"gte pass greater", Assertion{Path: "/test", GTE: ptr("10")}, "15", true},
		{"gte fail", Assertion{Path: "/test", GTE: ptr("10")}, "5", false},
		{"lte pass equal", Assertion{Path: "/test", LTE: ptr("10")}, "10", true},
		{"lte pass less", Assertion{Path: "/test", LTE: ptr("10")}, "5", true},
		{"lte fail", Assertion{Path: "/test", LTE: ptr("10")}, "15", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.assert.Validate(tt.actual, true)
			if result.Passed != tt.want {
				t.Errorf("Validate() = %v, want %v", result.Passed, tt.want)
			}
		})
	}
}

func TestValidate_NumericError(t *testing.T) {
	a := Assertion{Path: "/test", GT: ptr("10")}
	result := a.Validate("not-a-number", true)
	if result.Error == nil {
		t.Error("expected error for non-numeric value")
	}
}

func TestGetName(t *testing.T) {
	tests := []struct {
		name string
		a    Assertion
		want string
	}{
		{"with name", Assertion{Name: "My Test", Path: "/some/path"}, "My Test"},
		{"without name", Assertion{Path: "/some/path"}, "/some/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.GetName(); got != tt.want {
				t.Errorf("GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}
