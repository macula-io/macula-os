#!/bin/bash
# Rebrand images/ Dockerfiles from k3os to macula
set -e

cd "$(dirname "$0")/.."

echo "Rebranding images/ Dockerfiles..."

# Function to rebrand a file
rebrand_file() {
    local file="$1"
    if [ -f "$file" ]; then
        echo "Rebranding: $file"

        # Docker image names
        sed -i 's|k3os-base|macula-base|g' "$file"
        sed -i 's|k3os-gobuild|macula-gobuild|g' "$file"
        sed -i 's|k3os-progs|macula-progs|g' "$file"
        sed -i 's|k3os-rootfs|macula-rootfs|g' "$file"
        sed -i 's|k3os-k3s|macula-k3s|g' "$file"
        sed -i 's|k3os-bin|macula-bin|g' "$file"
        sed -i 's|k3os-kernel|macula-kernel|g' "$file"
        sed -i 's|k3os-package|macula-package|g' "$file"
        sed -i 's|k3os-iso|macula-iso|g' "$file"
        sed -i 's|k3os-tar|macula-tar|g' "$file"

        # Build stages
        sed -i 's|as k3os$|as macula|g' "$file"
        sed -i 's|as k3os-build|as macula-build|g' "$file"
        sed -i 's|--from=k3os |--from=macula |g' "$file"

        # Go module paths
        sed -i 's|github.com/rancher/k3os|github.com/macula-io/macula-os|g' "$file"

        # Output binary names
        sed -i 's|/output/k3os|/output/maculaos|g' "$file"
        sed -i 's|k3os-install\.sh|macula-install.sh|g' "$file"

        # Symlink paths
        sed -i 's|/k3os/system/k3os/|/macula/system/macula/|g' "$file"
        sed -i 's|/k3os/system/k3s/|/macula/system/k3s/|g' "$file"
        sed -i 's|/sbin/k3os|/sbin/maculaos|g' "$file"
        sed -i 's|/libexec/k3os/|/libexec/macula/|g' "$file"

        # Artifact names in comments/echos
        sed -i 's|k3OS|MaculaOS|g' "$file"
    fi
}

# Find and rebrand all Dockerfiles in images/
find images/ -name "Dockerfile" -o -name "gobuild" | while read file; do
    rebrand_file "$file"
done

# Rebrand Dockerfile.dapper
rebrand_file "Dockerfile.dapper"

echo "Done!"
