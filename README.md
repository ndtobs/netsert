# netsert

Declarative network state testing — validate live networks against YAML assertions via gNMI.

## Overview

`netsert` lets you define what your network *should* look like as YAML assertions, then validates that state against live devices via gNMI. Catch misconfigurations before they become outages.

```
YAML Assertions → netsert run → Live Network → Pass/Fail Results
```

## Installation

```bash
go install github.com/ndtobs/netsert/cmd/netsert@latest
```

Or build from source:

```bash
git clone https://github.com/ndtobs/netsert
cd netsert
go build -o netsert ./cmd/netsert
```

## Quick Start

Run assertions against a device:

```bash
netsert run assertions.yaml --target spine1:6030 -u admin -P password -k
```

Generate assertions from live state:

```bash
netsert generate spine1:6030 -u admin -P password -k > baseline.yaml
```

Run against all devices in inventory:

```bash
netsert run assertions.yaml -i inventory.yaml
```

## Assertion Format

```yaml
targets:
  - host: spine1:6030
    assertions:
      - name: Ethernet1 is UP
        path: interface[Ethernet1]/state/oper-status
        equals: "UP"

      - name: BGP peer established
        path: bgp[default]/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state
        equals: "ESTABLISHED"
```

### Example Output

```bash
$ netsert run assertions.yaml

✓ [PASS] Ethernet1 is UP @ spine1:6030
✓ [PASS] BGP peer established @ spine1:6030

Completed in 92ms
  Total:  2
  Passed: 2
  Failed: 0
```

## Assertion Types

| Type | Example | Description |
|------|---------|-------------|
| `equals` | `equals: "UP"` | Exact match |
| `contains` | `contains: "Ethernet"` | Substring match |
| `matches` | `matches: "^(UP\|DOWN)$"` | Regex match |
| `exists` | `exists: true` | Path exists |
| `absent` | `absent: true` | Path does not exist |
| `gt`, `lt`, `gte`, `lte` | `gt: "100"` | Numeric comparison |

## Generators

Generate assertions from current device state:

```bash
netsert generate spine1:6030 --generators interfaces,bgp
```

| Generator | Description |
|-----------|-------------|
| `interfaces` | Interface status, IPs, descriptions |
| `bgp` | BGP neighbors and session state |
| `ospf` | OSPF neighbors and areas |
| `lldp` | LLDP neighbor relationships |
| `system` | Hostname, version, NTP |

## Inventory

Organize devices into groups:

```yaml
# inventory.yaml
groups:
  spine:
    - spine1:6030
    - spine2:6030
  leaf:
    - leaf1:6030
    - leaf2:6030
  all:
    - "@spine"
    - "@leaf"

defaults:
  username: admin
  password: admin
  insecure: true
```

Run by group:

```bash
netsert run assertions.yaml -i inventory.yaml -g spine
```

## Short Paths

OpenConfig paths can be verbose. netsert supports short paths:

| Short Path | Expands To |
|------------|------------|
| `interface[Ethernet1]/...` | `/interfaces/interface[name=Ethernet1]/...` |
| `bgp[default]/neighbors/...` | `/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/...` |
| `ospf[default]/...` | `/network-instances/network-instance[name=default]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf/...` |
| `system/...` | `/system/...` |
| `lldp/...` | `/lldp/...` |

Paths starting with `/` are absolute. Paths without `/` are short paths.

## Concurrency

```bash
netsert run -w 10 -p 5 assertions.yaml
```

| Flag | Default | Description |
|------|---------|-------------|
| `-w, --workers` | 10 | Concurrent targets (devices) |
| `-p, --parallel` | 5 | Concurrent assertions per target |

## CI/CD Integration

netsert exits with code 1 on failure:

```yaml
# GitHub Actions
- name: Validate network state
  run: netsert run -i inventory.yaml assertions.yaml

# JSON output for parsing
- run: netsert run -o json assertions.yaml > results.json
```

## CLI Reference

```
netsert run <file> [flags]

Flags:
  -t, --target string      single target (host:port)
  -i, --inventory string   inventory file for groups
  -g, --group string       run against specific group
  -u, --username string    gNMI username
  -P, --password string    gNMI password
  -k, --insecure           skip TLS verification
  -w, --workers int        concurrent targets (default 10)
  -p, --parallel int       concurrent assertions per target (default 5)
  -o, --output string      output format: text, json (default text)
      --timeout duration   gNMI timeout (default 30s)
```

## Commands

| Command | Description |
|---------|-------------|
| `run` | Execute assertions |
| `generate` | Create assertions from device state |
| `get` | Query a single gNMI path |
| `validate` | Check assertion file syntax |

## Related Tools

- **[netmodel](https://github.com/ndtobs/netmodel)** — Export network config to YAML for IaC
- **netsert** validates state, **netmodel** exports configuration

## License

MIT
