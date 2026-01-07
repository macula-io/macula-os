#!/bin/bash
# Rebrand .Macula. references to .Maculaos. in Go source files
# This is part of the consistent naming convention update

set -e

cd "$(dirname "$0")/.."

echo "Rebranding .Macula. references to .Maculaos. in Go source files..."

# Files that contain .Macula. references
GO_FILES=(
    "pkg/sysctl/sysctl.go"
    "pkg/module/module.go"
    "pkg/cliinstall/install.go"
    "pkg/cliinstall/ask.go"
    "pkg/cc/funcs.go"
    "pkg/config/read_test.go"
    "pkg/config/write.go"
)

for filepath in "${GO_FILES[@]}"; do
    if [ -f "$filepath" ]; then
        echo "  Processing $filepath"
        sed -i 's/\.Macula\./\.Maculaos\./g' "$filepath"
    else
        echo "  Warning: $filepath not found"
    fi
done

echo "Done. .Macula. -> .Maculaos. struct reference renaming complete."
