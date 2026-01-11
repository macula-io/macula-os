#!/bin/bash
# Write MaculaOS ISO to USB drive
set -e

ISO_PATH="${1:-$(dirname $0)/../dist/artifacts/maculaos-amd64.iso}"
USB_DEVICE="${2:-/dev/sdb}"

if [ ! -f "$ISO_PATH" ]; then
    echo "ERROR: ISO not found at $ISO_PATH"
    exit 1
fi

if [ ! -b "$USB_DEVICE" ]; then
    echo "ERROR: USB device $USB_DEVICE not found"
    exit 1
fi

# Show device info
echo "=== USB Device Info ==="
lsblk -o NAME,SIZE,MODEL,MOUNTPOINT "$USB_DEVICE"
echo ""

# Confirm
read -p "WARNING: This will ERASE ALL DATA on $USB_DEVICE. Continue? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 1
fi

# Unmount any mounted partitions
echo "Unmounting any mounted partitions..."
for part in ${USB_DEVICE}*; do
    if mountpoint -q "$part" 2>/dev/null || mount | grep -q "$part"; then
        sudo umount "$part" 2>/dev/null || true
    fi
done

# Write ISO
ISO_SIZE=$(du -h "$ISO_PATH" | cut -f1)
echo "Writing ISO to $USB_DEVICE..."
echo "ISO: $ISO_PATH ($ISO_SIZE)"
echo "This may take a few minutes..."

# Use pv for progress if available, otherwise dd with progress
if command -v pv &>/dev/null; then
    sudo sh -c "pv '$ISO_PATH' | dd of='$USB_DEVICE' bs=4M conv=fsync 2>/dev/null"
else
    echo "(Install 'pv' for better progress display)"
    sudo dd if="$ISO_PATH" of="$USB_DEVICE" bs=4M status=progress oflag=direct conv=fsync
fi

echo ""
echo "=== Complete ==="
echo "You can now boot from $USB_DEVICE"
echo ""
echo "Boot menu options:"
echo "  1. MaculaOS LiveCD & Installer - Live boot (no install)"
echo "  2. MaculaOS Installer - Install to disk"
echo "  3. MaculaOS Rescue Shell - Emergency shell"
