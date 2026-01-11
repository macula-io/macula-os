#!/bin/sh
# MaculaOS Welcome Script
# Displays system info with ASCII logo on interactive login

# Only run for interactive shells
case "$-" in
    *i*) ;;
    *) return ;;
esac

# Only run once per session
[ -n "$MACULA_WELCOME_SHOWN" ] && return
export MACULA_WELCOME_SHOWN=1

# Colors
O='\033[1;33m'    # Orange/Yellow
C='\033[1;36m'    # Cyan
W='\033[1;37m'    # White
G='\033[0;37m'    # Gray
R='\033[0m'       # Reset

# Check if fastfetch is available
if command -v fastfetch >/dev/null 2>&1; then
    fastfetch --config /etc/macula/fastfetch.jsonc 2>/dev/null
else
    # Fallback: colorful banner
    echo ""
    echo "${O}      .o---o.       ${W}__  __                 _        ___  ____${R}"
    echo "${O}     /|     |\\     ${W}|  \\/  | __ _  ___ _   _| | __ _ / _ \\/ ___|${R}"
    echo "${O}    o-+-----+-o    ${W}| |\\/| |/ _\` |/ __| | | | |/ _\` | | | \\___ \\${R}"
    echo "${O}     \\|     |/     ${W}| |  | | (_| | (__| |_| | | (_| | |_| |___) |${R}"
    echo "${O}      'o---o'      ${W}|_|  |_|\\__,_|\\___|\\__,_|_|\\__,_|\\___/|____/${R}"
    echo "${O}        \\ /${R}"
    echo "${O}         o         ${C}Decentralized Edge Computing Platform${R}"
    echo ""
    echo "${G}  Hostname: $(hostname)${R}"
    echo "${G}  IP:       $(ip -4 addr show scope global 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | head -1)${R}"
    echo "${G}  Uptime:   $(uptime -p 2>/dev/null || uptime)${R}"
    echo ""
fi

# Show quick menu
echo ""
echo "${O}+-------------------------------------------------------------------+${R}"
echo "${O}|${W}  Quick Commands:                                                ${O}|${R}"
echo "${O}+-------------------------------------------------------------------+${R}"
echo "${O}|${G}  macula-tui      ${W}- Interactive management dashboard            ${O}|${R}"
echo "${O}|${G}  macula-wizard   ${W}- First-time setup wizard                     ${O}|${R}"
echo "${O}|${G}  kubectl get all ${W}- View Kubernetes resources                   ${O}|${R}"
echo "${O}|${G}  btop            ${W}- System monitor                              ${O}|${R}"
echo "${O}|${G}  journalctl -f   ${W}- Follow system logs                          ${O}|${R}"
echo "${O}+-------------------------------------------------------------------+${R}"
echo ""
