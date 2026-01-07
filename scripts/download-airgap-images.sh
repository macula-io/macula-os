#!/bin/bash
# Download container images for airgap operation
# Run this on a machine with Docker to prepare images for ISO build

set -e

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIR="${REPO_ROOT}/overlay/var/lib/rancher/k3s/agent/images"

# Images to pre-load for MaculaOS
IMAGES=(
    "maculacid/macula-console:latest"
    "rancher/mirrored-pause:3.6"
    "rancher/local-path-provisioner:v0.0.24"
    "coredns/coredns:1.10.1"
)

echo "MaculaOS Airgap Image Downloader"
echo "================================"
echo "Output directory: ${OUTPUT_DIR}"
echo ""

mkdir -p "${OUTPUT_DIR}"

for image in "${IMAGES[@]}"; do
    # Create safe filename from image name
    filename=$(echo "$image" | tr '/:' '_').tar
    output_path="${OUTPUT_DIR}/${filename}"

    echo "Processing: ${image}"

    if [[ -f "${output_path}" ]]; then
        echo "  Already exists: ${filename}"
        continue
    fi

    echo "  Pulling..."
    docker pull "${image}"

    echo "  Saving to ${filename}..."
    docker save "${image}" > "${output_path}"

    # Compress if larger than 10MB
    size=$(stat -f%z "${output_path}" 2>/dev/null || stat -c%s "${output_path}" 2>/dev/null || echo 0)
    if [[ ${size} -gt 10485760 ]]; then
        echo "  Compressing (${size} bytes)..."
        gzip "${output_path}"
        output_path="${output_path}.gz"
    fi

    echo "  Done: $(ls -lh "${output_path}" | awk '{print $5}')"
    echo ""
done

echo "All images downloaded to ${OUTPUT_DIR}"
echo ""
echo "Total size:"
du -sh "${OUTPUT_DIR}"
