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

**ISO Boot Fixes (2026-01-11):**

- Fixed grub.cfg to use dynamic kernel discovery for ISO9660 compatibility:
  - ISO9660 symlinks appear as 0-byte files, breaking "current" symlink
  - Added `find_kernel` function that iterates directories to find kernel.squashfs
  - Committed in `4238c50` - "fix(grub): use dynamic kernel discovery for ISO9660 compatibility"
- Consolidated paths for boot consistency:
  - Changed `/maculaos/system/` → `/macula/system/` to match init scripts
  - Init scripts expect `MACULA_SYSTEM=/.base/macula/system`
  - Files updated: images/60-package/Dockerfile, images/70-iso/grub.cfg, package/Dockerfile, scripts/package
  - Committed in `423fd33` - "fix(build): consolidate /macula/system/ paths for boot consistency"
- Boot now succeeds with `macula.mode=live` - reaches login prompt

**QEMU Testing (2026-01-11):**

- run-qemu script bypasses GRUB, loads kernel/initrd directly
- Requires `macula.mode=live` as argument: `./scripts/run-qemu macula.mode=live`
- ISO boots to login prompt with all services running (k3s, sshd, first-boot wizard)

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

| Category         | k3OS Original            | Final MaculaOS       |
| ---------------- | ------------------------ | -------------------- |
| Partition label  | `K3OS_STATE`             | `MACULAOS_STATE`     |
| ISO volume label | `K3OS`                   | `MACULAOS`           |
| Config directory | `/var/lib/rancher/k3os/` | `/var/lib/maculaos/` |
| YAML config key  | `k3os:`                  | `maculaos:`          |
| Go struct        | `K3OS`                   | `Maculaos`           |
| Login user       | `rancher`                | `macula`             |

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

| Feature         | Config Field        | Applicator        | CLI Prompt      | Web UI   |
| --------------- | ------------------- | ----------------- | --------------- | -------- |
| Keyboard Layout | `maculaos.keyboard` | `ApplyKeyboard()` | `AskKeyboard()` | Dropdown |
| Timezone        | `maculaos.timezone` | `ApplyTimezone()` | `AskTimezone()` | Dropdown |
| Locale          | `maculaos.locale`   | `ApplyLocale()`   | `AskLocale()`   | Dropdown |

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
    persistence: true # Enable persistent overlay
    persistence_device: auto # auto-detect or specify device
    persistence_size: 4G # Size of persistence partition
```

- [ ] Add persistence config fields
- [ ] Modify live boot script to mount persistence overlay
- [ ] Add "Enable persistence?" prompt to installer
- [ ] Create persistence partition on USB stick (if space available)

#### 4.4 Files to Create/Modify

| File                              | Changes                                                                       |
| --------------------------------- | ----------------------------------------------------------------------------- |
| `pkg/config/config.go`            | Add `Keyboard`, `Timezone`, `Locale`, `Mesh`, `Live` fields                   |
| `pkg/cc/funcs.go`                 | Add `ApplyKeyboard()`, `ApplyTimezone()`, `ApplyLocale()`, `ApplyMeshRoles()` |
| `pkg/cliinstall/ask.go`           | Add `AskKeyboard()`, `AskTimezone()`, `AskLocale()`, `AskMeshRoles()`         |
| `pkg/cliinstall/install.go`       | Integrate new prompts into wizard flow                                        |
| `cmd/macula-firstboot/main.go`    | Add API endpoints for new config                                              |
| `cmd/macula-firstboot/templates/` | Add settings pages                                                            |
| `overlay/libexec/macula/live`     | Add persistence overlay support                                               |

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

| ISO Type      | Size       | Internet Required | Use Case                                  |
| ------------- | ---------- | ----------------- | ----------------------------------------- |
| **Netboot**   | ~200-300MB | Yes (at install)  | Quick eval, cloud VMs, fast downloads     |
| **Airgapped** | ~800MB-1GB | No                | Offline installs, air-gapped environments |

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

| Component       | Primary Source  | Fallback        |
| --------------- | --------------- | --------------- |
| rootfs.squashfs | GitHub Releases | Self-hosted CDN |
| kernel.squashfs | GitHub Releases | Self-hosted CDN |
| Checksums       | GitHub Releases | Embedded in ISO |
| GPG signature   | GitHub Releases | None            |

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
      source: github # github, self-hosted, local
      url: "https://github.com/macula-io/macula-os/releases"
      verify_signature: true # Require GPG signature

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

| Format | Variant   | Use Case                   | Size (est.) |
| ------ | --------- | -------------------------- | ----------- |
| ISO    | Netboot   | USB boot with internet     | ~200-300MB  |
| ISO    | Airgapped | USB boot, offline install  | ~800MB-1GB  |
| IMG    | Airgapped | Direct SD card write (RPi) | ~800MB      |
| OVA    | Airgapped | VirtualBox/VMware import   | ~900MB      |
| QCOW2  | Airgapped | KVM/Proxmox/libvirt        | ~800MB      |
| TAR    | -         | Container/chroot base      | ~400MB      |

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
