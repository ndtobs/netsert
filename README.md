# netsert

Declarative network state assertions using gNMI.

```yaml
targets:
  - address: spine1:6030
    assertions:
      - name: Ethernet1 is UP
        path: /interfaces/interface[name=Ethernet1]/state/oper-status
        equals: "UP"
```

```bash
$ netsert run assertions.yaml

✓ [PASS] Ethernet1 is UP @ spine1:6030
```

## Features

- **Declarative** — Define expected state, not procedures
- **Fast** — gNMI over gRPC, not CLI scraping  
- **CI/CD ready** — JSON output, exit codes
- **Generate from live state** — Bootstrap assertions from devices

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
# 1. Set up credentials
cp examples/netsert.yaml .

# 2. Generate assertions from a live device
./netsert generate spine1:6030 -f baseline.yaml

# 3. Run assertions
./netsert run baseline.yaml
```

## Commands

| Command | Description |
|---------|-------------|
| `run` | Execute assertions against devices |
| `generate` | Create assertions from live device state |
| `get` | Query a single gNMI path |
| `validate` | Check assertion file syntax |

## Assertion Types

| Type | Example |
|------|---------|
| `equals` | `equals: "UP"` |
| `contains` | `contains: "Ethernet"` |
| `matches` | `matches: "^(UP\|DOWN)$"` |
| `exists` | `exists: true` |
| `gt`, `lt`, `gte`, `lte` | `gt: "100"` |

## Documentation

- **[Tutorial](TUTORIAL.md)** — Hands-on walkthrough with Containerlab
- **[Examples](examples/)** — Sample assertion files and lab topology

## Supported Platforms

Tested with Arista cEOS. Should work with any gNMI-enabled device (Nokia SR Linux, Cisco IOS-XR, Juniper).

## License

MIT
