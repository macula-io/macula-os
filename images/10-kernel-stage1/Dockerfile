ARG REPO
ARG TAG
# MaculaOS: Use Alpine 3.20 with built-in kernel packages
FROM alpine:3.20

RUN apk --no-cache add \
    coreutils \
    kmod \
    lz4 \
    rsync \
    squashfs-tools \
    xz \
    zstd

ARG ARCH

# Install Alpine's LTS kernel (6.6.x series)
# linux-lts is the Long Term Support kernel
RUN apk --no-cache add \
    linux-lts \
    linux-firmware \
    mkinitfs

# Get kernel version dynamically
RUN KVERSION=$(ls /lib/modules | head -1) && \
    echo "Kernel version: $KVERSION" && \
    echo "$KVERSION" > /tmp/kversion

# Create output directory structure
RUN mkdir -p /output/lib/modules /output/lib/firmware /output/headers

# Copy kernel, modules, and firmware
RUN KVERSION=$(cat /tmp/kversion) && \
    cp /boot/vmlinuz-lts /output/vmlinuz && \
    cp /boot/System.map-lts /output/System.map 2>/dev/null || true && \
    cp /boot/config-lts /output/config 2>/dev/null || true && \
    cp -r /lib/modules/$KVERSION /output/lib/modules/ && \
    cp -r /lib/firmware/* /output/lib/firmware/ 2>/dev/null || true && \
    echo "$KVERSION" > /output/version

# Generate initrd module and firmware lists
RUN KVERSION=$(cat /tmp/kversion) && \
    cd /output && \
    find lib/modules -name "*.ko*" > /output/initrd-modules && \
    echo "lib/modules/$KVERSION/modules.order" >> /output/initrd-modules && \
    echo "lib/modules/$KVERSION/modules.builtin" >> /output/initrd-modules && \
    find lib/firmware -type f > /output/initrd-firmware 2>/dev/null || touch /output/initrd-firmware

# Regenerate module dependencies
RUN KVERSION=$(cat /tmp/kversion) && \
    depmod -b /output $KVERSION
