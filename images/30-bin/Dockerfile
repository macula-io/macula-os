ARG REPO
ARG TAG
FROM ${REPO}/macula-rootfs:${TAG} as rootfs

ARG REPO
ARG TAG
FROM ${REPO}/macula-progs:${TAG} as progs

ARG REPO
ARG TAG
FROM ${REPO}/macula-base:${TAG}

COPY --from=rootfs /output/rootfs.squashfs /usr/src/
COPY install.sh /output/maculaos-install.sh
COPY --from=progs /output/maculaos /output/maculaos
RUN echo -n "_sqmagic_" >> /output/maculaos
RUN cat /usr/src/rootfs.squashfs >> /output/maculaos
