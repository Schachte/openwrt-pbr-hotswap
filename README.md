<p align="center">
  <img src="assets/logo.png" alt="PBR VPN Control" width="120">
</p>

<h1 align="center">PBR VPN Control</h1>

<p align="center">
  Web UI for managing policy-based routing on OpenWRT routers
</p>

---

<p align="center">
  <img src="assets/screenshot.png" alt="PBR VPN Control UI" width="400">
</p>

## Features

- Toggle VPN routing per device
- Favorite devices for quick access
- Custom device naming
- Multiple VPN interface support

## Quick Start

```bash
# Deploy to router
make deploy ROUTER_HOST=192.168.1.1 SSH_KEY=~/.ssh/openwrt

# Install as service (auto-start on boot + auto-restart on crash)
make install-service ROUTER_HOST=192.168.1.1 SSH_KEY=~/.ssh/openwrt

open http://192.168.1.1:8080
```

Options:
- `CONFIG_FILE=config.json` - Use custom config file
- `SSH_KEY=~/.ssh/openwrt` - SSH key for router access

## Configuration

Create `config.json` for multiple VPN interfaces:

```json
{
  "listen_addr": ":8080",
  "vpn_interfaces": [
    {"name": "vpn_amsterdam", "display_name": "Amsterdam"},
    {"name": "vpn_london", "display_name": "London"}
  ],
  "default_interface": "vpn_amsterdam",
  "dhcp_leases_path": "/tmp/dhcp.leases",
  "ethers_path": "/etc/ethers",
  "hosts_path": "/etc/hosts",
  "friendly_names": {
    "192.168.1.198": "Ryan's iPhone"
  }
}
```

## Troubleshooting

### "Failed to toggle VPN routing: exit status 2"

Check the logs for detailed error output:
```bash
make logs ROUTER_HOST=192.168.1.1 SSH_KEY=~/.ssh/openwrt
```

Common causes:

**Invalid interface in policy**: If a policy was created with an invalid interface (e.g., `br-lan` instead of a VPN interface), PBR validation will fail. Find and remove bad policies:
```bash
# SSH to router and list policies
uci show pbr | grep interface

# Delete policy with invalid interface (replace N with index)
uci delete pbr.@policy[N]
uci commit pbr
/etc/init.d/pbr reload
```

**PBR service not running**:
```bash
/etc/init.d/pbr status
/etc/init.d/pbr start
```

## License

MIT
