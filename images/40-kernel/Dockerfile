ARG REPO
ARG TAG
FROM ${REPO}/macula-bin:${TAG} as bin

FROM ${REPO}/macula-kernel-stage1:${TAG} as kernel

FROM ${REPO}/macula-base:${TAG}
ARG TAG
RUN apk add squashfs-tools
COPY --from=kernel /output/ /usr/src/kernel/

RUN mkdir -p /usr/src/initrd/lib && \
    cd /usr/src/kernel && \
    tar cf - -T initrd-modules -T initrd-firmware | tar xf - -C /usr/src/initrd/ && \
    depmod -b /usr/src/initrd $(cat /usr/src/kernel/version)

RUN mkdir -p /output && \
    cd /usr/src/kernel && \
    depmod -b . $(cat /usr/src/kernel/version) && \
    mksquashfs . /output/kernel.squashfs -comp gzip

RUN cp /usr/src/kernel/version /output/ && \
    cp /usr/src/kernel/vmlinuz /output/

COPY --from=bin /output/ /usr/src/macula/

# Add kmod utilities for module loading (required for loop device)
# kmod requires: libz (/lib), liblzma (/usr/lib), libzstd (/usr/lib), libcrypto (/lib), musl libc (/lib)
# Note: musl library name is architecture-dependent (ld-musl-x86_64.so.1 or ld-musl-aarch64.so.1)
RUN apk add kmod && cd /usr/src/initrd && \
    mkdir -p bin sbin lib && \
    cp /bin/kmod bin/ && \
    ln -s kmod bin/modprobe && \
    ln -s kmod bin/insmod && \
    ln -s kmod bin/rmmod && \
    ln -s kmod bin/lsmod && \
    ln -s ../bin/kmod sbin/modprobe && \
    cp /lib/libz.so.1* lib/ && \
    cp /usr/lib/liblzma.so.5* lib/ && \
    cp /usr/lib/libzstd.so.1* lib/ && \
    cp /lib/libcrypto.so.3 lib/ && \
    cp /lib/ld-musl-*.so.1 lib/

RUN cd /usr/src/initrd && \
    mkdir -p macula/system/macula/${TAG} && \
    cp /usr/src/macula/maculaos macula/system/macula/${TAG} && \
    ln -s ${TAG} macula/system/macula/current && \
    ln -s /macula/system/macula/current/maculaos init

RUN cd /usr/src/initrd && \
    find . | cpio -H newc -o | gzip -9 > /output/initrd
