# netsert

Declarative network state testing — validate live networks against YAML assertions via gNMI.

```
YAML Assertions → netsert run → Live Network → Pass/Fail Results
```

## Install

```bash
go install github.com/ndtobs/netsert/cmd/netsert@latest
```

## Quick Start

```bash
# Run assertions against a device
netsert run assertions.yaml --target spine1:6030 -u admin -P password -k

# Generate assertions from live state
netsert generate spine1:6030 -u admin -P password -k > baseline.yaml

# Run against inventory group
netsert run assertions.yaml -i inventory.yaml -g spine
```

## Example

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

  - host: leaf1:6030
    assertions:
      - name: VXLAN VTEP source is Loopback0
        path: interfaces/interface[name=Vxlan1]/arista-vxlan/state/src-ip-intf
        equals: Loopback0

      - name: VLAN 10 maps to VNI 10010
        path: interfaces/interface[name=Vxlan1]/arista-vxlan/vlan-to-vnis/vlan-to-vni[vlan=10]/state/vni
        equals: "10010"
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

## Try It

Use the [network-labs](https://github.com/ndtobs/network-labs) EVPN topology (requires [containerlab](https://containerlab.dev) + cEOS):

```bash
# Clone and deploy lab
git clone https://github.com/ndtobs/network-labs.git
cd network-labs/evpn-spine-leaf
sudo clab deploy -t topology.yaml

# Wait ~90s for boot, then run assertions
netsert run -i inventory.yaml assertions.yaml

# Run only against leaf switches
netsert run -i inventory.yaml -g leaf assertions.yaml

# Cleanup
sudo clab destroy -t topology.yaml
```

## Generators

Auto-generate assertions from live network state:

```bash
# Generate all assertions
netsert generate spine1:6030 -u admin -P password -k

# Generate specific types
netsert generate spine1:6030 --gen bgp,vxlan -u admin -P password -k
```

| Generator | Description |
|-----------|-------------|
| `interfaces` | Interface oper-status, IP addresses |
| `bgp` | BGP neighbor session state, AFI-SAFI |
| `ospf` | OSPF neighbor adjacencies |
| `lldp` | LLDP neighbor discovery |
| `vxlan` | VTEP source, VLAN→VNI, VRF→L3VNI mappings |
| `system` | Hostname, NTP sync status |

## Documentation

Full documentation: **[rob0t.tools/docs/netsert](https://rob0t.tools/docs/netsert/)**

- [Assertions](https://rob0t.tools/docs/netsert/assertions/) — All assertion types and path syntax
- [Generators](https://rob0t.tools/docs/netsert/generators/) — Auto-generate from live state
- [Inventory](https://rob0t.tools/docs/netsert/inventory/) — Organize devices into groups
- [CI/CD](https://rob0t.tools/docs/netsert/ci-cd/) — Pipeline integration

## Related

- **[netmodel](https://github.com/ndtobs/netmodel)** — Export network config to YAML for IaC

## License

MIT
