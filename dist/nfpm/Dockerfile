FROM --platform=$TARGETPLATFORM golang:1.17-buster AS build-base
ARG TARGETPLATFORM
RUN apt-get update -y && apt-get install -y pkg-config libseccomp-dev libseccomp2
RUN GOBIN=/bin go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.10.0
COPY . /apptainer
WORKDIR /apptainer

FROM build-base AS build
ARG VERSION
ARG NAME
ARG PREFIX
WORKDIR /apptainer
RUN echo $VERSION > VERSION && \
    ./mconfig --prefix=$PREFIX && \
    make -C builddir && \
    chmod 0644 ./etc/network/* && \
    chmod 0755 ./builddir/cni/*
RUN go run ./dist/nfpm/generate.go -version $VERSION -name $NAME -prefix $PREFIX | \
    nfpm package -f /dev/stdin -p rpm -t ./builddir
RUN go run ./dist/nfpm/generate.go -version $VERSION -name $NAME -prefix $PREFIX | \
    nfpm package -f /dev/stdin -p deb -t ./builddir

FROM build-base AS build-rootless
ARG VERSION
ARG NAME
ARG PREFIX
WORKDIR /apptainer
RUN echo $VERSION > VERSION && \
    ./mconfig --prefix=$PREFIX --without-suid && \
    make -C builddir && \
    chmod 0644 ./etc/network/* && \
    chmod 0755 ./builddir/cni/*
RUN go run ./dist/nfpm/generate.go -version $VERSION -name $NAME-rootless -prefix $PREFIX -rootless | \
    nfpm package -f /dev/stdin -p rpm -t ./builddir
RUN go run ./dist/nfpm/generate.go -version $VERSION -name $NAME-rootless -prefix $PREFIX -rootless | \
    nfpm package -f /dev/stdin -p deb -t ./builddir

FROM scratch as build-packages
COPY --from=build /apptainer/builddir/*.rpm /
COPY --from=build /apptainer/builddir/*.deb /
COPY --from=build-rootless /apptainer/builddir/*.rpm /
COPY --from=build-rootless /apptainer/builddir/*.deb /
