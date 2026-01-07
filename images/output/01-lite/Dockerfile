ARG REPO
ARG TAG
FROM ${REPO}/macula-tar:${TAG} as tar

ARG REPO
ARG TAG
FROM ${REPO}/macula-base:${TAG}
ARG ARCH

COPY --from=tar /output/userspace.tar /output/macula-rootfs-${ARCH}.tar
RUN gzip /output/macula-rootfs-${ARCH}.tar
