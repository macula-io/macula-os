# MaculaOS - Custom Linux Distribution Plan

**Status:** In Progress (Phase 3 Complete, Phase 4 Planned)
**Created:** 2026-01-07
**Last Updated:** 2026-01-07 (Added: Mesh Roles, Setup Wizard, Branding, Tools, Updates)
**Repository:** `macula-io/macula-os`
**Based on:** k3OS (rancher/k3os fork)

---

## Executive Summary

MaculaOS is a custom lightweight Linux distribution optimized for running Macula edge nodes. Built on the foundation of k3OS, it provides:

- **Zero-touch deployment** - Boot, scan QR, done
- **Pre-installed k3s** - Kubernetes ready out of the box
- **Macula Console** - Management UI pre-configured
- **mDNS auto-discovery** - Automatic LAN node clustering
- **Immutable rootfs** - Secure, reproducible, upgradeable
- **Multi-arch support** - amd64 and arm64 (RPi, NUC, servers)

---

## 1. Why Custom OS vs Existing Solutions

### 1.1 Alternatives Considered

| Solution | Pros | Cons |
|----------|------|------|
| Ubuntu + cloud-init | Familiar, well-documented | Heavy (~2GB), complex config, mutable |
| Elemental (SUSE) | k3s-focused, immutable | Heavy dependency on Rancher ecosystem |
| Flatcar/CoreOS | Immutable, container-focused | No k3s integration, complex |
| Alpine + k3s manual | Lightweight (~100MB) | No config framework, manual everything |
| **k3OS (fork)** | Purpose-built for k3s, lightweight (~300MB), declarative config | Archived project, needs maintenance |

### 1.2 Why k3OS Fork

1. **Purpose-built**: Designed specifically for k3s from the start
2. **Lightweight**: ~300MB ISO vs 2GB+ for Ubuntu
3. **Declarative**: YAML-based config (`/maculaos/system/config.yaml`)
4. **Immutable rootfs**: Squashfs root, overlay for persistence
5. **Upgrade system**: A/B partition scheme for safe upgrades
6. **Boot modes**: Install, live, recovery built-in
7. **Existing codebase**: Multi-stage Docker build system ready

### 1.3 MaculaOS Value-Add

On top of k3OS base, we add:
- Pre-installed Macula Console container image
- mDNS responder + discovery daemon
- First-boot wizard (QR pairing flow)
- Macula-branded boot splash and UI
- Optimized k3s config for edge workloads
- Local git daemon for offline GitOps

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         MACULAOS BOOT STACK                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐            │
│  │   Bootloader │     │    Kernel    │     │    Initrd    │            │
│  │   (syslinux/ │────►│   (Linux)    │────►│  (busybox +  │            │
│  │    grub)     │     │              │     │   k3os-init) │            │
│  └──────────────┘     └──────────────┘     └──────────────┘            │
│                                                   │                     │
│                                                   ▼                     │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                      k3OS Bootstrap                              │   │
│  │  - Mount squashfs rootfs                                        │   │
│  │  - Detect boot mode (install/live/disk)                         │   │
│  │  - Parse cloud-config (/maculaos/system/config.yaml)             │   │
│  │  - Configure network, hostname, SSH                             │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                   │                     │
│                                                   ▼                     │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                         OpenRC Init                              │   │
│  │  - Start system services                                        │   │
│  │  - Launch k3s (server or agent mode)                            │   │
│  │  - Start mDNS daemon                                            │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                   │                     │
│                                                   ▼                     │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                     Macula Layer                                 │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │   │
│  │  │  k3s with   │  │   Macula    │  │   mDNS      │              │   │
│  │  │ pre-loaded  │  │  Console    │  │  Discovery  │              │   │
│  │  │   images    │  │  (Phoenix)  │  │   Daemon    │              │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘              │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 3. k3OS Build System (Existing)

k3OS uses a multi-stage Docker build orchestrated by Dapper:

```
images/
├── 00-base/          # Alpine base + build tools
├── 10-gobuild/       # Go compiler for k3os binaries
├── 20-kernel/        # Linux kernel build
├── 30-pkg/           # Package builder (syslinux, etc.)
├── 40-rootfs/        # Root filesystem assembly
├── 50-iso/           # ISO image creation
├── 60-qemu/          # QEMU test environment
├── 70-installer/     # Installer image
└── 80-tar/           # Final tarball
```

### 3.1 Build Commands

```bash
# Full build (all architectures)
make build

# Single arch build
ARCH=amd64 make build

# Quick iteration (rootfs only)
make rootfs

# Create ISO
make iso

# Run in QEMU for testing
make qemu
```

### 3.2 Key Files

| File | Purpose |
|------|---------|
| `Dockerfile.dapper` | Dapper build environment |
| `Makefile` | Build targets |
| `images/*/Dockerfile` | Stage-specific builds |
| `overlay/` | Files overlaid on rootfs |
| `scripts/` | Build and init scripts |

---

## 4. MaculaOS Modifications

### 4.1 New Build Stages

Add new stages to the existing build system:

```
images/
├── ... (existing stages)
├── 45-macula/        # NEW: Macula-specific components
│   ├── Dockerfile
│   └── assets/
│       ├── macula-console.tar   # Pre-pulled container image
│       ├── macula-branding/     # Boot splash, logos
│       └── k3s-airgap.tar       # Pre-loaded k3s images
└── 55-macula-iso/    # NEW: MaculaOS ISO customization
```

### 4.2 Overlay Additions

```
overlay/
├── etc/
│   ├── macula/
│   │   └── config.yaml.example  # Template with Macula defaults
│   ├── init.d/
│   │   ├── macula-mdns          # NEW: mDNS service
│   │   └── macula-firstboot     # NEW: First-boot wizard trigger
│   └── motd                     # Macula welcome message
├── opt/
│   └── macula/
│       ├── console/             # Console static files
│       ├── firstboot/           # First-boot wizard
│       └── scripts/             # Utility scripts
└── var/
    └── lib/
        └── rancher/
            └── k3s/
                └── agent/
                    └── images/   # Pre-loaded container images
```

### 4.3 Default Configuration

```yaml
# /var/lib/maculaos/config.yaml (MaculaOS defaults)

maculaos:
  # Hostname template - will be macula-XXXX where XXXX is from MAC
  hostname: macula-${RANDOM_SUFFIX}

  # Enable mDNS for .local resolution
  dns_nameservers:
    - 1.1.1.1
    - 8.8.8.8

  # Default labels for k3s node
  labels:
    macula.io/role: edge
    macula.io/arch: ${ARCH}

  # Modules to load
  modules:
    - br_netfilter
    - overlay

k3s:
  # Single-node server by default
  args:
    - server
    - --disable=traefik          # We use Macula ingress
    - --disable=servicelb        # Not needed for edge
    - --write-kubeconfig-mode=644
    - --data-dir=/var/lib/rancher/k3s

  # Pre-loaded images (airgap support)
  airgap: true

# Network configuration
network:
  # DHCP by default, can be overridden
  dhcp: true

# SSH access
ssh_authorized_keys: []

# Run on first boot
run_cmd:
  - /opt/macula/scripts/firstboot-check.sh
```

### 4.4 Mesh Role Configuration (NEW)

MaculaOS nodes can serve different roles in the Macula mesh. These roles are **not mutually exclusive** - a node can have multiple roles enabled.

```yaml
# /var/lib/maculaos/config.yaml - Mesh roles section

maculaos:
  mesh:
    # Mesh roles (Peer is always implicitly enabled)
    roles:
      bootstrap: false    # DHT bootstrap registry endpoint
      gateway: false      # NAT relay / API ingress gateway

    # Bootstrap peers to connect to (if not a bootstrap node itself)
    bootstrap_peers:
      - "https://boot.macula.io:443"

    # Realm identifier (reverse domain notation)
    realm: "io.macula"

    # TLS mode: "development" (self-signed) or "production" (verified)
    tls_mode: "development"
```

**Role Matrix:**

| Role | Purpose | Network Requirements | Use Case |
|------|---------|---------------------|----------|
| **Peer** | Regular mesh participant | Outbound only | Home device, edge sensor, workstation |
| **Bootstrap** | DHT entry point for new nodes | Public IP/port, well-known DNS | Cloud VM, datacenter server |
| **Gateway** | NAT relay, external API access | Public IP/port, bandwidth | Edge server with public IP |

**Role Combinations:**

```
┌─────────────────────────────────────────────────────────────┐
│  Macula Mesh Role Selector                                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Base Role (always enabled):                                │
│  ✓ Peer - Connect to mesh, provide/consume services        │
│                                                             │
│  Additional Roles (toggle):                                 │
│  [ ] Bootstrap - Serve as DHT entry point for new nodes    │
│  [ ] Gateway   - Relay traffic for NAT'd peers             │
│                                                             │
│  Common Configurations:                                     │
│  • Home device:     Peer only                              │
│  • Cloud entry:     Peer + Bootstrap                        │
│  • Edge relay:      Peer + Gateway                          │
│  • Infrastructure:  Peer + Bootstrap + Gateway              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**What Each Role Enables:**

| Role | Services Started | Ports Opened | Config Required |
|------|------------------|--------------|-----------------|
| Peer | macula-mesh | Outbound only | bootstrap_peers, realm |
| Bootstrap | macula-bootstrap | 443/tcp (QUIC) | Public DNS, TLS cert |
| Gateway | macula-gateway | 443/tcp, 80/tcp | Public IP, bandwidth limits |

**Implementation Tasks:**
- [ ] Add `mesh` section to config schema (`pkg/config/config.go`)
- [ ] Add `ApplyMeshRoles()` applicator (`pkg/cc/funcs.go`)
- [ ] Add mesh role selection to CLI wizard (`pkg/cliinstall/ask.go`)
- [ ] Add mesh role selection to firstboot web UI
- [ ] Create systemd/OpenRC services for each role
- [ ] Configure firewall rules based on roles

### 4.5 ASCII Art Branding & Fastfetch (NEW)

Display MaculaOS branding and system info at login via fastfetch with custom ASCII logo.

**ASCII Logo (hand-crafted for terminal):**

```
      ○───○
     /│   │\        __  __                  _
    ○─┼───┼─○      |  \/  | __ _  ___ _   _| | __ _
     \│   │/       | |\/| |/ _` |/ __| | | | |/ _` |
      ○───○        | |  | | (_| | (__| |_| | | (_| |
       \ /         |_|  |_|\__,_|\___|\__,_|_|\__,_|
        ○
                   MaculaOS v1.0.0 (amd64)
```

**Fastfetch Integration:**

```
      ○───○           macula@macula-a1b2
     /│   │\          ─────────────────────
    ○─┼───┼─○         OS: MaculaOS 1.0.0 amd64
     \│   │/          Kernel: 6.6.x-lts
      ○───○           Uptime: 2 hours, 15 mins
       \ /            Memory: 1.2 GiB / 8 GiB
        ○             Mesh: Connected (12 peers)
                      Role: Peer
                      Realm: io.macula
```

**Implementation:**

| File | Purpose |
|------|---------|
| `overlay/etc/macula/fastfetch.jsonc` | Fastfetch config with custom logo |
| `overlay/etc/macula/logo.txt` | ASCII art logo file |
| `overlay/etc/profile.d/macula-welcome.sh` | Run fastfetch on login |
| `overlay/sbin/macula-sysinfo` | Custom script for mesh info |

**Implementation Tasks:**
- [ ] Create ASCII art logo file
- [ ] Create fastfetch config with Macula branding
- [ ] Add custom module for mesh status (peers, role, realm)
- [ ] Add to profile.d for automatic display on login
- [ ] Update MOTD with minimal banner for non-interactive

### 4.6 Included Tools (NEW)

MaculaOS includes essential CLI tools for system administration and debugging.

**Included by Default:**

| Tool | Size | Category | Purpose |
|------|------|----------|---------|
| **vim** | ~30MB | Editor | Config file editing |
| **nano** | ~2MB | Editor | Beginner-friendly editing |
| **btop** | ~2MB | Monitor | Beautiful system monitor |
| **htop** | ~500KB | Monitor | Lightweight process viewer |
| **fastfetch** | ~1MB | Info | System info with ASCII logo |
| **tmux** | ~1MB | Terminal | Session multiplexer for SSH |
| **curl** | ~1MB | Network | HTTP client (already in Alpine) |
| **jq** | ~500KB | JSON | JSON parsing for scripts |
| **rsync** | ~500KB | Files | File synchronization |
| **git** | ~15MB | VCS | Version control |

**Total additional size:** ~55MB

**NOT Included (available via containers):**

| Tool | Size | Why Not Included |
|------|------|------------------|
| k9s | ~50MB | Large, optional K8s TUI |
| neovim | ~40MB | vim is sufficient for base |
| docker CLI | ~50MB | Use crictl or k3s instead |

**Installation in Dockerfile:**

```dockerfile
# images/00-base/Dockerfile
RUN apk add --no-cache \
    vim nano \
    btop htop \
    fastfetch \
    tmux \
    jq \
    rsync \
    git
```

### 4.7 Immutable Design & Package Management (NEW)

MaculaOS follows an **immutable infrastructure** design - the root filesystem is read-only, and updates are atomic.

**Filesystem Layout:**

```
┌─────────────────────────────────────────────────────────────┐
│  MaculaOS Filesystem Architecture                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  /                    ← squashfs (READ-ONLY)                │
│  ├── bin/             ← immutable binaries                  │
│  ├── etc/             ← overlay (tmpfs or persistent)       │
│  ├── home/            ← overlay                             │
│  ├── opt/             ← immutable (Macula tools)            │
│  ├── usr/             ← immutable                           │
│  └── var/             ← persistent data partition           │
│      └── lib/                                               │
│          ├── maculaos/   ← config, credentials              │
│          └── rancher/                                       │
│              └── k3s/    ← k3s data, containers             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Package Manager Behavior:**

| Action | Result | Persistence |
|--------|--------|-------------|
| `apk add vim` | Installs to overlay | ❌ Lost on reboot |
| `apk add vim` (with persistence) | Installs to overlay | ✅ Survives reboot |
| Deploy via k3s | Container runs | ✅ Managed by k8s |
| Custom ISO build | Baked into squashfs | ✅ Permanent |

**For users who need persistent packages:**

```yaml
# /var/lib/maculaos/config.yaml
maculaos:
  live:
    persistence: true           # Enable persistent overlay
    persistence_device: auto    # auto-detect or /dev/sda3
    persistence_size: 4G        # Size of persistence partition
```

**Recommended approach by use case:**

| Use Case | Approach |
|----------|----------|
| Quick testing | `apk add <pkg>` (temporary) |
| Development | Enable persistence overlay |
| Production software | Deploy as k3s pod/container |
| Enterprise customization | Build custom ISO |

### 4.8 Update Mechanism (NEW)

MaculaOS uses an **A/B partition scheme** for atomic, rollback-safe updates.

**Disk Partition Layout:**

```
┌──────────┬──────────┬──────────┬──────────────────────────┐
│  boot    │ rootfs-A │ rootfs-B │         data             │
│  (EFI)   │ (active) │ (standby)│      (persistent)        │
│  ~100MB  │  ~400MB  │  ~400MB  │      remaining           │
└──────────┴──────────┴──────────┴──────────────────────────┘
```

**Update Process:**

```
┌─────────────────────────────────────────────────────────────┐
│  MaculaOS Update Flow                                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Check for updates                                       │
│     └── Query GitHub Releases or self-hosted endpoint       │
│                                                             │
│  2. Download new squashfs                                   │
│     └── Stream to rootfs-B (standby partition)              │
│                                                             │
│  3. Verify integrity                                        │
│     └── SHA256 checksum + optional GPG signature            │
│                                                             │
│  4. Update bootloader                                       │
│     └── Set next_boot = B, boot_count = 0                   │
│                                                             │
│  5. Reboot                                                  │
│     └── System boots from rootfs-B                          │
│                                                             │
│  6. Health check                                            │
│     └── If boot fails 3x → automatic rollback to A          │
│                                                             │
│  7. Confirm success                                         │
│     └── Mark B as active, A becomes standby                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Update Triggers:**

```bash
# CLI - Manual
maculaos upgrade --check          # Check for updates
maculaos upgrade --apply          # Download and apply
maculaos upgrade --rollback       # Revert to previous

# CLI - Automatic (via system-upgrade-controller)
# Already built into k3OS, managed by k3s

# Console UI
Dashboard → System → Updates → Check Now
```

**Update Sources:**

| Source | Config | Use Case |
|--------|--------|----------|
| GitHub Releases | `upgrade.url: github://macula-io/macula-os` | Default |
| Self-hosted | `upgrade.url: https://updates.example.com/` | Enterprise |
| USB drive | `maculaos upgrade --from /mnt/usb/` | Air-gapped |

**Configuration:**

```yaml
# /var/lib/maculaos/config.yaml
maculaos:
  upgrade:
    channel: stable              # stable, beta, or nightly
    auto_check: true             # Check for updates on boot
    auto_apply: false            # Require manual approval
    url: github://macula-io/macula-os
```

**Rollback Safety:**

- Boot counter tracks failed boots
- After 3 consecutive failures → automatic rollback
- User can manually rollback anytime via `maculaos upgrade --rollback`
- Previous version always preserved until next successful update

### 4.9 Embedded Infrastructure Services (NEW)

MaculaOS includes built-in infrastructure services optimized for edge and offline operation.

#### 4.9.1 Local Git Server (GitOps)

For offline/air-gapped GitOps workflows without requiring internet access.

```
┌─────────────────────────────────────────────────────────────┐
│  Local GitOps Architecture                                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌────────────┐  │
│  │   Upstream   │     │  Local Git   │     │   Flux     │  │
│  │   (GitHub)   │────▶│   Server     │────▶│ Controller │  │
│  │              │sync │ (soft-serve) │watch│            │  │
│  └──────────────┘     └──────────────┘     └────────────┘  │
│                              │                     │        │
│                              ▼                     ▼        │
│                       ┌────────────┐       ┌────────────┐  │
│                       │   Local    │       │    k3s     │  │
│                       │   Repos    │       │  Workloads │  │
│                       └────────────┘       └────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

**Server Options:**

| Server | Size | Features | Recommendation |
|--------|------|----------|----------------|
| **soft-serve** | ~20MB | SSH access, TUI, lightweight | Default |
| **gitea** | ~100MB | Full web UI, issues, PRs | Optional |
| **git-daemon** | ~0 | Read-only, simplest | Minimal installs |

**Configuration:**

```yaml
maculaos:
  gitops:
    enabled: true
    server: soft-serve           # soft-serve, gitea, or git-daemon
    port: 23231                  # SSH port for soft-serve
    data_path: /var/lib/maculaos/git
    upstream_sync:
      enabled: true              # Sync from upstream when online
      url: https://github.com/org/fleet-repo
      interval: 5m               # Sync interval
```

#### 4.9.2 P2P Image Registry

Share container images between mesh nodes without central registry.

```
┌─────────────────────────────────────────────────────────────┐
│  P2P Registry with Spegel                                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌────────┐  ┌────────┐  ┌────────┐                        │
│  │ Node 1 │══│ Node 2 │══│ Node 3 │   P2P image sharing    │
│  │┌──────┐│  │┌──────┐│  │┌──────┐│                        │
│  ││Spegel││══││Spegel││══││Spegel││                        │
│  │└──────┘│  │└──────┘│  │└──────┘│                        │
│  └────────┘  └────────┘  └────────┘                        │
│       │           │           │                             │
│       └───────────┴───────────┘                             │
│                   │ (only if image not in mesh)             │
│                   ▼                                         │
│            ┌──────────────┐                                 │
│            │   Upstream   │                                 │
│            │  (ghcr.io)   │                                 │
│            └──────────────┘                                 │
│                                                             │
│  Flow:                                                      │
│  1. Node needs image → ask mesh peers first                │
│  2. If peer has it → P2P transfer (fast, local)            │
│  3. If not → pull from upstream, share with mesh           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Registry Options:**

| Registry | Size | P2P Native | Notes |
|----------|------|------------|-------|
| **Spegel** | ~10MB | Yes | k8s-native, containerd integration |
| **Zot** | ~30MB | Sync API | OCI-native, lightweight |
| **distribution** | ~20MB | No | Official Docker registry |

**Configuration:**

```yaml
maculaos:
  registry:
    enabled: true
    mode: spegel                 # spegel (P2P) or local (single-node cache)
    pull_through_cache: true     # Cache upstream pulls locally
    mirrors:
      docker.io: []              # Use mesh + upstream
      ghcr.io: []
      quay.io: []
```

#### 4.9.3 Observability Stack

Built-in monitoring and logging for edge nodes.

```
┌─────────────────────────────────────────────────────────────┐
│  Edge Observability                                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Metrics:                                                   │
│  ┌────────────────┐     ┌────────────────┐                 │
│  │ Node Exporter  │────▶│ Local Storage  │──▶ (Mesh sync)  │
│  │ (~10MB)        │     │ (Victoria)     │                 │
│  └────────────────┘     └────────────────┘                 │
│                                                             │
│  Logs:                                                      │
│  ┌────────────────┐     ┌────────────────┐                 │
│  │  Fluent-bit    │────▶│ Local Buffer   │──▶ (Forward)    │
│  │  (~15MB)       │     │                │                 │
│  └────────────────┘     └────────────────┘                 │
│                                                             │
│  Mesh Health:                                               │
│  ┌────────────────┐                                        │
│  │ macula-health  │  ← Custom health checks                │
│  │ (built-in)     │    - Mesh connectivity                 │
│  └────────────────┘    - Peer count, latency               │
│                        - Storage health                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**

```yaml
maculaos:
  observability:
    metrics:
      enabled: true
      node_exporter: true        # System metrics
      retention: 7d              # Local retention
      mesh_sync: true            # Sync to mesh aggregator
    logs:
      enabled: true
      driver: fluent-bit         # fluent-bit or vector
      retention: 3d              # Local retention
      forward_to: ""             # Optional: central aggregator
```

#### 4.9.4 Security Services

Built-in security infrastructure for edge operation.

| Service | Purpose | Size | Default |
|---------|---------|------|---------|
| **WireGuard** | Mesh VPN tunnels | ~1MB (kernel) | Enabled |
| **Local CA** | Issue node certificates | Built-in | Enabled |
| **Firewall** | iptables/nftables management | ~0 | Enabled |
| **Fail2ban** | Brute-force protection | ~5MB | Optional |

**Configuration:**

```yaml
maculaos:
  security:
    wireguard:
      enabled: true
      mesh_interface: wg-mesh    # Interface name
      port: 51820
    firewall:
      enabled: true
      default_policy: drop       # drop or accept
      allow_mesh: true           # Allow mesh traffic
      allow_ssh: true            # Allow SSH (port 22)
    local_ca:
      enabled: true
      path: /var/lib/maculaos/ca
```

#### 4.9.5 Edge-Specific Services

Services optimized for IoT/edge workloads.

| Service | Purpose | Size | Use Case |
|---------|---------|------|----------|
| **Mosquitto** | MQTT broker | ~5MB | IoT sensors, home automation |
| **SQLite** | Local database | ~1MB | App state, caching |
| **Chrony** | NTP client/server | ~2MB | Time sync (critical for mesh) |
| **Power mgmt** | Battery/solar aware | ~1MB | Mobile/solar deployments |

**Configuration:**

```yaml
maculaos:
  edge:
    mqtt:
      enabled: false             # Enable for IoT workloads
      port: 1883
      websocket_port: 9001
    time_sync:
      enabled: true
      servers:
        - pool.ntp.org
      serve_to_lan: true         # Act as NTP server for LAN devices
    power:
      enabled: false             # Enable for battery/solar
      shutdown_threshold: 10%    # Graceful shutdown at 10% battery
      wakeup_schedule: ""        # Cron for scheduled wakeup
```

#### 4.9.6 Summary: Embedded vs Container-Deployed

| Component | Embedded (in squashfs) | Container (via k3s) | Rationale |
|-----------|------------------------|---------------------|-----------|
| k3s | ✅ | - | Core orchestrator |
| Macula mesh | ✅ | - | Core networking |
| soft-serve (git) | ✅ | - | GitOps foundation |
| Spegel (registry) | ✅ | - | Image distribution |
| Fluent-bit | ✅ | - | System logging |
| Node exporter | ✅ | - | System metrics |
| WireGuard | ✅ (kernel) | - | Secure tunnels |
| Mosquitto | Optional | ✅ | IoT-specific |
| Grafana | - | ✅ | Heavy, optional UI |
| Loki | - | ✅ | Log aggregation |
| MinIO | - | ✅ | Object storage |
| Longhorn | - | ✅ | Distributed storage |
| Ollama | - | ✅ | Edge AI (large) |

**Total embedded infrastructure:** ~80MB additional (beyond base OS)

---

## 5. Component Details

### 5.1 mDNS Discovery Daemon

Enables automatic discovery of other MaculaOS nodes on the LAN.

**Implementation Options:**
1. **Avahi** - Standard mDNS/DNS-SD daemon (most compatible)
2. **mdns-repeater** - Lightweight, just repeats mDNS across interfaces
3. **Custom Go daemon** - Integrate with k3s node discovery

**Recommended: Avahi + custom service**

```bash
# /etc/avahi/services/macula.service
<?xml version="1.0" standalone='no'?>
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
<service-group>
  <name>MaculaOS Node</name>
  <service>
    <type>_macula._tcp</type>
    <port>6443</port>
    <txt-record>version=1.0.0</txt-record>
    <txt-record>role=server</txt-record>
  </service>
  <service>
    <type>_http._tcp</type>
    <port>80</port>
    <txt-record>path=/</txt-record>
  </service>
</service-group>
```

### 5.2 First-Boot Wizard

A lightweight web UI that runs on first boot (before Console is ready).

**Technology Options:**
1. **Simple Go binary** - Serve static HTML, handle pairing API
2. **BusyBox httpd + shell CGI** - Ultra-lightweight
3. **Phoenix (same as Console)** - Consistent, but heavier

**Recommended: Go binary** (~5MB, fast startup)

```
First Boot Flow:
1. User boots MaculaOS
2. Checks /var/lib/maculaos/.configured flag
3. If not configured:
   - Start firstboot server on port 80
   - Generate pairing code
   - Display: "Visit http://macula-XXXX.local or scan QR"
4. User visits page, enters Portal pairing code
5. Firstboot exchanges codes with Portal
6. Downloads: refresh token, certificates, GitOps config
7. Configures Console and Flux
8. Sets .configured flag
9. Reboots into normal mode
```

### 5.3 Pre-loaded Container Images

For airgap/offline operation, include essential images:

```bash
# Images to pre-load (/var/lib/rancher/k3s/agent/images/)
- ghcr.io/macula-io/macula-console:latest
- docker.io/rancher/mirrored-pause:3.6
- docker.io/rancher/local-path-provisioner:v0.0.24
- docker.io/coredns/coredns:1.10.1
```

### 5.4 Local Git Daemon

For Console's local GitOps repo (already implemented in Console):

```yaml
# k8s manifest for git daemon (if needed outside Console)
apiVersion: v1
kind: Pod
metadata:
  name: git-daemon
  namespace: macula-system
spec:
  containers:
  - name: git
    image: alpine/git:latest
    command: ["git", "daemon", "--verbose", "--export-all",
              "--base-path=/data", "--reuseaddr", "/data"]
    ports:
    - containerPort: 9418
    volumeMounts:
    - name: gitops
      mountPath: /data
```

---

## 6. Implementation Phases

### Phase 1: Fork and Verify Build (Week 1)

**Goal:** Get k3OS building from our fork

- [x] Fork rancher/k3os to macula-io/macula-os
- [x] Update Alpine base to latest LTS (3.20)
- [x] Update Linux kernel to latest LTS (6.6.x via Alpine linux-lts)
- [x] Complete k3os -> MaculaOS rebranding (Go code, scripts, Dockerfiles)
- [x] Verify amd64 build completes (2026-01-07)
- [ ] Verify arm64 build completes (requires QEMU user-mode emulation or native arm64)
- [x] Test boot in QEMU (2026-01-07) - PASSES
- [x] Document build process (2026-01-07)

**arm64 Build Requirements:**
The current build system doesn't support true cross-compilation. Options:

1. **Native arm64 hardware** (Recommended for production):
   - Build on Raspberry Pi 4/5, ARM server, or cloud ARM instance
   - `ARCH=arm64 make ci` works natively

2. **Docker buildx** (Future enhancement):
   - Requires modifying Dockerfiles to use `--platform linux/arm64`
   - Base images need explicit platform: `FROM --platform=linux/arm64 alpine:3.20`

Note: Setting `ARCH=arm64` on amd64 host with QEMU emulation creates artifacts
with arm64 naming but the binaries are still amd64 (base images pull host arch).

**QEMU Boot Test Results (2026-01-07):**
- ✅ Kernel loads successfully
- ✅ Init (maculaos binary) starts
- ✅ Loop device created, squashfs root mounted
- ✅ OpenRC starts all services (udev, dbus, connman, sshd, etc.)
- ✅ Login prompt displayed
- Memory requirement: 4GB minimum for 723MB initrd

**kmod Fix (2026-01-07):**
- Added kmod binary and required libraries to initrd:
  - `/lib/libz.so.1` (from `/lib/`)
  - `/lib/liblzma.so.5` (from `/usr/lib/`)
  - `/lib/libzstd.so.1` (from `/usr/lib/`)
  - `/lib/libcrypto.so.3` (from `/lib/`)
  - `/lib/ld-musl-x86_64.so.1` (dynamic linker)
- Modified Go code to try multiple paths for modprobe (`/sbin/modprobe`, `/bin/modprobe`, `modprobe`)
- Committed in `58cf948` - "fix: add kmod and libraries to initrd for loop device support"

**Additional rebranding (2026-01-07):**
- Fixed Go code: main.go, pkg/cc/funcs.go, pkg/cli/rc/rc.go, pkg/config/read_cc.go
- Fixed Kubernetes manifests: system-upgrade-controller.yaml, macula-latest.yaml
- Updated hostname prefix: `k3os-` → `macula-`
- Updated node labels: `k3os.io/*` → `macula.io/*`

**Minor issues remaining:**
- Welcome message still says "k3OS" in some places (cosmetic)
- Some internal comments may still reference k3os (cosmetic, non-functional)

**Rebranding completed (2026-01-07):**
- Go module: `github.com/rancher/k3os` → `github.com/macula-io/macula-os`
- CLI app: `k3os` → `maculaos`
- System paths: `/k3os/system` → `/macula/system`, `/run/k3os` → `/run/macula`
- Config paths: `/var/lib/rancher/k3os` → `/var/lib/maculaos`
- Environment vars: `K3OS_*` → `MACULAOS_*`
- Docker images: `k3os-*` → `macula-*`
- Partition labels: `K3OS_STATE` → `MACULAOS_STATE`
- ISO volume label: `K3OS` → `MACULAOS`
- YAML config key: `k3os:` → `maculaos:`
- Go struct: `Macula` → `Maculaos`
- Boot params: `k3os.mode` → `macula.mode`
- Login user: `rancher` → `macula` (unchanged - short for typing)

**Build artifacts (amd64) - 2026-01-07:**
- `maculaos-amd64.iso` - 1.5GB bootable ISO
- `macula-rootfs-amd64.tar.gz` - 1.5GB root filesystem
- `macula-kernel-amd64.squashfs` - 656MB kernel squashfs
- `maculaos-initrd-amd64` - 723MB initramfs
- `maculaos-vmlinuz-amd64` - 11MB Linux kernel
- Docker image: `maculacid/maculaos:dev`

**Build fixes (2026-01-07):**
- Changed `-mod=readonly` to `-mod=vendor` in `images/20-progs/Dockerfile` to fix Go module resolution

**Files to modify:**
- `images/00-base/Dockerfile` - Alpine version
- `images/20-kernel/Dockerfile` - Kernel version
- `Makefile` - Build targets

### Phase 2: Macula Layer Integration (Week 2-3)

**Goal:** Add Macula-specific components

- [ ] Create `images/45-macula/Dockerfile`
- [x] Add Avahi/mDNS support (2026-01-07)
  - Added avahi + avahi-tools to base image
  - Created Avahi service definition: `overlay/etc/avahi/services/macula.service`
  - Created Avahi daemon config: `overlay/etc/avahi/avahi-daemon.conf`
  - Enabled avahi-daemon in boot runlevel
- [x] Create default macula config template (2026-01-07)
  - Created `overlay/etc/macula/config.yaml.example`
  - Renamed K3OS struct to Macula in Go code
  - Updated all YAML configs to use `macula:` key
- [x] Add Macula branding (boot splash, MOTD) (2026-01-07)
  - Updated `overlay/sbin/update-issue` with MaculaOS banner
- [x] Pre-load Console container image (2026-01-07)
  - Created airgap images directory structure
  - Created `scripts/download-airgap-images.sh` for image download
  - Created README with instructions for image pre-loading
- [ ] Pre-load essential k3s images (airgap) - requires CI/CD integration
- [x] Update overlay files (2026-01-07)

**Files created:**
- `overlay/etc/avahi/services/macula.service`
- `overlay/etc/avahi/avahi-daemon.conf`
- `overlay/etc/macula/config.yaml.example`
- `overlay/var/lib/rancher/k3s/agent/images/README.md`
- `scripts/download-airgap-images.sh`
- `scripts/rebrand-config-struct.sh`
- `scripts/rebrand-macula-to-maculaos.sh` - Helper for consistent naming updates

### Phase 3: First-Boot Wizard (Week 3-4)

**Goal:** Zero-touch setup experience

- [x] Create firstboot Go binary (2026-01-07)
  - `cmd/macula-firstboot/main.go` - HTTP server with pairing flow
  - `cmd/macula-firstboot/templates/index.html` - Responsive web UI
  - `cmd/macula-firstboot/static/style.css` - Placeholder for static assets
- [x] Implement pairing flow UI (2026-01-07)
  - Modern dark theme with gradient accents
  - Step indicator showing pairing progress
  - QR code display for local URL
  - Portal code input with auto-formatting
- [x] Generate QR codes with pairing URL (2026-01-07)
  - Using skip2/go-qrcode library
  - QR served at /qr.png endpoint
  - Also displayed in console via ASCII
- [x] Exchange codes with Portal API (2026-01-07)
  - POST to /api/console/pair
  - Returns refresh token, user name, org identity
- [x] Store credentials securely (2026-01-07)
  - Stored in /var/lib/maculaos/credentials/portal.json
  - Directory permissions 0700, file permissions 0600
- [ ] Configure Console on success (requires Console integration)
- [x] Create init script for firstboot (2026-01-07)
  - `overlay/etc/init.d/macula-firstboot` - OpenRC service
  - Added to default runlevel in boot script
- [ ] Test full pairing flow (requires Portal and built ISO)

**Files created:**
- `cmd/macula-firstboot/main.go`
- `cmd/macula-firstboot/templates/index.html`
- `cmd/macula-firstboot/static/style.css`
- `overlay/etc/init.d/macula-firstboot`

**Build system updated:**
- `images/20-progs/Dockerfile` - Added firstboot build stage
- `images/20-rootfs/Dockerfile` - Copy firstboot binary to /sbin/

**Consistent Naming Convention (2026-01-07):**

The naming convention was standardized to use `maculaos` (not `macula`) for all system identifiers
to provide clear distinction from the login user (`macula`) and business-level naming:

| Category | k3OS Original | Final MaculaOS |
|----------|---------------|----------------|
| Partition label | `K3OS_STATE` | `MACULAOS_STATE` |
| ISO volume label | `K3OS` | `MACULAOS` |
| Config directory | `/var/lib/rancher/k3os/` | `/var/lib/maculaos/` |
| YAML config key | `k3os:` | `maculaos:` |
| Go struct | `K3OS` | `Maculaos` |
| Login user | `rancher` | `macula` |

Files updated for consistent naming:
- `install.sh` - MACULAOS_STATE partition labels
- `overlay/libexec/macula/boot` - MACULAOS_STATE, /var/lib/maculaos paths
- `overlay/libexec/macula/mode` - MACULAOS_STATE labels
- `overlay/libexec/macula/mode-disk` - MACULAOS_STATE labels
- `overlay/libexec/macula/live` - MACULAOS ISO label
- `overlay/libexec/macula/mode-local` - /var/lib/maculaos paths
- `overlay/etc/init.d/macula-firstboot` - /var/lib/maculaos paths
- `overlay/etc/macula/config.yaml.example` - maculaos: config key
- `images/70-iso/Dockerfile` - MACULAOS volume id
- `images/70-iso/grub.cfg` - MACULAOS fs_label
- `cmd/macula-firstboot/main.go` - /var/lib/maculaos paths
- `pkg/config/config.go` - Maculaos struct, maculaos json tag
- `pkg/config/read.go` - Maculaos struct initialization
- `pkg/system/system.go` - DefaultLocalDir = /var/lib/maculaos
- All packer config files - maculaos: config key

Commits:
- `152f09c` - refactor: consistent maculaos naming convention
- `13a174d` - fix: update Macula → Maculaos in read.go

### Phase 4: Enhanced Setup Wizard (NEW)

**Goal:** Complete setup experience with all configuration options

The existing setup infrastructure (`pkg/cliinstall/`, `cmd/macula-firstboot/`) needs extension to support:

#### 4.1 Missing System Configuration

| Feature | Config Field | Applicator | CLI Prompt | Web UI |
|---------|--------------|------------|------------|--------|
| Keyboard Layout | `maculaos.keyboard` | `ApplyKeyboard()` | `AskKeyboard()` | Dropdown |
| Timezone | `maculaos.timezone` | `ApplyTimezone()` | `AskTimezone()` | Dropdown |
| Locale | `maculaos.locale` | `ApplyLocale()` | `AskLocale()` | Dropdown |

**Implementation Tasks:**

- [ ] Add config fields to `pkg/config/config.go`:
  ```go
  type Maculaos struct {
      // ... existing fields ...
      Keyboard string `json:"keyboard,omitempty"`  // e.g., "us", "de", "fr"
      Timezone string `json:"timezone,omitempty"`  // e.g., "Europe/Berlin"
      Locale   string `json:"locale,omitempty"`    // e.g., "en_US.UTF-8"
  }
  ```

- [ ] Implement applicators in `pkg/cc/funcs.go`:
  ```go
  func ApplyKeyboard(cfg *config.CloudConfig) error {
      // loadkeys or setup-keymap (Alpine)
  }
  func ApplyTimezone(cfg *config.CloudConfig) error {
      // ln -sf /usr/share/zoneinfo/$TZ /etc/localtime
  }
  func ApplyLocale(cfg *config.CloudConfig) error {
      // echo "LANG=$LOCALE" > /etc/locale.conf
  }
  ```

- [ ] Add CLI prompts in `pkg/cliinstall/ask.go`:
  ```go
  func AskKeyboard() (string, error)   // Show common layouts
  func AskTimezone() (string, error)   // Show region → city picker
  func AskLocale() (string, error)     // Show common locales
  ```

- [ ] Add to firstboot web UI (`cmd/macula-firstboot/`)

#### 4.2 Mesh Role Selection

- [ ] Add mesh roles to config schema (see Section 4.4)
- [ ] Add CLI prompts for mesh roles:
  ```go
  func AskMeshRoles() (*MeshConfig, error) {
      // "Will this node serve as a bootstrap entry point?" [y/N]
      // "Will this node relay traffic for NAT'd peers?" [y/N]
  }
  ```
- [ ] Add mesh role toggles to firstboot web UI
- [ ] Validate role requirements (bootstrap needs public IP warning)

#### 4.3 Boot Mode / Persistence Options

Current boot modes: `disk`, `local`, `live-server`, `live-agent`, `shell`, `install`

**Add persistence option for live mode:**

```yaml
maculaos:
  live:
    persistence: true           # Enable persistent overlay
    persistence_device: auto    # auto-detect or specify device
    persistence_size: 4G        # Size of persistence partition
```

- [ ] Add persistence config fields
- [ ] Modify live boot script to mount persistence overlay
- [ ] Add "Enable persistence?" prompt to installer
- [ ] Create persistence partition on USB stick (if space available)

#### 4.4 Files to Create/Modify

| File | Changes |
|------|---------|
| `pkg/config/config.go` | Add `Keyboard`, `Timezone`, `Locale`, `Mesh`, `Live` fields |
| `pkg/cc/funcs.go` | Add `ApplyKeyboard()`, `ApplyTimezone()`, `ApplyLocale()`, `ApplyMeshRoles()` |
| `pkg/cliinstall/ask.go` | Add `AskKeyboard()`, `AskTimezone()`, `AskLocale()`, `AskMeshRoles()` |
| `pkg/cliinstall/install.go` | Integrate new prompts into wizard flow |
| `cmd/macula-firstboot/main.go` | Add API endpoints for new config |
| `cmd/macula-firstboot/templates/` | Add settings pages |
| `overlay/libexec/macula/live` | Add persistence overlay support |

### Phase 5: Multi-Arch Builds & Testing

**Goal:** Production-ready images

- [ ] Verify amd64 ISO boots on real hardware
- [ ] Verify arm64 ISO boots on Raspberry Pi 4/5
- [ ] Test USB boot ("Macula on a stick")
- [ ] Test VM deployment (QEMU, VirtualBox)
- [ ] Create OVA/QCOW2 formats
- [ ] Set up CI/CD for image builds
- [ ] Publish images to GitHub Releases

**Files to create:**
- `.github/workflows/build.yml`
- `scripts/create-vm-images.sh`

### Phase 6: Distribution & Documentation

**Goal:** Ready for users

- [ ] Create download page
- [ ] Write installation guide
- [ ] Write troubleshooting guide
- [ ] Create demo video
- [ ] Announce to community
- [ ] Set up image distribution (S3, GitHub Releases)

---

## 7. Output Artifacts

### 7.1 Image Formats

| Format | Use Case | Size (est.) |
|--------|----------|-------------|
| ISO | USB boot, VM install | ~400MB |
| IMG | Direct SD card write (RPi) | ~400MB |
| OVA | VirtualBox/VMware import | ~500MB |
| QCOW2 | KVM/Proxmox/libvirt | ~400MB |
| TAR | Container/chroot base | ~300MB |

### 7.2 File Naming

```
macula-os-{version}-{arch}.{format}

Examples:
- macula-os-1.0.0-amd64.iso
- macula-os-1.0.0-arm64.img
- macula-os-1.0.0-amd64.ova
- macula-os-1.0.0-amd64.qcow2
```

---

## 8. User Experience Flow

### 8.1 "Macula on a Stick" Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    MACULA ON A STICK - USER FLOW                        │
└─────────────────────────────────────────────────────────────────────────┘

1. User downloads MaculaOS ISO
   └── https://get.macula.io/downloads

2. User writes to USB stick
   └── balenaEtcher, dd, Rufus, etc.

3. User boots target hardware from USB
   └── Press F12/Del for boot menu

4. MaculaOS boots, shows welcome screen
   ┌─────────────────────────────────────────┐
   │                                         │
   │   ╔═══════════════════════════════╗    │
   │   ║      Welcome to MaculaOS      ║    │
   │   ╚═══════════════════════════════╝    │
   │                                         │
   │   Scan QR code or visit:               │
   │                                         │
   │   http://macula-a1b2.local             │
   │                                         │
   │   [QR CODE HERE]                        │
   │                                         │
   │   Pairing Code: ABC-123                │
   │                                         │
   └─────────────────────────────────────────┘

5. User scans QR → Opens Portal on phone

6. Portal shows "Authorize this node?"
   └── User clicks "Authorize"

7. Node receives credentials, configures itself

8. Node reboots into production mode

9. User accesses Console at http://macula-a1b2.local
   └── Dashboard shows green "Connected to Mesh"
```

### 8.2 Installation to Disk (Optional)

```
# From live boot, user can install to disk
sudo macula-install /dev/sda

# Or via Console UI
Dashboard → System → Install to Disk
```

---

## 9. Security Considerations

### 9.1 Immutable Root

- Root filesystem is read-only squashfs
- Changes via overlay (tmpfs or persistent)
- Upgrades replace entire squashfs

### 9.2 Secure Boot

- Sign kernel and initrd (future)
- Verify squashfs signature before mount
- TPM integration (future)

### 9.3 Secrets Management

- Pairing credentials encrypted at rest
- Use k3s secrets for sensitive data
- No plaintext passwords in config

### 9.4 Network Security

- Firewall enabled by default (iptables)
- Only required ports open:
  - 80/443 (Console HTTP/HTTPS)
  - 6443 (k3s API)
  - 9418 (Git daemon, local only)
  - 5353 (mDNS)
  - 10250 (kubelet)

---

## 10. Maintenance & Upgrades

### 10.1 Upgrade Strategy

```
Current: rootfs-A (active)
         rootfs-B (standby)

Upgrade Process:
1. Download new squashfs to rootfs-B
2. Verify signature
3. Update bootloader to boot from B
4. Reboot
5. If fails, automatic rollback to A
```

### 10.2 Version Management

- SemVer for MaculaOS releases
- Changelog in GitHub releases
- Upgrade notifications in Console

---

## 11. Success Criteria

- [ ] Boot to Console in < 60 seconds
- [ ] Pairing completes in < 5 minutes (user time)
- [ ] Works on Raspberry Pi 4/5, Intel NUC, generic x86
- [ ] Survives power loss (no corruption)
- [ ] mDNS discovery finds other nodes in < 30 seconds
- [ ] Airgap operation works (offline)
- [ ] ISO size < 500MB

---

## 12. Open Questions

1. **Kernel version**: Use Alpine's kernel or build custom?
2. **Init system**: Keep OpenRC or switch to systemd?
3. **Installer UI**: Text-based or graphical?
4. **Secure boot**: Worth the complexity for v1?
5. **ARM32**: Support armv7 (older RPi) or arm64 only?

---

## 13. References

- [k3OS Source](https://github.com/rancher/k3os) (archived)
- [k3s Documentation](https://docs.k3s.io/)
- [Alpine Linux Wiki](https://wiki.alpinelinux.org/)
- [Avahi Documentation](https://avahi.org/)
- [Squashfs Tools](https://github.com/plougher/squashfs-tools)
