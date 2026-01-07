# Pre-loaded Container Images (Airgap Support)

This directory holds pre-loaded container images for airgap/offline operation.

k3s automatically imports any `.tar` or `.tar.gz` files in this directory
at startup when `--airgap-extra-registry` is not set.

## Adding Images

### Method 1: Build Stage (Recommended for ISO)

Create/modify `images/45-macula/Dockerfile` to download images:

```dockerfile
FROM alpine:3.20 as macula-images

# Pull and save images
RUN apk add --no-cache docker-cli && \
    mkdir -p /output/images

# Add each image
RUN docker pull maculacid/macula-console:latest && \
    docker save maculacid/macula-console:latest > /output/images/macula-console.tar

# ... add more images as needed
```

### Method 2: Manual Addition

```bash
# Pull and save image locally
docker pull maculacid/macula-console:latest
docker save maculacid/macula-console:latest > macula-console.tar

# Copy to overlay
cp macula-console.tar overlay/var/lib/rancher/k3s/agent/images/
```

### Method 3: Post-Install Script

Use `scripts/download-airgap-images.sh` to download images after installation.

## Images to Include

For basic MaculaOS operation:

| Image | Purpose | Size |
|-------|---------|------|
| maculacid/macula-console:latest | Macula Console UI | ~150MB |
| rancher/mirrored-pause:3.6 | k8s pause container | ~0.5MB |
| rancher/local-path-provisioner:v0.0.24 | Local storage | ~10MB |
| coredns/coredns:1.10.1 | DNS service | ~15MB |

## Note

Large images significantly increase ISO size. Consider:
- Downloading images on first boot if internet available
- Using compressed tar.gz format
- Only including essential images for offline operation
