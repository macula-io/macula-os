ARG REPO
ARG TAG
ARG VERSION
FROM ${REPO}/macula-gobuild:${TAG} as gobuild

ENV LINUXKIT v0.8

FROM gobuild as linuxkit
ENV GO111MODULE off
RUN git clone https://github.com/linuxkit/linuxkit.git $GOPATH/src/github.com/linuxkit/linuxkit
WORKDIR $GOPATH/src/github.com/linuxkit/linuxkit/pkg/metadata
RUN git checkout -b current $LINUXKIT
RUN gobuild -o /output/metadata

FROM gobuild as macula
ARG VERSION
COPY go.mod $GOPATH/src/github.com/macula-io/macula-os/
COPY go.sum $GOPATH/src/github.com/macula-io/macula-os/
COPY /pkg/ $GOPATH/src/github.com/macula-io/macula-os/pkg/
COPY /main.go $GOPATH/src/github.com/macula-io/macula-os/
COPY /vendor/ $GOPATH/src/github.com/macula-io/macula-os/vendor/
WORKDIR $GOPATH/src/github.com/macula-io/macula-os
RUN gobuild -mod=vendor -o /output/maculaos

FROM gobuild as firstboot
ARG VERSION
COPY go.mod $GOPATH/src/github.com/macula-io/macula-os/
COPY go.sum $GOPATH/src/github.com/macula-io/macula-os/
COPY /cmd/ $GOPATH/src/github.com/macula-io/macula-os/cmd/
COPY /vendor/ $GOPATH/src/github.com/macula-io/macula-os/vendor/
WORKDIR $GOPATH/src/github.com/macula-io/macula-os
RUN gobuild -mod=vendor -o /output/macula-firstboot ./cmd/macula-firstboot

FROM gobuild as nats
ARG ARCH
# Download NATS server static binary
# NATS uses different arch names: amd64, arm64
RUN NATS_VERSION="2.10.24" && \
    NATS_ARCH=$(echo $ARCH | sed 's/amd64/amd64/;s/arm64/arm64/') && \
    wget -q "https://github.com/nats-io/nats-server/releases/download/v${NATS_VERSION}/nats-server-v${NATS_VERSION}-linux-${NATS_ARCH}.tar.gz" -O /tmp/nats.tar.gz && \
    tar -xzf /tmp/nats.tar.gz -C /tmp && \
    mv /tmp/nats-server-v${NATS_VERSION}-linux-${NATS_ARCH}/nats-server /output/nats-server && \
    chmod +x /output/nats-server

FROM gobuild
COPY --from=linuxkit /output/ /output/
COPY --from=macula /output/ /output/
COPY --from=firstboot /output/ /output/
COPY --from=nats /output/nats-server /output/nats-server
WORKDIR /output
RUN git clone --branch v0.7.0 https://github.com/ahmetb/kubectx.git \
 && chmod -v +x kubectx/kubectx kubectx/kubens
