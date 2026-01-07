#!/bin/sh
# MaculaOS Welcome Script
# Displays system info with ASCII logo on interactive login

# Only run for interactive shells
case "$-" in
    *i*) ;;
    *) return ;;
esac

# Only run once per session (check if we're in a subshell)
[ -n "$MACULA_WELCOME_SHOWN" ] && return
export MACULA_WELCOME_SHOWN=1

# Check if fastfetch is available
if command -v fastfetch >/dev/null 2>&1; then
    fastfetch --config /etc/macula/fastfetch.jsonc 2>/dev/null
else
    # Fallback: simple banner
    if [ -f /etc/macula/banner.txt ]; then
        cat /etc/macula/banner.txt
    fi
    echo ""
    echo "  Hostname: $(hostname)"
    echo "  IP:       $(ip -4 addr show scope global | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | head -1)"
    echo "  Uptime:   $(uptime -p 2>/dev/null || uptime)"
    echo ""
fi
