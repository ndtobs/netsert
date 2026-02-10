# netsert

Define what your network *should* look like. netsert tells you if it *does*.

```yaml
targets:
  - host: spine1:6030
    assertions:
      - name: Ethernet1 is UP
        path: /interfaces/interface[name=Ethernet1]/state/oper-status
        equals: "UP"

      - name: BGP peer established
        path: /network-instances/network-instance[name=default]/protocols/protocol[identifier=BGP][name=BGP]/bgp/neighbors/neighbor[neighbor-address=10.0.0.2]/state/session-state
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
        path: /interfaces/interface[name=Ethernet1]/state/oper-status
        equals: UP
      - name: BGP peer 10.0.0.2 is ESTABLISHED
        path: /network-instances/.../neighbor[neighbor-address=10.0.0.2]/state/session-state
        equals: ESTABLISHED
```

Available generators: `interfaces`, `bgp` (more coming)

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

**How it catches bad changes:**
1. Engineer pushes config change in PR
2. CI deploys to lab/staging
3. netsert runs assertions against devices
4. If BGP goes down, interface breaks, etc. → exit 1 → pipeline fails
5. Merge blocked until fix pushed

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
