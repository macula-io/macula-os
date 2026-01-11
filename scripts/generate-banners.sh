#!/bin/sh
# Generate MaculaOS ASCII art banners
# Requires: figlet
# Optional: chafa (for image-to-ASCII conversion)

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OVERLAY_DIR="$SCRIPT_DIR/../overlay"

# ANSI color codes
CYAN='\033[1;36m'
ORANGE='\033[1;33m'
WHITE='\033[1;37m'
GRAY='\033[0;37m'
RESET='\033[0m'
BOLD='\033[1m'

# The mesh logo (7 lines) - irregular mesh network
LOGO_1='    o-------o'
LOGO_2='   /|       |\'
LOGO_3='  o-+--o----+-o'
LOGO_4='   \|\ | /| /'
LOGO_5='    o--O--o'
LOGO_6='   /|  |  |\'
LOGO_7='  o----o----o'

show_help() {
    echo "MaculaOS Banner Generator"
    echo ""
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  --fonts              Show all available figlet fonts with MACULA preview"
    echo "  --font <name>        Preview specific font (e.g., --font slant)"
    echo "  --preview [font]     Show color preview (default: standard)"
    echo "  --write [font]       Write all banner files using specified font"
    echo "  --from-image <file>  Convert PNG/SVG to ASCII (requires chafa)"
    echo "  --help               Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 --fonts                    # See all font options"
    echo "  $0 --font slant               # Preview slant font"
    echo "  $0 --preview standard         # Color preview with standard font"
    echo "  $0 --write standard           # Generate all files with standard font"
    echo "  $0 --from-image logo.png      # Convert image to ASCII"
}

show_fonts() {
    echo "=== Available Figlet Fonts ==="
    echo ""

    # List of interesting fonts to show
    FONTS="standard big slant small shadow lean mini block banner script"

    for font in $FONTS; do
        if [ -f "/usr/share/figlet/fonts/${font}.flf" ]; then
            printf "${BOLD}=== %s ===${RESET}\n" "$font"
            figlet -f "$font" "MACULA" 2>/dev/null || echo "(error)"
            echo ""
        fi
    done

    echo "---"
    echo "To use a font: $0 --preview <font_name>"
    echo "All fonts in: /usr/share/figlet/fonts/"
}

preview_font() {
    FONT="${1:-standard}"

    if ! command -v figlet >/dev/null 2>&1; then
        echo "Error: figlet is required. Install with: sudo pacman -S figlet"
        exit 1
    fi

    echo "Generating with font: $FONT"
    echo ""

    FIGLET_OUTPUT=$(figlet -f "$FONT" "MACULA  OS" 2>/dev/null)
    if [ -z "$FIGLET_OUTPUT" ]; then
        echo "Error: Font '$FONT' not found"
        exit 1
    fi

    # Extract lines
    LINE1=$(echo "$FIGLET_OUTPUT" | sed -n '1p')
    LINE2=$(echo "$FIGLET_OUTPUT" | sed -n '2p')
    LINE3=$(echo "$FIGLET_OUTPUT" | sed -n '3p')
    LINE4=$(echo "$FIGLET_OUTPUT" | sed -n '4p')
    LINE5=$(echo "$FIGLET_OUTPUT" | sed -n '5p')
    LINE6=$(echo "$FIGLET_OUTPUT" | sed -n '6p')

    echo "=== Raw figlet output ==="
    echo "$FIGLET_OUTPUT"
    echo ""
    echo "=== Combined with logo (no colors) ==="
    echo ""
    echo "${LOGO_1}"
    echo "${LOGO_2}       ${LINE1}"
    echo "${LOGO_3}     ${LINE2}"
    echo "${LOGO_4}      ${LINE3}"
    echo "${LOGO_5}       ${LINE4}"
    echo "${LOGO_6}         ${LINE5}"
    echo "${LOGO_7}         ${LINE6}"
    echo "                  Decentralized Edge Computing Platform"
    echo ""
}

color_preview() {
    FONT="${1:-standard}"

    if ! command -v figlet >/dev/null 2>&1; then
        echo "Error: figlet is required. Install with: sudo pacman -S figlet"
        exit 1
    fi

    FIGLET_OUTPUT=$(figlet -f "$FONT" "MACULA  OS" 2>/dev/null)
    if [ -z "$FIGLET_OUTPUT" ]; then
        echo "Error: Font '$FONT' not found"
        exit 1
    fi

    LINE1=$(echo "$FIGLET_OUTPUT" | sed -n '1p')
    LINE2=$(echo "$FIGLET_OUTPUT" | sed -n '2p')
    LINE3=$(echo "$FIGLET_OUTPUT" | sed -n '3p')
    LINE4=$(echo "$FIGLET_OUTPUT" | sed -n '4p')
    LINE5=$(echo "$FIGLET_OUTPUT" | sed -n '5p')
    LINE6=$(echo "$FIGLET_OUTPUT" | sed -n '6p')

    echo ""
    printf "${BOLD}=== COLOR PREVIEW (font: %s) ===${RESET}\n" "$FONT"
    echo ""
    printf "${CYAN}${LOGO_1}${RESET}\n"
    printf "${CYAN}${LOGO_2}${RESET}       ${ORANGE}${LINE1}${RESET}\n"
    printf "${CYAN}${LOGO_3}${RESET}     ${ORANGE}${LINE2}${RESET}\n"
    printf "${CYAN}${LOGO_4}${RESET}      ${ORANGE}${LINE3}${RESET}\n"
    printf "${CYAN}${LOGO_5}${RESET}       ${ORANGE}${LINE4}${RESET}\n"
    printf "${CYAN}${LOGO_6}${RESET}         ${ORANGE}${LINE5}${RESET}\n"
    printf "${CYAN}${LOGO_7}${RESET}         ${ORANGE}${LINE6}${RESET}\n"
    printf "                  ${WHITE}Decentralized Edge Computing Platform${RESET}\n"
    echo ""
}

from_image() {
    IMAGE="$1"

    if [ -z "$IMAGE" ]; then
        echo "Error: Please specify an image file"
        echo "Usage: $0 --from-image <file.png|file.svg>"
        exit 1
    fi

    if [ ! -f "$IMAGE" ]; then
        echo "Error: File not found: $IMAGE"
        exit 1
    fi

    echo "=== Image to ASCII conversion tools ==="
    echo ""

    # Try chafa first (best quality)
    if command -v chafa >/dev/null 2>&1; then
        echo "Using: chafa"
        echo ""
        echo "--- Size: 40 cols, ASCII only ---"
        chafa --size=40 --symbols=ascii "$IMAGE"
        echo ""
        echo "--- Size: 60 cols, ASCII only ---"
        chafa --size=60 --symbols=ascii "$IMAGE"
        echo ""
        echo "--- Size: 40 cols, with colors ---"
        chafa --size=40 "$IMAGE"
        echo ""
    elif command -v img2txt >/dev/null 2>&1; then
        echo "Using: img2txt (libcaca)"
        echo ""
        img2txt -W 40 "$IMAGE"
    elif command -v jp2a >/dev/null 2>&1; then
        echo "Using: jp2a (JPEG only)"
        echo ""
        jp2a --width=40 "$IMAGE" 2>/dev/null || echo "Note: jp2a only supports JPEG files"
    else
        echo "No image-to-ASCII tool found!"
        echo ""
        echo "Install one of:"
        echo "  sudo pacman -S chafa      # Recommended - best quality"
        echo "  sudo pacman -S libcaca    # For img2txt"
        echo "  sudo pacman -S jp2a       # JPEG only"
        exit 1
    fi
}

write_files() {
    FONT="${1:-standard}"

    if ! command -v figlet >/dev/null 2>&1; then
        echo "Error: figlet is required. Install with: sudo pacman -S figlet"
        exit 1
    fi

    FIGLET_OUTPUT=$(figlet -f "$FONT" "MACULA  OS" 2>/dev/null)
    if [ -z "$FIGLET_OUTPUT" ]; then
        echo "Error: Font '$FONT' not found"
        exit 1
    fi

    LINE1=$(echo "$FIGLET_OUTPUT" | sed -n '1p')
    LINE2=$(echo "$FIGLET_OUTPUT" | sed -n '2p')
    LINE3=$(echo "$FIGLET_OUTPUT" | sed -n '3p')
    LINE4=$(echo "$FIGLET_OUTPUT" | sed -n '4p')
    LINE5=$(echo "$FIGLET_OUTPUT" | sed -n '5p')

    echo ""
    echo "=== Writing banner files (font: $FONT) ==="

    # 1. Write logo.txt
    cat > "$OVERLAY_DIR/etc/macula/logo.txt" << 'EOF'
    .o---o.
   /|     |\
  o-+-----+-o
   \|     |/
    'o---o'
      \ /
       o
EOF
    echo "Wrote: $OVERLAY_DIR/etc/macula/logo.txt"

    # 2. Write banner.txt
    cat > "$OVERLAY_DIR/etc/macula/banner.txt" << EOF

${LOGO_1}
${LOGO_2}       ${LINE1}
${LOGO_3}     ${LINE2}
${LOGO_4}      ${LINE3}
${LOGO_5}       ${LINE4}
${LOGO_6}         ${LINE5}
${LOGO_7}
                  Decentralized Edge Computing Platform
EOF
    echo "Wrote: $OVERLAY_DIR/etc/macula/banner.txt"

    # 3. Write /etc/issue
    cat > "$OVERLAY_DIR/etc/issue" << EOF

${LOGO_1}
${LOGO_2}       ${LINE1}
${LOGO_3}     ${LINE2}
${LOGO_4}      ${LINE3}
${LOGO_5}       ${LINE4}
${LOGO_6}         ${LINE5}
${LOGO_7}
                  Decentralized Edge Computing Platform

  Kernel \\r on \\m (\\l)

  Login: macula / macula

EOF
    echo "Wrote: $OVERLAY_DIR/etc/issue"

    # 4. Write update-issue script
    # Escape the figlet output for embedding in the script
    cat > "$OVERLAY_DIR/sbin/update-issue" << SCRIPT
#!/bin/sh
# Generate /etc/issue with colorful MaculaOS banner
# Font: $FONT

. /etc/os-release

ESC=\$(printf '\\033')
CYAN="\${ESC}[1;36m"
WHITE="\${ESC}[1;37m"
ORANGE="\${ESC}[1;33m"
GRAY="\${ESC}[0;37m"
RESET="\${ESC}[0m"

cat > /etc/issue << BANNER

\${CYAN}    .o---o.\${RESET}
\${CYAN}   /|     |\\\\\${RESET}       \${ORANGE}${LINE1}\${RESET}
\${CYAN}  o-+-----+-o\${RESET}     \${ORANGE}${LINE2}\${RESET}
\${CYAN}   \\\\|     |/\${RESET}      \${ORANGE}${LINE3}\${RESET}
\${CYAN}    'o---o'\${RESET}       \${ORANGE}${LINE4}\${RESET}
\${CYAN}      \\\\ /\${RESET}         \${ORANGE}${LINE5}\${RESET}
\${CYAN}       o\${RESET}
                  \${WHITE}Decentralized Edge Computing Platform\${RESET}

\${GRAY}  \$PRETTY_NAME - Kernel \\r on \\m (\\l)\${RESET}

BANNER

NICS=\$(ip -br addr show 2>/dev/null | grep -E -v '^(lo|flannel|cni|veth|docker|br-)' | head -3)
if [ -n "\$NICS" ]; then
    printf '%s  Network:%s\\n' "\$CYAN" "\$RESET" >> /etc/issue
    printf '%s%s%s\\n\\n' "\$GRAY" "\$NICS" "\$RESET" >> /etc/issue
fi

printf '%s  Login: %smacula%s / %smacula%s\\n\\n' "\$WHITE" "\$ORANGE" "\$WHITE" "\$ORANGE" "\$RESET" >> /etc/issue
SCRIPT
    chmod +x "$OVERLAY_DIR/sbin/update-issue"
    echo "Wrote: $OVERLAY_DIR/sbin/update-issue"

    # 5. Write macula-welcome.sh
    cat > "$OVERLAY_DIR/etc/profile.d/macula-welcome.sh" << SCRIPT
#!/bin/sh
# MaculaOS Welcome Script

case "\$-" in *i*) ;; *) return ;; esac
[ -n "\$MACULA_WELCOME_SHOWN" ] && return
export MACULA_WELCOME_SHOWN=1

ESC=\$(printf '\\033')
C="\${ESC}[1;36m"
W="\${ESC}[1;37m"
O="\${ESC}[1;33m"
G="\${ESC}[0;37m"
R="\${ESC}[0m"

if command -v fastfetch >/dev/null 2>&1; then
    fastfetch --config /etc/macula/fastfetch.jsonc 2>/dev/null
else
    printf '\\n'
    printf '%s    .o---o.%s\\n' "\$C" "\$R"
    printf '%s   /|     |\\\\%s       %s${LINE1}%s\\n' "\$C" "\$R" "\$O" "\$R"
    printf '%s  o-+-----+-o%s     %s${LINE2}%s\\n' "\$C" "\$R" "\$O" "\$R"
    printf '%s   \\\\|     |/%s      %s${LINE3}%s\\n' "\$C" "\$R" "\$O" "\$R"
    printf "%s    'o---o'%s       %s${LINE4}%s\\n" "\$C" "\$R" "\$O" "\$R"
    printf '%s      \\\\ /%s         %s${LINE5}%s\\n' "\$C" "\$R" "\$O" "\$R"
    printf '%s       o%s\\n' "\$C" "\$R"
    printf '                  %sDecentralized Edge Computing Platform%s\\n' "\$W" "\$R"
    printf '\\n'
    printf '%s  Hostname: %s%s\\n' "\$G" "\$(hostname)" "\$R"
    printf '%s  Uptime:   %s%s\\n' "\$G" "\$(uptime -p 2>/dev/null || uptime)" "\$R"
    printf '\\n'
fi

printf '\\n'
printf '%s+-------------------------------------------------------------------+%s\\n' "\$O" "\$R"
printf '%s|%s  Quick Commands:                                                  %s|%s\\n' "\$O" "\$W" "\$O" "\$R"
printf '%s+-------------------------------------------------------------------+%s\\n' "\$O" "\$R"
printf '%s|%s  macula-tui      %s- Interactive management dashboard              %s|%s\\n' "\$O" "\$C" "\$G" "\$O" "\$R"
printf '%s|%s  macula-wizard   %s- First-time setup wizard                       %s|%s\\n' "\$O" "\$C" "\$G" "\$O" "\$R"
printf '%s|%s  kubectl get all %s- View Kubernetes resources                     %s|%s\\n' "\$O" "\$C" "\$G" "\$O" "\$R"
printf '%s|%s  btop            %s- System monitor                                %s|%s\\n' "\$O" "\$C" "\$G" "\$O" "\$R"
printf '%s+-------------------------------------------------------------------+%s\\n' "\$O" "\$R"
printf '\\n'
SCRIPT
    echo "Wrote: $OVERLAY_DIR/etc/profile.d/macula-welcome.sh"

    echo ""
    echo "=== All banner files written with font: $FONT ==="
}

# Main command handling
case "$1" in
    --help|-h)
        show_help
        ;;
    --fonts)
        show_fonts
        ;;
    --font)
        preview_font "$2"
        ;;
    --preview)
        color_preview "$2"
        ;;
    --write)
        write_files "$2"
        ;;
    --from-image)
        from_image "$2"
        ;;
    *)
        show_help
        ;;
esac
