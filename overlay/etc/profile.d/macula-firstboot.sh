#!/bin/sh
# Run macula-wizard on first login if not yet configured

# Only run in interactive shells
case "$-" in *i*) ;; *) return ;; esac

# Only run once per session
[ -n "$MACULA_FIRSTBOOT_CHECKED" ] && return
export MACULA_FIRSTBOOT_CHECKED=1

# Check if already configured
MARKER="/var/lib/maculaos/.configured"
if [ ! -f "$MARKER" ]; then
    echo ""
    echo "First-time setup required. Starting wizard..."
    echo ""
    sleep 1

    # Run the wizard
    if command -v macula-wizard >/dev/null 2>&1; then
        macula-wizard
    else
        echo "Warning: macula-wizard not found"
    fi
fi
