ARG REPO
ARG TAG
FROM ${REPO}/macula-kernel:${TAG} as kernel

ARG REPO
ARG TAG
FROM ${REPO}/macula-package:${TAG}
ARG VERSION

COPY --from=kernel /output/ /output/macula/system/kernel/

WORKDIR /output/macula/system/kernel
RUN mkdir -vp $(cat version)
RUN ln -sf $(cat version) current
RUN mv -vf initrd kernel.squashfs current/
RUN rm -vf version vmlinuz
