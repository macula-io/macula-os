ARG REPO
ARG TAG
FROM ${REPO}/macula-tar:${TAG} as tar

ARG REPO
ARG TAG
FROM ${REPO}/macula-iso:${TAG} as iso

ARG REPO
ARG TAG
FROM ${REPO}/macula-kernel:${TAG} as kernel

ARG REPO
ARG TAG
FROM ${REPO}/macula-base:${TAG}
ARG ARCH

COPY --from=kernel /output/vmlinuz /output/maculaos-vmlinuz-${ARCH}
COPY --from=kernel /output/initrd /output/maculaos-initrd-${ARCH}
COPY --from=kernel /output/kernel.squashfs /output/maculaos-kernel-${ARCH}.squashfs
COPY --from=kernel /output/version /output/maculaos-kernel-version-${ARCH}
COPY --from=iso /output/maculaos.iso /output/maculaos-${ARCH}.iso
COPY --from=tar /output/userspace.tar /output/maculaos-rootfs-${ARCH}.tar
RUN gzip /output/maculaos-rootfs-${ARCH}.tar
