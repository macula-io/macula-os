# MaculaOS - Custom Linux Distribution Plan

**Status:** In Progress (v1.0 Complete, v1.1 Mostly Complete)
**Created:** 2026-01-07
**Last Updated:** 2026-01-07 (v1.1: Mesh roles, health checks, backup, GitOps, P2P registry)
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

#### 4.9.5 NATS Messaging (NEW)

Embedded NATS server for non-BEAM services to integrate with the Macula mesh.

```
┌─────────────────────────────────────────────────────────────────┐
│  NATS Mesh Bridge Architecture                                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                       │
│  │ Go Svc   │  │ Rust Svc │  │ Python   │   Non-BEAM Services   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                       │
│       │             │             │                              │
│       └─────────────┼─────────────┘                              │
│                     │ nats://localhost:4222                      │
│                     ▼                                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    NATS Server                           │    │
│  │                  (embedded in OS)                        │    │
│  └─────────────────────────────────────────────────────────┘    │
│                     │                                            │
│                     │ NATS Bridge Module                         │
│                     ▼                                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                  Macula Console                          │    │
│  │               (NATS ↔ Mesh Bridge)                       │    │
│  └─────────────────────────────────────────────────────────┘    │
│                     │                                            │
│                     │ HTTP/3 QUIC                                │
│                     ▼                                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                   Macula Mesh                            │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Why NATS:**
- Native pub/sub (mirrors DHT pubsub semantics)
- Native request/reply (mirrors DHT RPC semantics)
- Tiny footprint (~20MB binary)
- Clients for every language (Go, Rust, Python, JS, Java, etc.)
- No HTTP callback servers needed
- JetStream for persistent streams (optional)

**NATS Subject Mapping:**

| NATS Subject | Mesh Operation |
|--------------|----------------|
| `macula.rpc.{realm}.{procedure}` | DHT RPC call |
| `macula.pub.{realm}.{topic}` | DHT PubSub publish |
| `macula.sub.{realm}.{pattern}` | DHT PubSub subscribe |
| `macula.discover.{realm}.{pattern}` | Service discovery |

**Configuration:**

```yaml
maculaos:
  nats:
    enabled: true               # Enable NATS server
    listen: 127.0.0.1:4222     # Localhost only (secure default)
    max_payload: 1MB
    jetstream:
      enabled: false            # Enable for persistent streams
      store_dir: /var/lib/maculaos/nats/jetstream
      max_file: 1GB
    cluster:
      enabled: false            # Enable for multi-node NATS cluster
      port: 6222
```

**Files:**
- `/bin/nats-server` - NATS server binary
- `/etc/macula/nats.conf` - Server configuration
- `/etc/init.d/nats-server` - OpenRC service

#### 4.9.6 Edge-Specific Services

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

#### 4.9.7 Summary: Embedded vs Container-Deployed

| Component | Embedded (in squashfs) | Container (via k3s) | Rationale |
|-----------|------------------------|---------------------|-----------|
| k3s | ✅ | - | Core orchestrator |
| Macula mesh | ✅ | - | Core networking |
| **NATS server** | ✅ | - | Mesh integration for non-BEAM |
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

**Total embedded infrastructure:** ~100MB additional (beyond base OS, includes NATS ~20MB)

### 4.10 Recovery & Troubleshooting (NEW)

Ensure nodes can always be recovered, even in worst-case scenarios.

#### 4.10.1 Recovery Mode

Dedicated rescue environment accessible from bootloader.

```
┌─────────────────────────────────────────────────────────────┐
│  Boot Menu                                                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. MaculaOS (normal boot)                                  │
│  2. MaculaOS (previous version)         ← Rollback          │
│  3. Recovery Mode                        ← Rescue shell     │
│  4. Factory Reset                        ← Wipe & reinstall │
│                                                             │
│  Auto-boot in 5 seconds...                                  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Recovery Mode Features:**
- Minimal BusyBox environment (runs from initrd)
- Network access for remote troubleshooting
- Mount/unmount data partitions
- Repair filesystem errors
- Reset passwords
- Restore from backup

**Implementation:**
```yaml
# Kernel cmdline for recovery
macula.mode=recovery

# Recovery services started:
- SSH server (on port 22)
- Serial console
- Network (DHCP)
```

#### 4.10.2 Factory Reset

One-command reset to clean installation state.

```bash
# From running system
maculaos factory-reset --confirm

# From recovery mode
factory-reset

# From bootloader (hold button during boot)
# Physical button support for headless devices
```

**Factory Reset Behavior:**
| Data | Action |
|------|--------|
| OS (squashfs) | Keep current version |
| `/var/lib/maculaos/` | **WIPED** (config, credentials) |
| `/var/lib/rancher/k3s/` | **WIPED** (k3s state, pods) |
| User data (`/var/lib/data/`) | Optional: keep or wipe |

#### 4.10.3 Remote Console Access

Multiple fallback paths for remote access.

```
┌─────────────────────────────────────────────────────────────┐
│  Remote Access Fallback Chain                               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. SSH (port 22)          ← Primary                        │
│     └── Key-based auth                                      │
│                                                             │
│  2. Macula Mesh RPC        ← If SSH unreachable            │
│     └── Console UI remote shell                            │
│                                                             │
│  3. Serial Console         ← Physical access               │
│     └── 115200 baud, ttyS0/ttyUSB0                         │
│                                                             │
│  4. Out-of-Band (IPMI/iLO) ← Enterprise hardware           │
│     └── Optional integration                               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 4.10.4 Built-in Diagnostics

```bash
# Network diagnostics
maculaos diag network
  ✓ Interface eth0: UP, 192.168.1.100/24
  ✓ Default gateway: 192.168.1.1 (reachable)
  ✓ DNS: 1.1.1.1 (resolving)
  ✓ Internet: google.com (reachable)
  ✓ Mesh bootstrap: boot.macula.io (connected)

# Storage diagnostics
maculaos diag storage
  ✓ Root partition: 45% used (healthy)
  ✓ Data partition: 23% used (healthy)
  ✓ SMART status: OK

# Mesh diagnostics
maculaos diag mesh
  ✓ Mesh status: Connected
  ✓ Peers: 12 (3 direct, 9 relayed)
  ✓ Realm: io.macula
  ✓ Role: Peer
  ✓ Latency to bootstrap: 23ms

# Full system report (for support)
maculaos diag --full > /tmp/support-bundle.tar.gz
```

### 4.11 Hardware Support (NEW)

Hardware compatibility and driver support.

#### 4.11.1 Supported Platforms

| Platform | Architecture | Status | Notes |
|----------|--------------|--------|-------|
| Generic x86_64 | amd64 | ✅ Supported | Primary target |
| Intel NUC | amd64 | ✅ Supported | Tested |
| Raspberry Pi 4/5 | arm64 | 🔄 Planned | Community priority |
| NVIDIA Jetson | arm64 | 🔄 Planned | AI edge |
| Generic ARM64 | arm64 | 🔄 Planned | Server-class |
| Rockchip (Pine64, etc.) | arm64 | ❓ Community | Best-effort |

#### 4.11.2 Hardware Security

| Feature | Support | Notes |
|---------|---------|-------|
| **TPM 2.0** | 🔄 Planned | Secure boot, secret storage |
| **Secure Boot** | 🔄 Planned | Signed kernel/initrd |
| **Hardware RNG** | ✅ Supported | `/dev/hwrng` if available |
| **Hardware Watchdog** | ✅ Supported | Auto-reboot on hang |

**TPM Integration (Future):**
```yaml
maculaos:
  security:
    tpm:
      enabled: true
      seal_secrets: true       # Seal secrets to TPM
      measured_boot: true      # Measure boot chain
```

#### 4.11.3 Accelerators & GPUs

| Device | Support | Use Case |
|--------|---------|----------|
| **Google Coral** | 🔄 Planned | Edge TPU for ML inference |
| **Intel Movidius** | 🔄 Planned | Neural compute stick |
| **NVIDIA GPU** | 🔄 Planned | CUDA, AI training |
| **AMD GPU** | ❓ Future | ROCm support |

**GPU Container Support (via k3s):**
```yaml
# When GPU support enabled
maculaos:
  hardware:
    gpu:
      enabled: true
      runtime: nvidia          # nvidia or intel
```

#### 4.11.4 IoT & Peripherals

| Interface | Support | Notes |
|-----------|---------|-------|
| **GPIO (RPi)** | 🔄 Planned | `/dev/gpiochip0` |
| **I2C** | 🔄 Planned | Sensor buses |
| **SPI** | 🔄 Planned | Display, peripherals |
| **USB Serial** | ✅ Supported | `/dev/ttyUSB*` |
| **Bluetooth** | 🔄 Planned | BLE for IoT |
| **Zigbee/Z-Wave** | 🔄 Planned | USB dongles |

#### 4.11.5 Networking Hardware

| Interface | Support | Notes |
|-----------|---------|-------|
| **Ethernet** | ✅ Supported | Primary |
| **WiFi** | ✅ Supported | wpa_supplicant |
| **LTE/5G Modem** | 🔄 Planned | ModemManager |
| **LoRa** | ❓ Future | IoT long-range |
| **Satellite (Starlink)** | ❓ Future | High-latency handling |

**Cellular Modem Configuration:**
```yaml
maculaos:
  network:
    cellular:
      enabled: true
      apn: "internet"
      pin: ""                  # SIM PIN if required
      failover: true           # Failover from ethernet/wifi
```

### 4.12 Fleet Management (NEW)

Managing multiple MaculaOS nodes at scale.

#### 4.12.1 Fleet Overview

```
┌─────────────────────────────────────────────────────────────┐
│  Fleet Management Architecture                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                   Macula Portal                       │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐           │  │
│  │  │  Fleet   │  │  Health  │  │  Update  │           │  │
│  │  │ Registry │  │ Monitor  │  │ Manager  │           │  │
│  │  └──────────┘  └──────────┘  └──────────┘           │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                 │
│           ┌───────────────┼───────────────┐                │
│           ▼               ▼               ▼                │
│      ┌────────┐      ┌────────┐      ┌────────┐           │
│      │ Node 1 │      │ Node 2 │      │ Node N │           │
│      │ edge-01│      │ edge-02│      │ edge-N │           │
│      └────────┘      └────────┘      └────────┘           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 4.12.2 Fleet Health Dashboard

Via Macula Console/Portal:

```
┌─────────────────────────────────────────────────────────────┐
│  Fleet Health                                    [Refresh]  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Summary: 47 nodes │ 45 healthy │ 2 degraded │ 0 offline   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ Node          │ Status │ Version │ Uptime │ CPU/Mem │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │ edge-warehouse-01 │ ✅ │ 1.2.0 │ 45d │ 12%/34% │       │
│  │ edge-warehouse-02 │ ✅ │ 1.2.0 │ 45d │ 8%/28%  │       │
│  │ edge-store-nyc-01 │ ⚠️ │ 1.1.9 │ 12d │ 89%/78% │       │
│  │ edge-store-nyc-02 │ ⚠️ │ 1.1.9 │ 12d │ 45%/56% │       │
│  │ ...                                                 │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  [Update Selected] [Restart Selected] [View Logs]          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 4.12.3 Coordinated Updates

Rolling updates across fleet with health checks.

```yaml
# Fleet update strategy
fleet:
  update:
    strategy: rolling          # rolling, blue-green, canary
    max_unavailable: 10%       # Max nodes updating at once
    health_check_wait: 60s     # Wait for health after update
    auto_rollback: true        # Rollback if health check fails

    # Canary settings (if strategy: canary)
    canary:
      percentage: 5%           # Start with 5% of fleet
      success_threshold: 95%   # Require 95% success to proceed
```

#### 4.12.4 Configuration Drift Detection

Detect and remediate when nodes diverge from desired state.

```bash
# Check for drift
maculaos fleet drift-check
  ⚠️ edge-store-nyc-01: config.yaml differs (3 keys)
  ⚠️ edge-store-nyc-02: extra package installed (htop)
  ✓ 45 nodes: no drift detected

# Remediate drift
maculaos fleet drift-fix --dry-run
maculaos fleet drift-fix --apply
```

#### 4.12.5 Remote Wipe / Decommissioning

Secure removal of nodes from fleet.

```bash
# From Portal/Console
maculaos fleet decommission edge-old-node --wipe

# Actions:
# 1. Revoke mesh credentials
# 2. Remove from fleet registry
# 3. Remote trigger factory reset
# 4. Optionally: secure wipe (multiple passes)
```

### 4.13 Data & Storage Strategy (NEW)

Data persistence, encryption, and backup strategies.

#### 4.13.1 Encryption at Rest

LUKS encryption for data partition.

```
┌─────────────────────────────────────────────────────────────┐
│  Encrypted Storage Layout                                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────────┐   │
│  │  boot    │  │ rootfs-A │  │      data (LUKS)       │   │
│  │  (clear) │  │ (clear)  │  │  ┌──────────────────┐  │   │
│  │          │  │          │  │  │  Decrypted FS    │  │   │
│  │          │  │          │  │  │  /var/lib/...    │  │   │
│  └──────────┘  └──────────┘  │  └──────────────────┘  │   │
│                              └────────────────────────┘   │
│                                                             │
│  Key Storage Options:                                       │
│  • TPM-sealed (if available)                               │
│  • Passphrase (entered at boot)                            │
│  • Network-fetched (enterprise key server)                 │
│  • USB key (physical token)                                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**
```yaml
maculaos:
  storage:
    encryption:
      enabled: true
      method: luks2            # luks2 or luks1
      key_source: tpm          # tpm, passphrase, network, usb
      cipher: aes-xts-plain64
```

#### 4.13.2 Backup & Restore

Automated backup to mesh or cloud.

```yaml
maculaos:
  backup:
    enabled: true
    schedule: "0 2 * * *"      # Daily at 2 AM
    retention: 7               # Keep 7 backups
    target: mesh               # mesh, s3, local

    # What to backup
    include:
      - /var/lib/maculaos/     # Config, credentials
      - /var/lib/data/         # User data
    exclude:
      - /var/lib/rancher/k3s/agent/containerd/  # Container layers (re-pullable)

    # Mesh backup (replicate to N peers)
    mesh:
      replication_factor: 2    # Store on 2 other nodes

    # S3 backup (enterprise)
    s3:
      endpoint: s3.amazonaws.com
      bucket: macula-backups
      prefix: "fleet/${NODE_ID}/"
```

**Restore:**
```bash
# List available backups
maculaos backup list

# Restore from backup
maculaos backup restore --from mesh --date 2024-01-15
maculaos backup restore --from s3 --latest
```

#### 4.13.3 Data Replication

Sync critical data across mesh nodes.

```yaml
maculaos:
  replication:
    enabled: true
    paths:
      - path: /var/lib/maculaos/shared/
        strategy: eventual      # eventual or strong
        replicas: 3             # Replicate to 3 nodes
```

#### 4.13.4 Storage Quotas

Prevent runaway disk usage.

```yaml
maculaos:
  storage:
    quotas:
      k3s_images: 20G          # Container image cache
      k3s_volumes: 50G         # PersistentVolumes
      logs: 5G                 # System logs
      user_data: unlimited     # /var/lib/data/
```

### 4.14 Developer Experience (NEW)

Tools and workflows for developers building on MaculaOS.

#### 4.14.1 Local Development

Run MaculaOS locally for development.

```bash
# Option 1: QEMU VM
maculaos-dev vm start
# Starts QEMU with MaculaOS, port-forwards SSH and Console

# Option 2: Docker container (limited, no k3s)
docker run -it --privileged maculacid/maculaos:dev

# Option 3: Multipass (macOS/Windows)
multipass launch maculaos
```

**Dev VM Features:**
- Pre-configured for development
- Hot-reload config changes
- Port forwarding (SSH, Console, k3s API)
- Shared folder with host

#### 4.14.2 SDK & CLI Tools

```bash
# Install MaculaOS SDK
brew install maculaos-sdk  # macOS
apt install maculaos-sdk   # Linux

# Create new MaculaOS app
maculaos-sdk new my-edge-app
cd my-edge-app
maculaos-sdk build
maculaos-sdk deploy --target edge-01.local
```

#### 4.14.3 Custom Image Builder

Build custom MaculaOS images with additional packages.

```yaml
# maculaos-custom.yaml
base: maculaos:1.2.0

# Additional Alpine packages
packages:
  - python3
  - py3-pip
  - opencv

# Additional container images (pre-loaded)
images:
  - myregistry.io/my-app:latest

# Custom overlay files
overlay:
  /etc/myapp/config.yaml: |
    setting: value

# Custom firstboot script
firstboot:
  - /opt/myapp/setup.sh
```

```bash
# Build custom image
maculaos-sdk build-image --config maculaos-custom.yaml

# Output: maculaos-custom-1.2.0-amd64.iso
```

#### 4.14.4 Testing Framework

```bash
# Run MaculaOS integration tests
maculaos-sdk test --target qemu

# Test scenarios:
# - Boot and reach Console
# - Mesh connectivity
# - App deployment
# - Update and rollback
# - Recovery mode
```

### 4.15 Enterprise Features (NEW)

Features for enterprise deployments.

#### 4.15.1 Role-Based Access Control (RBAC)

```yaml
maculaos:
  rbac:
    enabled: true
    roles:
      - name: admin
        permissions: ["*"]
      - name: operator
        permissions: ["read:*", "restart:services", "view:logs"]
      - name: viewer
        permissions: ["read:*"]

    users:
      - username: alice
        role: admin
        ssh_keys: [...]
      - username: bob
        role: operator
        ssh_keys: [...]
```

#### 4.15.2 Audit Logging

Compliance audit trail for all actions.

```yaml
maculaos:
  audit:
    enabled: true
    log_path: /var/log/maculaos/audit.log
    retention: 90d             # Keep 90 days
    forward_to: siem.corp.com  # Forward to SIEM

    # What to audit
    events:
      - auth.login
      - auth.logout
      - config.change
      - service.restart
      - update.apply
      - mesh.join
      - mesh.leave
```

**Audit Log Format:**
```json
{
  "timestamp": "2024-01-15T10:23:45Z",
  "event": "config.change",
  "user": "alice",
  "source_ip": "192.168.1.100",
  "details": {
    "file": "/var/lib/maculaos/config.yaml",
    "changes": ["network.dns_servers"]
  }
}
```

#### 4.15.3 LDAP/SSO Integration

Enterprise identity integration.

```yaml
maculaos:
  auth:
    provider: ldap             # ldap, oidc, or local

    ldap:
      url: ldaps://ldap.corp.com:636
      base_dn: "ou=users,dc=corp,dc=com"
      bind_dn: "cn=maculaos,ou=services,dc=corp,dc=com"
      bind_password_file: /var/lib/maculaos/secrets/ldap-password
      user_filter: "(uid=%s)"
      group_filter: "(memberOf=cn=maculaos-users,ou=groups,dc=corp,dc=com)"

    oidc:
      issuer: https://auth.corp.com
      client_id: maculaos
      client_secret_file: /var/lib/maculaos/secrets/oidc-secret
```

#### 4.15.4 Air-Gap Certificate Management

Offline PKI for secure environments.

```yaml
maculaos:
  pki:
    mode: airgap               # online or airgap

    airgap:
      ca_cert: /var/lib/maculaos/ca/ca.crt
      ca_key: /var/lib/maculaos/ca/ca.key
      crl_update: usb          # usb, manual
      cert_renewal: local      # Local CA signs renewals
```

### 4.16 Edge Computing Patterns (NEW)

Patterns for edge workloads.

#### 4.16.1 Edge Functions (FaaS)

Lightweight serverless functions at edge.

```
┌─────────────────────────────────────────────────────────────┐
│  Edge Functions Architecture                                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  HTTP Request                                         │  │
│  │  POST /api/process-image                             │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                 │
│                           ▼                                 │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Edge Function Runtime (Spin/Wasmer/Deno)            │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │  function processImage(request) {              │  │  │
│  │  │    const image = await request.blob();         │  │  │
│  │  │    const result = await ml.classify(image);    │  │  │
│  │  │    return Response.json(result);               │  │  │
│  │  │  }                                             │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  Benefits:                                                  │
│  • Cold start < 10ms (vs 100ms+ for containers)            │
│  • Memory: ~10MB per function (vs 100MB+ for pods)         │
│  • Sandboxed execution (WASM)                              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**
```yaml
maculaos:
  edge_functions:
    enabled: true
    runtime: spin              # spin, wasmer, or deno
    port: 3000
    functions_path: /var/lib/maculaos/functions/
```

#### 4.16.2 Data Pipelines

Stream processing at edge.

```yaml
maculaos:
  pipelines:
    enabled: true
    engine: benthos            # benthos or vector

    # Example pipeline: sensor data processing
    pipelines:
      - name: sensor-ingest
        input:
          mqtt:
            urls: ["tcp://localhost:1883"]
            topics: ["sensors/#"]
        processors:
          - jq: '.temperature = (.temperature * 1.8 + 32)'  # C to F
          - filter: '.temperature > 100'                    # Alert threshold
        output:
          http:
            url: "https://api.example.com/alerts"
```

#### 4.16.3 ML Inference

Optimized ML inference at edge.

```yaml
maculaos:
  ml:
    enabled: true
    runtime: onnx              # onnx, tflite, or openvino
    models_path: /var/lib/maculaos/models/

    # Hardware acceleration
    acceleration:
      cpu: true
      gpu: false               # Enable if GPU available
      tpu: false               # Enable for Coral
```

**Pre-loaded Models (Optional):**
- Object detection (YOLO, MobileNet)
- Text classification
- Anomaly detection

#### 4.16.4 Edge Caching

CDN-style caching at edge nodes.

```yaml
maculaos:
  cache:
    enabled: true
    engine: varnish            # varnish or nginx
    size: 10G

    # Cache rules
    rules:
      - match: "*.jpg,*.png,*.webp"
        ttl: 7d
      - match: "/api/static/*"
        ttl: 1h
      - match: "/api/dynamic/*"
        ttl: 0                 # No cache
```

### 4.17 Resilience & Self-Healing (NEW)

Automatic recovery from failures.

#### 4.17.1 Hardware Watchdog

Auto-reboot on system hang.

```yaml
maculaos:
  watchdog:
    enabled: true
    device: /dev/watchdog      # Hardware watchdog
    timeout: 60                # Reboot if not fed for 60s

    # Software watchdog (if no hardware)
    software_fallback: true
```

**Implementation:**
- Kernel hardware watchdog driver
- systemd `watchdog` service
- MaculaOS feeds watchdog every 30s
- If system hangs → automatic reboot after 60s

#### 4.17.2 Service Health Checks

Auto-restart unhealthy services.

```yaml
maculaos:
  health:
    checks:
      - name: k3s
        type: process
        process: k3s-server
        restart_on_failure: true
        max_restarts: 3

      - name: mesh
        type: http
        url: http://localhost:8080/health
        interval: 30s
        timeout: 5s
        restart_on_failure: true

      - name: disk
        type: disk
        path: /var/lib
        threshold: 90%         # Alert at 90% full
        action: alert          # alert or cleanup
```

#### 4.17.3 Partition Tolerance

Handle mesh network splits gracefully.

```
┌─────────────────────────────────────────────────────────────┐
│  Split-Brain Handling                                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Normal:                                                    │
│  [Node A]═══[Node B]═══[Node C]═══[Node D]                 │
│                                                             │
│  Network Partition:                                         │
│  [Node A]═══[Node B]   ║   [Node C]═══[Node D]             │
│       Partition 1      ║      Partition 2                   │
│                                                             │
│  Behavior:                                                  │
│  • Each partition continues operating                       │
│  • Local services remain available                          │
│  • Writes queue for sync when healed                       │
│  • Eventually consistent (not strong)                       │
│  • Automatic re-merge when connectivity restored           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 4.17.4 Auto-Healing Actions

Automated remediation for common issues.

| Issue | Detection | Auto-Action |
|-------|-----------|-------------|
| Disk full | `>90%` usage | Clean old logs, images |
| OOM | Kernel OOM killer | Restart offending pod |
| Service crash | Health check fail | Restart service (3x max) |
| Network down | No connectivity | Restart network stack |
| Mesh disconnect | No peers | Re-bootstrap mesh |
| Clock drift | NTP check | Force time sync |

```yaml
maculaos:
  auto_heal:
    enabled: true

    actions:
      disk_cleanup:
        trigger: disk_usage > 90%
        action: |
          journalctl --vacuum-size=100M
          crictl rmi --prune

      oom_restart:
        trigger: oom_kill_detected
        action: restart_pod
        cooldown: 5m
```

---

## 4.18 Priority Roadmap (NEW)

Prioritized implementation roadmap.

### v1.0 - Foundation (Must Have)

| Feature | Section | Status |
|---------|---------|--------|
| Boot and basic operation | 1-3 | ✅ Done |
| First-boot pairing | 5.2 | ✅ Done |
| A/B updates with rollback | 4.8 | 🔄 Partial |
| Recovery mode | 4.10.1 | ✅ Done |
| Factory reset | 4.10.2 | ✅ Done |
| Hardware watchdog | 4.17.1 | ✅ Done |
| Encryption at rest | 4.13.1 | ✅ Done |
| Basic diagnostics | 4.10.4 | ✅ Done |

### v1.1 - Edge Ready (Should Have)

| Feature | Section | Status |
|---------|---------|--------|
| Mesh role selection | 4.4 | ✅ Done |
| Local Git server | 4.9.1 | ✅ Done |
| P2P image registry | 4.9.2 | ✅ Done |
| Fleet health dashboard | 4.12.2 | ⬜ TODO (Portal UI) |
| Coordinated updates | 4.12.3 | ⬜ TODO (Portal UI) |
| Service health checks | 4.17.2 | ✅ Done |
| Backup/restore | 4.13.2 | ✅ Done |
| QEMU dev images | 4.14.1 | ⬜ TODO |

### v1.2 - Enterprise (Nice to Have)

| Feature | Section | Status |
|---------|---------|--------|
| RBAC | 4.15.1 | ⬜ TODO |
| Audit logging | 4.15.2 | ⬜ TODO |
| LDAP/SSO | 4.15.3 | ⬜ TODO |
| Edge functions | 4.16.1 | ⬜ TODO |
| ML inference | 4.16.3 | ⬜ TODO |
| Custom image builder | 4.14.3 | ⬜ TODO |

### v2.0+ - Future

| Feature | Section | Status |
|---------|---------|--------|
| TPM/Secure Boot | 4.11.2 | ⬜ Future |
| GPU/NPU support | 4.11.3 | ⬜ Future |
| Cellular modem | 4.11.5 | ⬜ Future |
| Satellite support | 4.11.5 | ⬜ Future |
| Air-gap PKI | 4.15.4 | ⬜ Future |

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

## 7. Dual ISO Strategy (Netboot + Airgapped)

MaculaOS provides two ISO variants to optimize for different deployment scenarios:

### 7.1 ISO Types Overview

| ISO Type | Size | Internet Required | Use Case |
|----------|------|-------------------|----------|
| **Netboot** | ~200-300MB | Yes (at install) | Quick eval, cloud VMs, fast downloads |
| **Airgapped** | ~800MB-1GB | No | Offline installs, air-gapped environments |

### 7.2 Netboot ISO Architecture

The Netboot ISO contains only what's needed to boot and download the rest:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         NETBOOT ISO CONTENTS                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Boot Loader (syslinux/grub)           ~2MB                            │
│  ├── grub.cfg / syslinux.cfg                                           │
│  └── EFI files                                                          │
│                                                                         │
│  Linux Kernel (vmlinuz)                ~11MB                           │
│  └── Compressed kernel image                                            │
│                                                                         │
│  Minimal Initrd                        ~50-100MB                       │
│  ├── BusyBox (core utilities)                                          │
│  ├── Network drivers (common NICs)                                     │
│  ├── curl/wget (HTTP client)                                           │
│  ├── Installer script                                                   │
│  └── Progress UI (dialog/whiptail)                                     │
│                                                                         │
│  Metadata                              ~1KB                            │
│  ├── version.txt                                                        │
│  └── checksums.txt (for downloads)                                     │
│                                                                         │
│  Total: ~200-300MB                                                     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**What's NOT in Netboot ISO:**
- rootfs.squashfs (~400MB) - downloaded during install
- kernel.squashfs (~200MB) - downloaded during install
- Airgap container images (~200MB) - downloaded as needed

### 7.3 Netboot Boot Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         NETBOOT INSTALL FLOW                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. Boot from USB/ISO                                                   │
│     └── Load kernel + minimal initrd into RAM                          │
│                                                                         │
│  2. Network Setup                                                       │
│     ├── Detect network interfaces                                       │
│     ├── DHCP or manual IP configuration                                │
│     └── Test internet connectivity                                      │
│                                                                         │
│  3. Download Components                                                 │
│     │                                                                   │
│     │  ┌─────────────────────────────────────────────────────────┐     │
│     │  │  Downloading MaculaOS v1.0.0...                         │     │
│     │  │                                                         │     │
│     │  │  [████████████████████░░░░░░░░░░░░] 65%                 │     │
│     │  │                                                         │     │
│     │  │  rootfs.squashfs    [████████████████████] 100%        │     │
│     │  │  kernel.squashfs    [██████████░░░░░░░░░░]  50%        │     │
│     │  │                                                         │     │
│     │  │  Source: github.com/macula-io/macula-os/releases       │     │
│     │  └─────────────────────────────────────────────────────────┘     │
│     │                                                                   │
│     ├── Download rootfs.squashfs from GitHub Releases                  │
│     ├── Download kernel.squashfs from GitHub Releases                  │
│     ├── Verify SHA256 checksums                                        │
│     └── Verify GPG signature (optional)                                │
│                                                                         │
│  4. Installation                                                        │
│     ├── Select target disk                                              │
│     ├── Create partitions (boot, rootfs-A, rootfs-B, data)             │
│     ├── Write squashfs files to rootfs-A                               │
│     └── Install bootloader                                              │
│                                                                         │
│  5. First Boot Setup                                                    │
│     └── Same as airgapped (pairing, config, etc.)                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.4 Airgapped ISO Architecture

The Airgapped ISO contains everything needed for offline installation:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        AIRGAPPED ISO CONTENTS                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Boot Loader                           ~2MB                            │
│  Linux Kernel                          ~11MB                           │
│  Full Initrd                           ~150MB                          │
│                                                                         │
│  rootfs.squashfs                       ~400MB                          │
│  ├── Alpine base system                                                │
│  ├── k3s binary                                                        │
│  ├── Macula components                                                 │
│  ├── NATS server                                                       │
│  └── All tools (vim, btop, git, etc.)                                 │
│                                                                         │
│  kernel.squashfs                       ~200MB                          │
│  ├── Linux kernel modules                                              │
│  └── Firmware blobs                                                    │
│                                                                         │
│  Airgap Images (optional)              ~200MB                          │
│  ├── macula-console:latest                                             │
│  ├── pause:3.6                                                         │
│  └── coredns:1.10.1                                                    │
│                                                                         │
│  Total: ~800MB-1GB                                                     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.5 Download Sources

| Component | Primary Source | Fallback |
|-----------|----------------|----------|
| rootfs.squashfs | GitHub Releases | Self-hosted CDN |
| kernel.squashfs | GitHub Releases | Self-hosted CDN |
| Checksums | GitHub Releases | Embedded in ISO |
| GPG signature | GitHub Releases | None |

**GitHub Release URLs:**
```
https://github.com/macula-io/macula-os/releases/download/v{VERSION}/
├── maculaos-{VERSION}-{ARCH}.iso           # Airgapped
├── maculaos-{VERSION}-{ARCH}-netboot.iso   # Netboot
├── maculaos-rootfs-{ARCH}.squashfs         # For netboot download
├── maculaos-kernel-{ARCH}.squashfs         # For netboot download
├── SHA256SUMS.txt
└── SHA256SUMS.txt.asc                      # GPG signature
```

### 7.6 Build System Changes

New Makefile targets:

```makefile
# Build netboot ISO (minimal, downloads rest)
netboot:
	ARCH=$(ARCH) ISO_TYPE=netboot make iso

# Build airgapped ISO (full, self-contained)
airgapped:
	ARCH=$(ARCH) ISO_TYPE=airgapped make iso

# Build both
all-isos: netboot airgapped
```

New Docker build stage:

```
images/
├── ... (existing stages)
├── 70-iso/                  # Existing - becomes airgapped
├── 71-netboot-iso/          # NEW - netboot variant
│   ├── Dockerfile
│   └── grub.cfg             # Netboot-specific boot config
```

### 7.7 Netboot Initrd Contents

The netboot initrd is smaller but must include networking:

```dockerfile
# images/71-netboot-iso/Dockerfile
FROM alpine:3.20 AS netboot-initrd

# Core utilities
RUN apk add --no-cache \
    busybox-static \
    curl \
    dialog \
    e2fsprogs \
    parted \
    dosfstools

# Network drivers (common hardware)
RUN apk add --no-cache \
    linux-firmware-none \
    linux-firmware-intel \
    linux-firmware-realtek \
    linux-firmware-broadcom

# Installer script
COPY scripts/netboot-install.sh /init
```

### 7.8 Installer Script (netboot-install.sh)

```bash
#!/bin/sh
# MaculaOS Netboot Installer

RELEASE_URL="https://github.com/macula-io/macula-os/releases/download"
VERSION="$(cat /etc/maculaos-version)"
ARCH="$(uname -m)"

# 1. Configure network
setup_network() {
    # Try DHCP first
    udhcpc -i eth0 || udhcpc -i eno1 || manual_network
}

# 2. Download components
download_components() {
    cd /tmp
    curl -L -o rootfs.squashfs \
        "${RELEASE_URL}/v${VERSION}/maculaos-rootfs-${ARCH}.squashfs"
    curl -L -o kernel.squashfs \
        "${RELEASE_URL}/v${VERSION}/maculaos-kernel-${ARCH}.squashfs"

    # Verify checksums
    curl -L -o SHA256SUMS.txt "${RELEASE_URL}/v${VERSION}/SHA256SUMS.txt"
    sha256sum -c SHA256SUMS.txt || exit 1
}

# 3. Install to disk
install_to_disk() {
    # ... partition and install ...
}
```

### 7.9 Configuration

```yaml
# /var/lib/maculaos/config.yaml
maculaos:
  install:
    # Netboot settings
    netboot:
      source: github              # github, self-hosted, local
      url: "https://github.com/macula-io/macula-os/releases"
      verify_signature: true      # Require GPG signature

    # Self-hosted option for enterprise
    self_hosted:
      url: "https://updates.corp.example.com/maculaos"
      ca_cert: "/etc/ssl/corp-ca.pem"
```

### 7.10 CI/CD Updates

```yaml
# .github/workflows/build.yml additions

jobs:
  build-netboot-amd64:
    runs-on: ubuntu-latest
    steps:
      - name: Build Netboot ISO
        run: ARCH=amd64 ISO_TYPE=netboot make iso

  build-airgapped-amd64:
    runs-on: ubuntu-latest
    steps:
      - name: Build Airgapped ISO
        run: ARCH=amd64 ISO_TYPE=airgapped make iso

  release:
    needs: [build-netboot-amd64, build-airgapped-amd64, ...]
    steps:
      - name: Upload Release Artifacts
        run: |
          gh release upload v$VERSION \
            maculaos-$VERSION-amd64.iso \
            maculaos-$VERSION-amd64-netboot.iso \
            maculaos-rootfs-amd64.squashfs \
            maculaos-kernel-amd64.squashfs \
            SHA256SUMS.txt
```

### 7.11 Implementation Tasks

- [ ] Create `images/71-netboot-iso/Dockerfile`
- [ ] Create `scripts/netboot-install.sh` installer script
- [ ] Add netboot/airgapped targets to Makefile
- [ ] Update CI/CD to build both ISO types
- [ ] Create download progress UI (dialog-based)
- [ ] Add GPG signing to release workflow
- [ ] Test netboot flow end-to-end
- [ ] Document netboot requirements (network, DHCP)

---

## 8. Output Artifacts

### 8.1 Image Formats

| Format | Variant | Use Case | Size (est.) |
|--------|---------|----------|-------------|
| ISO | Netboot | USB boot with internet | ~200-300MB |
| ISO | Airgapped | USB boot, offline install | ~800MB-1GB |
| IMG | Airgapped | Direct SD card write (RPi) | ~800MB |
| OVA | Airgapped | VirtualBox/VMware import | ~900MB |
| QCOW2 | Airgapped | KVM/Proxmox/libvirt | ~800MB |
| TAR | - | Container/chroot base | ~400MB |

### 8.2 File Naming

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
