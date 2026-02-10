# netsert

Declarative network state assertions using gNMI.

Define expected network state in YAML, validate continuously against live devices.

## Features

- **Declarative assertions**: Define expected state, not procedures
- **gNMI-native**: Uses streaming telemetry for real-time validation
- **CI/CD ready**: Exit codes and structured output for pipeline integration
- **Containerlab friendly**: Test your assertions before production

## Installation

```bash
go install github.com/ndtobs/netsert/cmd/netsert@latest
```

## Quick Start

1. Define your assertions:

```yaml
# assertions.yaml
targets:
  - address: spine1:6030
    assertions:
      - path: /interfaces/interface[name=Ethernet1]/state/oper-status
        equals: "UP"
      - path: /network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state
        equals: "ESTABLISHED"
```

2. Run assertions:

```bash
netsert run assertions.yaml
```

3. Watch mode (continuous validation):

```bash
netsert watch assertions.yaml
```

## Assertion Types

| Type | Description |
|------|-------------|
| `equals` | Exact match |
| `contains` | Substring match |
| `matches` | Regex match |
| `exists` | Path exists (any value) |
| `absent` | Path does not exist |
| `gt`, `lt`, `gte`, `lte` | Numeric comparisons |

## Example with Containerlab

```bash
cd examples/lab
clab deploy -t topology.yaml
netsert run ../assertions.yaml
clab destroy -t topology.yaml
```

## License

MIT
