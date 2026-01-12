# Plan: MaculaOS Lite

**Status:** Planning
**Created:** 2026-01-12
**Goal:** Reduce ISO from ~1.5GB to ~100-130MB by downloading k3s/Flux at install time

## Overview

MaculaOS Lite is a minimal bootable image that keeps the polished TUI experience but downloads heavy components (k3s, Flux, container images) during first-boot setup. This requires internet connectivity at installation time.

## Target Specifications

| Metric | Current | Lite Target |
|--------|---------|-------------|
| ISO Size | ~1.5GB | ~100-130MB |
| Boot Time | ~30s | ~20s |
| Install Time | ~2min (offline) | ~5-10min (online) |
| Internet Required | No | Yes |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     MaculaOS Lite ISO                       │
│                        (~100-130MB)                         │
├─────────────────────────────────────────────────────────────┤
│  vmlinuz        │ Minimal kernel (~15MB)                    │
│  initrd         │ Minimal initramfs (~20MB)                 │
│  rootfs.squashfs│ Alpine + TUIs (~70MB)                     │
│  grub + EFI     │ Bootloader (~5MB)                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ First Boot
┌─────────────────────────────────────────────────────────────┐
│                    macula-wizard TUI                        │
├─────────────────────────────────────────────────────────────┤
│  Step 1: Welcome                                            │
│  Step 2: Network Configuration                              │
│  Step 3: Identity (Ed25519 keypair)                         │
│  Step 4: Download k3s ─────────────────► get.k3s.io         │
│  Step 5: Download Flux ────────────────► fluxcd.io          │
│  Step 6: GitOps Setup (local/GitHub)                        │
│  Step 7: Deploy Console ───────────────► Docker Hub         │
│  Step 8: Summary + LAN Instructions                         │
└─────────────────────────────────────────────────────────────┘
```

## Components

### KEEP (Embedded in ISO)

| Component | Size | Purpose |
|-----------|------|---------|
| Linux kernel | ~15MB | Minimal: network, storage, USB, ext4, squashfs |
| Kernel modules | ~30MB | Essential drivers only |
| Alpine base | ~25MB | busybox, openrc, /bin/sh |
| Rust TUIs | ~10MB | macula-wizard, macula-tui |
| Networking | ~5MB | connman or networkmanager-cli |
| curl + ca-certs | ~3MB | For downloads |
| openssh | ~2MB | Remote access |
| Branding | ~1MB | Banners, motd, issue |

### REMOVE (Download at Install)

| Component | Size Saved | Download Source |
|-----------|------------|-----------------|
| k3s binary | ~60MB | get.k3s.io |
| nats-server | ~15MB | Not needed initially |
| Pre-pulled images | ~500MB+ | Docker Hub at runtime |
| Full kernel modules | ~400MB | Only embed essentials |
| Documentation | ~50MB | Available online |

## Implementation Phases

### Phase 1: Minimal Kernel

**Goal:** Reduce kernel from ~656MB to ~50MB

**Files to modify:**
- `images/10-kernel-stage1/Dockerfile` - Kernel config
- `images/40-kernel/Dockerfile` - Module selection

**Kernel modules to KEEP:**
```
# Storage
ext4, squashfs, loop, sd_mod, ahci, nvme

# Network
e1000e, r8169, igb, ixgbe          # Common Ethernet
iwlwifi, ath9k, rtl8xxxu           # Common WiFi (optional)
bridge, veth, vxlan                 # Container networking

# USB
usb-storage, xhci_hcd, ehci_hcd

# Filesystem
overlay, nfs, cifs                  # k3s needs these

# Other
tun, nf_conntrack, iptables modules # k3s networking
```

**Kernel modules to REMOVE:**
```
# GPU drivers (not needed for server)
# Sound drivers
# Uncommon hardware
# Debugging/tracing
# Virtualization (unless needed)
```

### Phase 2: Minimal Rootfs

**Goal:** Reduce rootfs from ~300MB to ~70MB

**Files to modify:**
- `images/05-base/Dockerfile` - Base Alpine packages
- `images/20-rootfs/Dockerfile` - Rootfs assembly

**Alpine packages to KEEP:**
```apk
busybox openrc curl ca-certificates openssh
connman iptables iproute2 util-linux
e2fsprogs dosfstools parted
```

**Alpine packages to REMOVE:**
```apk
vim (use vi from busybox)
btop (download later if wanted)
tmux (download later if wanted)
man-pages, docs
```

**Changes to rootfs Dockerfile:**
```dockerfile
# REMOVE these lines:
COPY --from=k3s /output/install.sh ...
COPY --from=progs /output/nats-server ...

# KEEP these lines:
COPY --from=progs /output/macula-wizard ...
COPY --from=progs /output/macula-tui ...
```

### Phase 3: Enhanced macula-wizard

**Goal:** Add download/install steps to the TUI wizard

**Files to modify:**
- `rust/crates/macula-wizard/src/app.rs` - Add new steps
- `rust/crates/macula-wizard/src/installer.rs` - NEW: Download logic
- `rust/crates/macula-tui-common/src/widgets/` - Progress bar widget

**New wizard steps:**

```rust
pub enum Step {
    Welcome,
    Network,
    Identity,
    // NEW STEPS
    DownloadK3s,      // Download and install k3s
    WaitK3s,          // Wait for k3s to be ready
    InstallFlux,      // Download and install Flux CLI
    GitOpsSetup,      // Choose local/GitHub/existing
    DeployConsole,    // Apply GitOps manifests
    ConfigureHosts,   // Add console.macula.io to /etc/hosts
    // END NEW
    Summary,
}
```

**New installer module:**

```rust
// rust/crates/macula-wizard/src/installer.rs

pub async fn download_k3s(progress: &ProgressBar) -> Result<()> {
    // curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable=traefik" sh -
    // Update progress bar as download proceeds
}

pub async fn install_flux(progress: &ProgressBar) -> Result<()> {
    // curl -s https://fluxcd.io/install.sh | bash
}

pub async fn install_nginx_ingress() -> Result<()> {
    // kubectl apply -f https://raw.githubusercontent.com/.../deploy.yaml
}

pub async fn create_gitops_manifests(config: &GitOpsConfig) -> Result<()> {
    // Generate and apply Kustomize manifests
}

pub async fn deploy_console(config: &ConsoleConfig) -> Result<()> {
    // flux install + kubectl apply
}
```

**Progress display:**

```
┌─────────────────────────────────────────────────────────────┐
│                    Installing k3s                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Downloading k3s binary...                                  │
│  ████████████████████░░░░░░░░░░░░░░░░░░░░  52% (31MB/60MB)  │
│                                                             │
│  This may take a few minutes depending on your connection.  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Phase 4: Simplified Build Pipeline

**Goal:** Faster builds, smaller output

**Files to modify:**
- `Makefile` or `scripts/images` - Add "lite" target
- `images/output/02-lite/Dockerfile` - NEW: Lite ISO builder

**New build target:**

```bash
# Build full ISO (current behavior)
make build

# Build lite ISO (new)
make build-lite
```

**Lite Dockerfile:**

```dockerfile
# images/output/02-lite/Dockerfile
FROM macula-kernel-lite:${TAG} as kernel
FROM macula-rootfs-lite:${TAG} as rootfs

# Assemble minimal ISO
# - Smaller kernel
# - Smaller initrd
# - Smaller rootfs.squashfs
# - No pre-pulled container images
```

### Phase 5: First-Boot Flow

**Goal:** Seamless network-dependent installation

**Boot sequence:**

```
1. GRUB loads kernel + initrd
2. Kernel mounts rootfs.squashfs
3. OpenRC starts services:
   - connman (auto DHCP)
   - sshd
   - macula-firstboot (checks .configured flag)
4. If not configured:
   - getty displays login banner
   - User logs in (macula/macula)
   - macula-wizard auto-starts (via profile.d)
5. Wizard runs through all steps:
   - Network (skip if already connected)
   - Identity
   - Download k3s (with progress)
   - Download Flux (with progress)
   - GitOps setup
   - Deploy Console
   - Summary with LAN instructions
6. Creates /var/lib/maculaos/.configured
7. User can now access Console at http://console.macula.io
```

## File Changes Summary

| File | Action | Description |
|------|--------|-------------|
| `images/10-kernel-stage1/Dockerfile` | MODIFY | Minimal kernel config |
| `images/40-kernel/Dockerfile` | MODIFY | Reduce module list |
| `images/05-base/Dockerfile` | MODIFY | Minimal Alpine packages |
| `images/20-rootfs/Dockerfile` | MODIFY | Remove k3s, nats-server |
| `images/output/02-lite/Dockerfile` | CREATE | Lite ISO assembly |
| `rust/crates/macula-wizard/src/app.rs` | MODIFY | Add download steps |
| `rust/crates/macula-wizard/src/installer.rs` | CREATE | Download/install logic |
| `rust/crates/macula-wizard/src/gitops.rs` | CREATE | GitOps manifest generation |
| `rust/crates/macula-tui-common/src/widgets/progress.rs` | MODIFY | Download progress bar |
| `Makefile` | MODIFY | Add build-lite target |
| `scripts/build-lite` | CREATE | Lite build script |

## Testing Plan

1. **Build lite ISO** - Verify size is ~100-130MB
2. **Boot in QEMU** - Verify kernel loads, network works
3. **Run wizard** - Verify all download steps complete
4. **Access Console** - Verify http://console.macula.io works
5. **LAN access** - Verify from another machine
6. **Persistence** - Verify config survives reboot

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Slow internet = bad UX | Show progress bars, estimated time |
| Download fails mid-install | Retry logic, resume capability |
| Missing kernel module | Test on various hardware, add module |
| k3s version changes | Pin versions, test updates |

## Success Criteria

- [ ] ISO size under 150MB
- [ ] Boot to wizard in under 30 seconds
- [ ] Full install completes in under 10 minutes (on decent internet)
- [ ] Console accessible at http://console.macula.io
- [ ] Works on: QEMU, VirtualBox, bare metal x86_64
- [ ] TUI experience is polished (progress bars, clear messaging)

## Future Enhancements

1. **Offline mode** - USB drive with pre-downloaded packages
2. **ARM64 support** - Same lite approach for Raspberry Pi
3. **Recovery mode** - Minimal shell if wizard fails
4. **OTA updates** - Download new rootfs, A/B partition switch
