#!/bin/bash
# Rebrand K3OS config references to Macula
# This script updates Go source files to use .Macula. instead of .K3OS.

set -e

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "Rebranding .K3OS. references to .Macula. in Go source files..."

# Files that contain .K3OS. references
FILES=(
    "pkg/sysctl/sysctl.go"
    "pkg/config/read_test.go"
    "pkg/module/module.go"
    "pkg/config/write.go"
    "pkg/cliinstall/ask.go"
    "pkg/cliinstall/install.go"
    "pkg/cc/funcs.go"
)

for f in "${FILES[@]}"; do
    filepath="${REPO_ROOT}/${f}"
    if [[ -f "$filepath" ]]; then
        echo "  Updating ${f}..."
        sed -i 's/\.K3OS\./\.Macula\./g' "$filepath"
    else
        echo "  Warning: ${f} not found"
    fi
done

echo "Done. K3OS -> Macula struct reference renaming complete."
