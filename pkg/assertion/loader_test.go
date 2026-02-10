package assertion

import (
	"testing"
)

func TestParse_Valid(t *testing.T) {
	yaml := `
targets:
  - address: device1:6030
    username: admin
    password: secret
    assertions:
      - name: test1
        path: /interfaces/interface[name=Ethernet1]/state/oper-status
        equals: "UP"
      - path: /system/config/hostname
        contains: "spine"
`
	af, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(af.Targets) != 1 {
		t.Errorf("got %d targets, want 1", len(af.Targets))
	}

	target := af.Targets[0]
	if target.GetHost() != "device1:6030" {
		t.Errorf("GetHost() = %v, want device1:6030", target.GetHost())
	}
	if target.Username != "admin" {
		t.Errorf("Username = %v, want admin", target.Username)
	}
	if len(target.Assertions) != 2 {
		t.Errorf("got %d assertions, want 2", len(target.Assertions))
	}

	a1 := target.Assertions[0]
	if a1.Name != "test1" {
		t.Errorf("Assertion name = %v, want test1", a1.Name)
	}
	if a1.Equals == nil || *a1.Equals != "UP" {
		t.Errorf("Assertion equals = %v, want UP", a1.Equals)
	}
}

func TestParse_MissingHost(t *testing.T) {
	yaml := `
targets:
  - username: admin
    assertions:
      - path: /test
        equals: "value"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for missing address")
	}
}

func TestParse_MissingPath(t *testing.T) {
	yaml := `
targets:
  - address: device1:6030
    assertions:
      - name: test without path
        equals: "value"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	yaml := `
this is not valid yaml: [
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParse_EmptyFile(t *testing.T) {
	af, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(af.Targets) != 0 {
		t.Errorf("got %d targets, want 0", len(af.Targets))
	}
}

func TestParse_MultipleTargets(t *testing.T) {
	yaml := `
targets:
  - address: device1:6030
    assertions:
      - path: /test1
        equals: "a"
  - address: device2:6030
    assertions:
      - path: /test2
        equals: "b"
`
	af, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(af.Targets) != 2 {
		t.Errorf("got %d targets, want 2", len(af.Targets))
	}
}
