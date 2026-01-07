# MaculaOS Build Guide

This guide explains how to build MaculaOS from source.

## Prerequisites

- Docker (v20.10+)
- Git
- 16GB+ disk space for build artifacts
- 4GB+ RAM (for QEMU testing)

## Quick Start (amd64)

```bash
# Clone the repository
git clone https://github.com/macula-io/macula-os.git
cd macula-os

# Build for amd64
make ci

# Artifacts will be in dist/artifacts/
ls -la dist/artifacts/
```

## Build Targets

| Target | Description |
|--------|-------------|
| `make ci` | Full build: compile, test, validate, package |
| `make build` | Build all Docker images |
| `make test` | Run Go tests |
| `make package` | Create final artifacts |

## Build Artifacts

After a successful build, the following artifacts are created in `dist/artifacts/`:

| File | Description | Size |
|------|-------------|------|
| `maculaos-{arch}.iso` | Bootable ISO for USB/CD | ~1.5GB |
| `maculaos-vmlinuz-{arch}` | Linux kernel | ~11MB |
| `maculaos-initrd-{arch}` | Initial ramdisk | ~726MB |
| `macula-kernel-{arch}.squashfs` | Kernel modules squashfs | ~656MB |
| `macula-rootfs-{arch}.tar.gz` | Root filesystem tarball | ~1.5GB |

## Architecture-Specific Builds

### amd64 (default)

```bash
# Explicit amd64 build
ARCH=amd64 make ci
```

### arm64

The current build system doesn't support true cross-compilation from amd64.

**Option 1: Native arm64 hardware** (Recommended)
```bash
# On Raspberry Pi 4/5, ARM server, or cloud ARM instance
ARCH=arm64 make ci
```

**Option 2: Future - Docker buildx**
True cross-compilation would require modifying the Dockerfiles to use
`--platform linux/arm64` on all `FROM` statements. This is planned for
a future enhancement.

**Note:** Setting `ARCH=arm64` on an amd64 host will create artifacts with
arm64 naming, but the binaries will still be amd64 because Docker pulls
images matching the host architecture.

## Testing with QEMU

Test the built image in QEMU (requires 4GB+ RAM due to large initrd):

```bash
# Run in QEMU with 4GB RAM
MEMORY=4096 ./scripts/run-qemu macula.mode=live

# Install to virtual disk
MEMORY=4096 ./scripts/run-qemu macula.mode=install macula.install.device=/dev/vda

# Login credentials (live mode)
# Username: macula
# Password: (shown on screen during boot)
```

### QEMU Boot Modes

| Mode | Kernel Parameter | Description |
|------|-----------------|-------------|
| Live | `macula.mode=live` | Boot from ISO, no installation |
| Install | `macula.mode=install` | Interactive installation |
| Disk | (default) | Boot from installed disk |

## Build System Architecture

MaculaOS uses a multi-stage Docker build system orchestrated by [Dapper](https://github.com/rancher/dapper):

```
images/
├── 00-base/          # Alpine base + build tools
├── 10-gobuild/       # Go compiler environment
├── 20-progs/         # maculaos Go binary
├── 30-kernel-stage1/ # Linux kernel compilation
├── 40-kernel/        # Kernel + initrd assembly
├── 50-rootfs/        # Root filesystem
├── 60-iso/           # ISO image creation
├── 70-pkg/           # Package builder
└── 80-tar/           # Final tarball
```

### Key Build Files

| File | Purpose |
|------|---------|
| `Dockerfile.dapper` | Dapper build environment |
| `Makefile` | Build targets |
| `scripts/version` | Version and architecture detection |
| `scripts/build` | Main build orchestration |
| `scripts/images` | Docker image build functions |

## Customization

### Default Configuration

Edit `overlay/etc/macula/config.yaml.tmpl` to change default settings.

### Boot Splash / Branding

- GRUB menu: `overlay/share/rancher/macula/grub.cfg`
- MOTD: `overlay/etc/motd`
- Issue: `overlay/etc/issue`

### Adding Packages

Add packages to the appropriate Dockerfile stage:
- System packages: `images/50-rootfs/Dockerfile`
- Initrd tools: `images/40-kernel/Dockerfile`

## Troubleshooting

### Build fails with Go module errors

Ensure vendor mode is used:
```dockerfile
# In images/20-progs/Dockerfile
RUN gobuild -mod=vendor -o /output/maculaos
```

### QEMU boot fails with "no such device"

The initrd requires kmod and its libraries. Verify `images/40-kernel/Dockerfile` includes:
```dockerfile
RUN apk add kmod && cd /usr/src/initrd && \
    cp /bin/kmod bin/ && \
    cp /lib/libz.so.1* lib/ && \
    cp /usr/lib/liblzma.so.5* lib/ && \
    cp /usr/lib/libzstd.so.1* lib/ && \
    cp /lib/libcrypto.so.3 lib/ && \
    cp /lib/ld-musl-x86_64.so.1 lib/
```

### arm64 build produces amd64 artifacts

QEMU user-mode emulation is not configured. Run:
```bash
docker run --privileged --rm tonistiigi/binfmt --install arm64
```

### Out of memory during QEMU test

The initrd is ~726MB and requires significant RAM to decompress:
```bash
MEMORY=4096 ./scripts/run-qemu macula.mode=live
```

## Version Information

- **Base**: Alpine Linux 3.20
- **Kernel**: Linux 6.6.x LTS (Alpine linux-lts)
- **k3s**: v1.23.3+k3s1
- **Init**: OpenRC

## References

- [k3OS (archived)](https://github.com/rancher/k3os)
- [k3s Documentation](https://docs.k3s.io/)
- [Alpine Linux](https://alpinelinux.org/)
- [Dapper](https://github.com/rancher/dapper)
