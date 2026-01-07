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
    mksquashfs . /output/kernel.squashfs

RUN cp /usr/src/kernel/version /output/ && \
    cp /usr/src/kernel/vmlinuz /output/

COPY --from=bin /output/ /usr/src/macula/
RUN cd /usr/src/initrd && \
    mkdir -p macula/system/macula/${TAG} && \
    cp /usr/src/macula/maculaos macula/system/macula/${TAG} && \
    ln -s ${TAG} macula/system/macula/current && \
    ln -s /macula/system/macula/current/maculaos init
    
RUN cd /usr/src/initrd && \
    find . | cpio -H newc -o | gzip -c -1 > /output/initrd
