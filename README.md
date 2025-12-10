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
make deploy ROUTER_HOST=192.168.1.1 SSH_KEY=~/.ssh/openwrt
open http://192.168.1.1:8080
```

Options:
- `CONFIG_FILE=myconfig.json` - Use custom config file

## Configuration

Create `config.json` for multiple VPN interfaces:

```json
{
  "listen_addr": ":8080",
  "vpn_interfaces": [
    {"name": "vpn_amsterdam", "display_name": "Amsterdam"},
    {"name": "vpn_london", "display_name": "London"}
  ],
  "dhcp_leases_path": "/tmp/dhcp.leases",
  "ethers_path": "/etc/ethers",
  "hosts_path": "/etc/hosts",
  "friendly_names": {
    "192.168.1.198": "Ryan's iPhone"
  }
}
```

## License

MIT
