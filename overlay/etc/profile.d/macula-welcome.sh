#!/bin/sh
# MaculaOS Welcome Script

case "$-" in *i*) ;; *) return ;; esac
[ -n "$MACULA_WELCOME_SHOWN" ] && return
export MACULA_WELCOME_SHOWN=1

ESC=$(printf '\033')
C="${ESC}[1;36m"
W="${ESC}[1;37m"
O="${ESC}[1;33m"
G="${ESC}[0;37m"
R="${ESC}[0m"

if command -v fastfetch >/dev/null 2>&1; then
    fastfetch --config /etc/macula/fastfetch.jsonc 2>/dev/null
else
    printf '\n'
    printf '%s    .o---o.%s\n' "$C" "$R"
    printf '%s   /|     |\\%s       %s __  __    _    ____ _   _ _        _        ___  ____  %s\n' "$C" "$R" "$O" "$R"
    printf '%s  o-+-----+-o%s     %s|  \/  |  / \  / ___| | | | |      / \      / _ \/ ___| %s\n' "$C" "$R" "$O" "$R"
    printf '%s   \\|     |/%s      %s| |\/| | / _ \| |   | | | | |     / _ \    | | | \___ \ %s\n' "$C" "$R" "$O" "$R"
    printf "%s    'o---o'%s       %s| |  | |/ ___ \ |___| |_| | |___ / ___ \   | |_| |___) |%s\n" "$C" "$R" "$O" "$R"
    printf '%s      \\ /%s         %s|_|  |_/_/   \_\____|\___/|_____/_/   \_\   \___/|____/ %s\n' "$C" "$R" "$O" "$R"
    printf '%s       o%s\n' "$C" "$R"
    printf '                  %sDecentralized Edge Computing Platform%s\n' "$W" "$R"
    printf '\n'
    printf '%s  Hostname: %s%s\n' "$G" "$(hostname)" "$R"
    printf '%s  Uptime:   %s%s\n' "$G" "$(uptime -p 2>/dev/null || uptime)" "$R"
    printf '\n'
fi

printf '\n'
printf '%s+-------------------------------------------------------------------+%s\n' "$O" "$R"
printf '%s|%s  Quick Commands:                                                  %s|%s\n' "$O" "$W" "$O" "$R"
printf '%s+-------------------------------------------------------------------+%s\n' "$O" "$R"
printf '%s|%s  macula-tui      %s- Interactive management dashboard              %s|%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s|%s  macula-wizard   %s- First-time setup wizard                       %s|%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s|%s  kubectl get all %s- View Kubernetes resources                     %s|%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s|%s  btop            %s- System monitor                                %s|%s\n' "$O" "$C" "$G" "$O" "$R"
printf '%s+-------------------------------------------------------------------+%s\n' "$O" "$R"
printf '\n'
