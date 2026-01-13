# MaculaOS
[![Buy Me A Coffee](https://img.shields.io/badge/Buy%20Me%20A%20Coffee-support-yellow.svg)](https://buymeacoffee.com/beamologist)

MaculaOS is a lightweight Linux distribution optimized for running Macula edge nodes. Based on k3OS, it provides:

- **Pre-installed k3s** - Kubernetes ready out of the box
- **Macula Console** - Management UI pre-configured
- **mDNS auto-discovery** - Automatic LAN node clustering
- **Zero-touch setup** - Boot, scan QR, done
- **Immutable rootfs** - Secure, reproducible, upgradeable

## Quick Start

1. Download the ISO from [Releases](https://github.com/macula-io/macula-os/releases)
2. Write to USB or boot in VM
3. Visit `http://macula-XXXX.local` (shown on screen)
4. Enter pairing code from [macula.io](https://macula.io)
5. Done!

## Building

```bash
# Install Docker and make, then:
make build

# Build ISO only
make iso

# Test in QEMU
make qemu
```

All artifacts are output to `./dist/artifacts`.

## Architecture

- **Base**: Alpine Linux 3.20
- **Kernel**: Linux 6.6 LTS
- **Container Runtime**: k3s (containerd)
- **Init System**: OpenRC

## Configuration

MaculaOS uses a YAML configuration format. Configuration is stored at:

```
/maculaos/system/config.yaml     # System config (read-only)
/var/lib/maculaos/config.yaml    # Runtime config
```

### Sample config.yaml

```yaml
ssh_authorized_keys:
  - github:yourusername

hostname: my-macula-node

maculaos:
  dns_nameservers:
    - 1.1.1.1
    - 8.8.8.8
  password: yourpassword
  k3s_args:
    - server
    - "--disable=traefik"
```

## Documentation

- [Implementation Plan](plans/PLAN_MACULAOS.md) - Detailed architecture and roadmap
- [k3OS Configuration Reference](https://github.com/rancher/k3os#configuration-reference) - Full config options

## Default User

Login with user: `macula` (no password by default, set via config.yaml)

## License

Apache License 2.0

Based on [k3OS](https://github.com/rancher/k3os) by Rancher Labs.
