package assertion

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// AssertionFile is the top-level structure for assertion YAML files
type AssertionFile struct {
	Targets []Target `yaml:"targets"`
}

// Target represents a device and its assertions
type Target struct {
	Host       string      `yaml:"host,omitempty"`
	Address    string      `yaml:"address,omitempty"` // Deprecated: use host
	Username   string      `yaml:"username,omitempty"`
	Password   string      `yaml:"password,omitempty"`
	Insecure   bool        `yaml:"insecure,omitempty"`
	Assertions []Assertion `yaml:"assertions"`
}

// GetHost returns the host address (prefers host over address)
func (t *Target) GetHost() string {
	if t.Host != "" {
		return t.Host
	}
	return t.Address
}

// Assertion represents a single state assertion
type Assertion struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
	Path        string `yaml:"path"`

	// Assertion types (only one should be set)
	Equals   *string `yaml:"equals,omitempty"`
	Contains *string `yaml:"contains,omitempty"`
	Matches  *string `yaml:"matches,omitempty"`
	Exists   *bool   `yaml:"exists,omitempty"`
	Absent   *bool   `yaml:"absent,omitempty"`
	GT       *string `yaml:"gt,omitempty"`
	LT       *string `yaml:"lt,omitempty"`
	GTE      *string `yaml:"gte,omitempty"`
	LTE      *string `yaml:"lte,omitempty"`
}

// Result represents the outcome of an assertion
type Result struct {
	Target      string
	Assertion   Assertion
	Passed      bool
	ActualValue string
	Error       error
}

// Validate checks if the assertion passes for a given value
func (a *Assertion) Validate(value string, exists bool) *Result {
	result := &Result{
		Assertion:   *a,
		ActualValue: value,
		Passed:      false,
	}

	// Handle exists/absent first
	if a.Exists != nil && *a.Exists {
		result.Passed = exists
		return result
	}

	if a.Absent != nil && *a.Absent {
		result.Passed = !exists
		return result
	}

	// For all other assertions, value must exist
	if !exists {
		result.Error = fmt.Errorf("path does not exist")
		return result
	}

	// Equals
	if a.Equals != nil {
		result.Passed = value == *a.Equals
		return result
	}

	// Contains
	if a.Contains != nil {
		result.Passed = strings.Contains(value, *a.Contains)
		return result
	}

	// Matches (regex)
	if a.Matches != nil {
		re, err := regexp.Compile(*a.Matches)
		if err != nil {
			result.Error = fmt.Errorf("invalid regex: %w", err)
			return result
		}
		result.Passed = re.MatchString(value)
		return result
	}

	// Numeric comparisons
	if a.GT != nil || a.LT != nil || a.GTE != nil || a.LTE != nil {
		actualNum, err := strconv.ParseFloat(value, 64)
		if err != nil {
			result.Error = fmt.Errorf("value is not numeric: %w", err)
			return result
		}

		if a.GT != nil {
			threshold, _ := strconv.ParseFloat(*a.GT, 64)
			result.Passed = actualNum > threshold
		} else if a.LT != nil {
			threshold, _ := strconv.ParseFloat(*a.LT, 64)
			result.Passed = actualNum < threshold
		} else if a.GTE != nil {
			threshold, _ := strconv.ParseFloat(*a.GTE, 64)
			result.Passed = actualNum >= threshold
		} else if a.LTE != nil {
			threshold, _ := strconv.ParseFloat(*a.LTE, 64)
			result.Passed = actualNum <= threshold
		}
		return result
	}

	result.Error = fmt.Errorf("no assertion type specified")
	return result
}

// GetName returns a display name for the assertion
func (a *Assertion) GetName() string {
	if a.Name != "" {
		return a.Name
	}
	// Generate a name from the path
	return a.Path
}
