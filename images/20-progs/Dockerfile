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
RUN gobuild -mod=readonly -o /output/maculaos

FROM gobuild
COPY --from=linuxkit /output/ /output/
COPY --from=macula /output/ /output/
WORKDIR /output
RUN git clone --branch v0.7.0 https://github.com/ahmetb/kubectx.git \
 && chmod -v +x kubectx/kubectx kubectx/kubens
