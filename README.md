<p align="center">
  <img src="assets/logo.svg" alt="netsert logo" width="300">
</p>

<p align="center">
  <strong>Define what your network <em>should</em> look like. netsert tells you if it <em>does</em>.</strong>
</p>

<p align="center">
  Network testing for the GitOps era. Write assertions as YAML, run them against live devices via gNMI, and catch misconfigurations before they become outages. Fast, declarative, and built for CI/CD pipelines.
</p>

---

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

```bash
$ netsert run assertions.yaml

✓ [PASS] Ethernet1 is UP @ spine1:6030
✓ [PASS] BGP peer established @ spine1:6030

Completed in 92ms
  Total:  2
  Passed: 2
  Failed: 0
```

## Why netsert?

- **Declarative** — Define expected state, not procedures
- **Fast** — gNMI over gRPC, not CLI scraping  
- **CI/CD ready** — JSON output, exit codes, runs headless
- **Generate from live state** — Bootstrap assertions from real devices
- **Vendor neutral** — Works with any gNMI-enabled device

## Install

```bash
go install github.com/ndtobs/netsert/cmd/netsert@latest
```

Or build from source:

```bash
git clone https://github.com/ndtobs/netsert.git
cd netsert && go build -o netsert ./cmd/netsert
```

## Quick Start

```bash
# Set up credentials
cp examples/netsert.yaml .

# Generate assertions from a live device
./netsert generate spine1:6030 -f baseline.yaml

# Run assertions
./netsert run baseline.yaml
```

## Generate Assertions from Live Devices

Don't write assertions by hand — generate them from current state:

```bash
$ netsert generate spine1:6030

targets:
  - host: spine1:6030
    assertions:
      - name: Ethernet1 is UP
        path: interface[Ethernet1]/state/oper-status
        equals: UP
      - name: BGP peer 10.0.0.2 is ESTABLISHED
        path: bgp[default]/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state
        equals: ESTABLISHED
```

netsert pulls the config and creates the assertion automatically from the device's current state.  

Available generators: `interfaces`, `bgp`, `ospf`, `lldp`, `system`

## Inventory Support

Run against device groups:

```yaml
# inventory.yaml
groups:
  spines:
    - spine1:6030
    - spine2:6030
  leafs:
    - leaf1:6030
    - leaf2:6030
  all:
    - "@spines"
    - "@leafs"
```

```bash
netsert run -i inventory.yaml assertions.yaml           # All devices
netsert run -i inventory.yaml -g spines assertions.yaml # Just spines
```

Also supports Ansible INI inventory format.

## CI/CD Integration

netsert exits with code 1 on failure — CI pipelines automatically block bad changes.

```yaml
# .gitlab-ci.yml
stages:
  - deploy
  - validate

deploy:
  stage: deploy
  script:
    - ansible-playbook push-config.yaml

validate:
  stage: validate
  script:
    - netsert run -i inventory.yaml assertions/post-deploy.yaml
    # If netsert exits 1, pipeline fails — merge blocked
```

```yaml
# GitHub Actions
- name: Validate network state
  run: netsert run -o json assertions.yaml > results.json
```

## Concurrency

netsert processes targets and assertions in parallel for speed:

```bash
netsert run -w 10 -p 5 assertions.yaml
```

| Flag | Default | Description |
|------|---------|-------------|
| `-w, --workers` | 10 | Concurrent targets (devices) |
| `-p, --parallel` | 5 | Concurrent assertions per target |

**Example:** 100 devices × 20 assertions each
- 10 devices processed at a time
- 5 assertions per device running concurrently
- Total: ~10 batches instead of 100 sequential connections

Set in config file for permanent defaults:

```yaml
# netsert.yaml
defaults:
  workers: 10
  parallel: 5
```

## Short Paths

OpenConfig paths can be verbose. netsert supports a **short path format** that expands automatically:

| Short Path | Expands To |
|------------|------------|
| `bgp[default]/neighbors/...` | `/network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/...` |
| `bgp[customer-a]/...` | `/network-instances/network-instance[name=customer-a]/protocols/protocol[identifier=BGP][name=BGP]/bgp/...` |
| `interface[Ethernet1]/...` | `/interfaces/interface[name=Ethernet1]/...` |
| `ospf[default]/...` | `/network-instances/network-instance[name=default]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf/...` |
| `isis[default]/...` | `/network-instances/network-instance[name=default]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/...` |
| `system/...` | `/system/...` |
| `lldp/...` | `/lldp/...` |
| `network-instance[mgmt]/...` | `/network-instances/network-instance[name=mgmt]/...` |

**The rule**: Paths starting with `/` are absolute (used as-is). Paths without a leading `/` are short paths that get expanded.

Full OpenConfig paths always work — short paths are optional convenience.

## Commands

| Command | Description |
|---------|-------------|
| `run` | Execute assertions |
| `generate` | Create assertions from device state |
| `get` | Query a single gNMI path |
| `validate` | Check assertion file syntax |

## Assertion Types

| Type | Example |
|------|---------|
| `equals` | `equals: "UP"` |
| `contains` | `contains: "Ethernet"` |
| `matches` | `matches: "^(UP\|DOWN)$"` |
| `exists` / `absent` | `exists: true` |
| `gt`, `lt`, `gte`, `lte` | `gt: "100"` |

## Documentation

- **[Tutorial](TUTORIAL.md)** — Hands-on walkthrough with Containerlab
- **[Examples](examples/)** — Sample files and lab topology

## Supported Platforms

Tested with Arista cEOS. Should work with any gNMI-enabled device (Nokia SR Linux, Cisco IOS-XR, Juniper).

## License

MIT
