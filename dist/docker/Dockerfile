ARG BASE_IMAGE=debian:bookworm-slim
ARG GOLANG_IMAGE=golang:1.21.5-bookworm

FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} as build

ARG SYSROOT_BUILD=/sysroot-build
ARG TARGETPLATFORM
ARG VERSION=dev

COPY dist/docker/buildenv.sh /buildenv.sh

RUN . /buildenv.sh && \
    dpkg --add-architecture ${TARGETARCH} && \
    apt-get -y update && \
    apt-get install -y crossbuild-essential-${TARGETARCH} findutils make \
    pkg-config libseccomp2:${TARGETARCH} libseccomp-dev:${TARGETARCH} libc6-dev:${TARGETARCH}

COPY --from=debian-target / ${SYSROOT_BUILD}

RUN . /buildenv.sh && \
    mkdir /download && cd /download && chown _apt:root /download && \
    apt-get -y -o Dir=${SYSROOT_BUILD} update && apt-get download -y \
    $(apt-cache depends --recurse --no-recommends --no-suggests \
    --no-conflicts --no-breaks --no-replaces --no-enhances --no-pre-depends \
    libc6-dev:${TARGETARCH} linux-libc-dev:${TARGETARCH} pkg-config:${TARGETARCH} \
    libseccomp2:${TARGETARCH} libseccomp-dev:${TARGETARCH} | \
    grep "^\w" | sort | uniq | grep "${GREP_TARGETARCH}") \
    libtirpc-common && \
    dpkg --force-architecture --root ${SYSROOT_BUILD} -i *.deb && cd / && rm -rf /download

WORKDIR /build
COPY . /build

RUN . /buildenv.sh && \
    echo ${VERSION} > VERSION && \
    export PKG_CONFIG_PATH= \
    export PKG_CONFIG_SYSROOT_DIR=${SYSROOT_BUILD} \
    export PKG_CONFIG_LIBDIR=${SYSROOT_BUILD}/usr/lib/${DEBIANARCH}-linux-gnu/pkgconfig \
    export CC=${DEBIANARCH}-linux-gnu-gcc && \
    export CFLAGS="--sysroot=${SYSROOT_BUILD}" && \
    export CGO_CFLAGS="$(go env CGO_CFLAGS) --sysroot=${SYSROOT_BUILD}" && \
    export CGO_LDFLAGS="$(go env CGO_CFLAGS) --sysroot=${SYSROOT_BUILD}" && \
    export CGO_ENABLED=1 && \
    export GOCACHE=/go-cache && \
    export GOMODCACHE=/gomod-cache && \
    ./mconfig -c ${DEBIANARCH}-linux-gnu-gcc --with-suid --prefix=/opt/apptainer && \
    make -C builddir && make -C builddir install

FROM --platform=${TARGETPLATFORM} ${BASE_IMAGE} as debian-target
FROM --platform=${BUILDPLATFORM} ${BASE_IMAGE} AS rootfs

ARG SYSROOT=/sysroot
ARG TARGETPLATFORM

COPY --from=debian-target / ${SYSROOT}
COPY dist/docker/buildenv.sh /buildenv.sh

RUN . /buildenv.sh && \
    dpkg --add-architecture ${TARGETARCH} && apt-get -y update && \
    mkdir /download && cd download && chown _apt:root /download && \
    apt-get -y -o Dir=${SYSROOT} update && apt-get download -y \
    $(apt-cache depends --recurse --no-recommends --no-suggests \
    --no-conflicts --no-breaks --no-replaces --no-enhances --no-pre-depends \
    libseccomp2:${TARGETARCH} squashfs-tools:${TARGETARCH} cryptsetup:${TARGETARCH} \
    libfuse3-3:${TARGETARCH} squashfuse:${TARGETARCH} fuse2fs:${TARGETARCH} \
    fuse-overlayfs:${TARGETARCH} fakeroot:${TARGETARCH} openssl:${TARGETARCH} | \
    grep "^\w" | sort | uniq | grep "${GREP_TARGETARCH}") \
    readline-common ca-certificates && \
    dpkg --force-architecture --root ${SYSROOT} -i *.deb && cd / && rm -rf /download && \
    ln -s /opt/apptainer/bin/apptainer ${SYSROOT}/usr/bin/apptainer

FROM scratch

COPY --from=rootfs /sysroot /
COPY --from=build /opt/apptainer /opt/apptainer

CMD ["/usr/bin/apptainer"]
