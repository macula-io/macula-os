# MaculaOS - Custom Linux Distribution Plan

**Status:** In Progress (Phase 1)
**Created:** 2026-01-07
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
3. **Declarative**: YAML-based config (`/k3os/system/config.yaml`)
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
│  │  - Parse cloud-config (/k3os/system/config.yaml)                │   │
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
│   ├── k3os/
│   │   └── config.yaml.tmpl     # Template with Macula defaults
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
# /k3os/system/config.yaml (MaculaOS defaults)

k3os:
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
2. Checks /var/lib/macula/.configured flag
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
- packer templates have k3os references (not critical for boot)

**Rebranding completed (2026-01-07):**
- Go module: `github.com/rancher/k3os` → `github.com/macula-io/macula-os`
- CLI app: `k3os` → `maculaos`
- System paths: `/k3os/system` → `/macula/system`, `/run/k3os` → `/run/macula`
- Environment vars: `K3OS_*` → `MACULA_*` / `MACULAOS_*`
- Docker images: `k3os-*` → `macula-*`
- Partition labels: `K3OS_STATE` → `MACULA_STATE`
- Boot params: `k3os.mode` → `macula.mode`

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
- [ ] Add Avahi/mDNS support
- [ ] Create default k3os config template
- [ ] Add Macula branding (boot splash, MOTD)
- [ ] Pre-load Console container image
- [ ] Pre-load essential k3s images (airgap)
- [ ] Update overlay files

**Files to create:**
- `images/45-macula/Dockerfile`
- `overlay/etc/init.d/macula-mdns`
- `overlay/etc/avahi/services/macula.service`
- `overlay/etc/k3os/config.yaml.tmpl`

### Phase 3: First-Boot Wizard (Week 3-4)

**Goal:** Zero-touch setup experience

- [ ] Create firstboot Go binary
- [ ] Implement pairing flow UI
- [ ] Generate QR codes with pairing URL
- [ ] Exchange codes with Portal API
- [ ] Store credentials securely
- [ ] Configure Console on success
- [ ] Create init script for firstboot
- [ ] Test full pairing flow

**Files to create:**
- `cmd/macula-firstboot/main.go`
- `overlay/opt/macula/firstboot/`
- `overlay/etc/init.d/macula-firstboot`

### Phase 4: Multi-Arch Builds & Testing (Week 4-5)

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

### Phase 5: Distribution & Documentation (Week 5-6)

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
