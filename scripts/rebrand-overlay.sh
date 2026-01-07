#!/bin/bash
# Rebrand overlay files from k3os to macula
set -e

cd "$(dirname "$0")/.."

echo "Rebranding overlay files..."

# Rename libexec directory
if [ -d "overlay/libexec/k3os" ]; then
    echo "Renaming overlay/libexec/k3os -> overlay/libexec/macula"
    mv overlay/libexec/k3os overlay/libexec/macula
fi

# Function to rebrand a file
rebrand_file() {
    local file="$1"
    if [ -f "$file" ]; then
        echo "Rebranding: $file"
        # Paths
        sed -i 's|/usr/libexec/k3os|/usr/libexec/macula|g' "$file"
        sed -i 's|/.base/k3os/system|/.base/macula/system|g' "$file"
        sed -i 's|/k3os/system|/macula/system|g' "$file"
        sed -i 's|/k3os/data|/macula/data|g' "$file"
        sed -i 's|/run/k3os|/run/macula|g' "$file"
        sed -i 's|/var/lib/rancher/k3os|/var/lib/macula|g' "$file"

        # Environment variables
        sed -i 's|K3OS_SYSTEM|MACULA_SYSTEM|g' "$file"
        sed -i 's|K3OS_DEBUG|MACULA_DEBUG|g' "$file"
        sed -i 's|K3OS_MODE|MACULA_MODE|g' "$file"
        sed -i 's|K3OS_VERSION|MACULA_VERSION|g' "$file"
        sed -i 's|K3OS_FILE|MACULA_FILE|g' "$file"
        sed -i 's|K3OS_SRC|MACULA_SRC|g' "$file"

        # Kernel command line params
        sed -i 's|k3os\.debug|macula.debug|g' "$file"
        sed -i 's|k3os\.mode|macula.mode|g' "$file"
        sed -i 's|k3os\.fallback_mode|macula.fallback_mode|g' "$file"

        # Hostname pattern
        sed -i 's|HOSTNAME=k3os-|HOSTNAME=macula-|g' "$file"

        # k3os binary references
        sed -i 's|/k3os config|/maculaos config|g' "$file"
        sed -i 's|k3os config|maculaos config|g' "$file"
        sed -i 's|/k3os rc|/maculaos rc|g' "$file"

        # Command name in paths like /macula/system/k3os/current/k3os
        sed -i 's|/macula/system/k3os/|/macula/system/macula/|g' "$file"
        sed -i 's|/current/k3os"|/current/maculaos"|g' "$file"
        sed -i 's|/current/k3os$|/current/maculaos|g' "$file"
        sed -i 's|/current/k3os |/current/maculaos |g' "$file"

        # Relative paths (without leading /)
        sed -i 's|k3os/system|macula/system|g' "$file"
        sed -i 's|k3os/data|macula/data|g' "$file"

        # Binary references
        sed -i 's|"sudo k3os install"|"sudo maculaos install"|g' "$file"

        # Function names
        sed -i 's|setup_k3os|setup_macula|g' "$file"

        # Messages
        sed -i 's|k3OS|MaculaOS|g' "$file"
    fi
}

# Rebrand all files in overlay/libexec/macula
for file in overlay/libexec/macula/*; do
    rebrand_file "$file"
done

# Rebrand init.d files
for file in overlay/etc/init.d/*; do
    rebrand_file "$file"
done

# Rebrand other overlay files
rebrand_file "overlay/etc/profile.d/aliases.sh"
rebrand_file "overlay/etc/motd"
rebrand_file "overlay/etc/issue"
rebrand_file "overlay/lib/os-release"
rebrand_file "overlay/init"

echo "Done!"
