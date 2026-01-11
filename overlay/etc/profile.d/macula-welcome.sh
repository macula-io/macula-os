#!/bin/sh
# MaculaOS Welcome Script - runs on interactive login

# Only run for interactive shells
case "$-" in
    *i*) ;;
    *) return ;;
esac

# Only run once per session
[ -n "$MACULA_WELCOME_SHOWN" ] && return
export MACULA_WELCOME_SHOWN=1

# ANSI colors
ESC=$(printf '\033')
C="${ESC}[1;36m"   # Cyan
W="${ESC}[1;37m"   # White
O="${ESC}[1;33m"   # Orange
G="${ESC}[0;37m"   # Gray
R="${ESC}[0m"      # Reset

# Check if fastfetch is available
if command -v fastfetch >/dev/null 2>&1; then
    fastfetch --config /etc/macula/fastfetch.jsonc 2>/dev/null
else
    # Fallback banner with ASCII art title
    printf '\n'
    printf '%s    .o---o.\n' "$C"
    printf '   /|     |\\%s      %s╔╦╗╔═╗╔═╗╦ ╦╦  ╔═╗  ╔═╗╔═╗%s\n' "$R" "$O" "$R"
    printf '%s  o-+-----+-o%s     %s║║║╠═╣║  ║ ║║  ╠═╣  ║ ║╚═╗%s\n' "$C" "$R" "$O" "$R"
    printf '%s   \\|     |/%s      %s╩ ╩╩ ╩╚═╝╚═╝╩═╝╩ ╩  ╚═╝╚═╝%s\n' "$C" "$R" "$O" "$R"
    printf '%s    '\''o---o'\''%s\n' "$C" "$R"
    printf '%s      \\ /%s         %sDecentralized Edge Computing%s\n' "$C" "$R" "$W" "$R"
    printf '%s       o%s\n' "$C" "$R"
    printf '\n'
    printf '%s  Hostname: %s%s\n' "$G" "$(hostname)" "$R"
    printf '%s  Uptime:   %s%s\n' "$G" "$(uptime -p 2>/dev/null || uptime)" "$R"
    printf '\n'
fi

# Quick menu with box-drawing characters
printf '\n'
printf '%s╔═══════════════════════════════════════════════════════════════════╗%s\n' "$O" "$R"
printf '%s║%s  Quick Commands:                                                  %s║%s\n' "$O" "$W" "$O" "$R"
printf '%s╠═══════════════════════════════════════════════════════════════════╣%s\n' "$O" "$R"
printf '%s║%s  macula-tui      %s- Interactive management dashboard              %s║%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s║%s  macula-wizard   %s- First-time setup wizard                       %s║%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s║%s  kubectl get all %s- View Kubernetes resources                     %s║%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s║%s  btop            %s- System monitor                                %s║%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s╚═══════════════════════════════════════════════════════════════════╝%s\n' "$O" "$R"
printf '\n'
