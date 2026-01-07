#!/bin/bash
# Setup Docker buildx with QEMU for multi-architecture builds
#
# This enables ARM64 builds on x86_64 hosts using QEMU user-mode emulation.
# Run this once before attempting ARM64 builds.
#
# Usage:
#   ./scripts/setup-buildx.sh
#
# After running, you can build for ARM64 with:
#   ARCH=arm64 make ci
#
set -e

echo "=== Setting up Docker buildx with QEMU for multi-arch builds ==="

# Check if running as root or with docker access
if ! docker info > /dev/null 2>&1; then
    echo "Error: Cannot access Docker. Are you in the docker group?"
    exit 1
fi

# Step 1: Install QEMU user-mode emulation via binfmt
echo ""
echo "Step 1: Installing QEMU binfmt handlers..."
echo "This registers QEMU as the handler for ARM64 binaries."
echo ""

# Use the official Docker image to set up binfmt_misc
docker run --rm --privileged tonistiigi/binfmt --install all

# Verify ARM64 is registered
if [ -f /proc/sys/fs/binfmt_misc/qemu-aarch64 ]; then
    echo "✓ ARM64 (aarch64) QEMU handler registered"
else
    echo "⚠ ARM64 handler may not be registered. Check /proc/sys/fs/binfmt_misc/"
fi

# Step 2: Create buildx builder with multi-platform support
echo ""
echo "Step 2: Creating buildx builder..."
echo ""

BUILDER_NAME="maculaos-builder"

# Remove existing builder if present
docker buildx rm ${BUILDER_NAME} 2>/dev/null || true

# Create new builder with multi-platform support
docker buildx create \
    --name ${BUILDER_NAME} \
    --driver docker-container \
    --platform linux/amd64,linux/arm64 \
    --bootstrap

# Set as default builder
docker buildx use ${BUILDER_NAME}

# Verify builder
echo ""
echo "Step 3: Verifying builder..."
docker buildx inspect --bootstrap

echo ""
echo "=== Setup complete! ==="
echo ""
echo "Available platforms:"
docker buildx inspect | grep Platforms

echo ""
echo "To build for ARM64:"
echo "  ARCH=arm64 USE_BUILDX=1 make ci"
echo ""
echo "To build for both architectures:"
echo "  USE_BUILDX=1 make ci-multiarch"
echo ""
