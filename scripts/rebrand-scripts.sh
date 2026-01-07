#!/bin/bash
# Rebrand scripts/ from k3os to macula
set -e

cd "$(dirname "$0")/.."

echo "Rebranding scripts/..."

for file in scripts/images scripts/package scripts/run scripts/run-qemu; do
    if [ -f "$file" ]; then
        echo "Rebranding: $file"

        # Docker image names
        sed -i 's|k3os-|macula-|g' "$file"
        sed -i 's|/k3os:|/maculaos:|g' "$file"

        # Artifact names
        sed -i 's|k3os-vmlinuz|macula-vmlinuz|g' "$file"
        sed -i 's|k3os-initrd|macula-initrd|g' "$file"
        sed -i 's|k3os-\$ARCH|maculaos-\$ARCH|g' "$file"
        sed -i 's|k3os-\${ARCH}|maculaos-\${ARCH}|g' "$file"

        # State dir
        sed -i 's|state/k3os-|state/macula-|g' "$file"

        # Kernel cmdline params
        sed -i 's|k3os\.mode|macula.mode|g' "$file"
        sed -i 's|k3os\.install|macula.install|g' "$file"
        sed -i 's|k3os\.password|macula.password|g' "$file"

        # Build output
        sed -i 's|/k3os$|/maculaos|g' "$file"
        sed -i 's|/k3os |/maculaos |g' "$file"
    fi
done

echo "Done!"
