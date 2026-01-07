#!/bin/bash
# Fix redundant newlines in fmt.Println calls
# These cause golint errors: "fmt.Println arg list ends with redundant newline"

set -e

cd "$(dirname "$0")/.."

# Files with the issue
FILES=(
    "pkg/cli/backup/backup.go"
    "pkg/cli/encrypt/encrypt.go"
    "pkg/cli/health/health.go"
    "pkg/cli/mesh/mesh.go"
)

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "Fixing $file"
        # Replace patterns like: fmt.Println("...\n") with fmt.Println("...")
        # Also handle: fmt.Println("...\033[0m\n") -> fmt.Println("...\033[0m")
        sed -i 's/fmt\.Println("\([^"]*\)\\n")/fmt.Println("\1")/g' "$file"
    fi
done

echo "Done. Run 'go vet ./...' to verify."
