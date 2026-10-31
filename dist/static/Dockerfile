# Builds a statically linked, non-SUID apptainer installation and packs it
# into a relocatable tarball. Use dist/static/build.sh rather than invoking
# this directly.

ARG GOLANG_VERSION="pass-in-as-build-arg"

FROM golang:${GOLANG_VERSION}-alpine AS build

ARG TARGETARCH
ARG VERSION=dev
ARG PREFIX=/opt/apptainer
ARG SQUASHFS_TOOLS_VERSION=4.7.2
ARG SQUASHFS_TOOLS_SHA256=4672b5c47d9418d3a5ae5b243defc6d9eae8275b9771022247c6a6082c815914

RUN apk add -q --no-cache bash alpine-sdk linux-headers gawk sed curl file \
    zlib-dev zlib-static xz-dev xz-static zstd-dev zstd-static \
    lz4-dev lz4-static

# squashfs-tools, so that "apptainer pull/build" can create SIF images
RUN curl -fsSL -o /tmp/sqfs.tar.gz \
    "https://github.com/plougher/squashfs-tools/archive/refs/tags/${SQUASHFS_TOOLS_VERSION}.tar.gz" && \
    echo "${SQUASHFS_TOOLS_SHA256}  /tmp/sqfs.tar.gz" | sha256sum -c - && \
    mkdir /tmp/sqfs && tar xzf /tmp/sqfs.tar.gz -C /tmp/sqfs --strip-components=1 && \
    make -C /tmp/sqfs/squashfs-tools -j"$(nproc)" LZO_SUPPORT=0 \
    EXTRA_LDFLAGS=-static mksquashfs unsquashfs

WORKDIR /build
COPY . /build

RUN echo "${VERSION}" > VERSION && \
    ./mconfig -s -S --without-suid --without-seccomp --without-libsubid \
    -b /builddir -p "${PREFIX}" && \
    make -C /builddir -j"$(nproc)" && \
    make -C /builddir install DESTDIR=/out

# offsetpreload.so is preloaded into host programs, which a musl build
# cannot be, so it is dropped. Every remaining executable must be static.
RUN set -e; \
    root="/out${PREFIX}"; \
    install -m 0755 /tmp/sqfs/squashfs-tools/mksquashfs \
    /tmp/sqfs/squashfs-tools/unsquashfs "${root}/libexec/apptainer/bin/"; \
    rm -rf "${root}/libexec/apptainer/lib"; \
    find "${root}" -type f -perm -u+x | while read -r f; do \
    if ! file "$f" | grep -q ELF; then continue; fi; \
    if ! file "$f" | grep -Eq "(statically|static-pie) linked"; then \
    echo "error: $f is not statically linked" >&2; exit 1; fi; \
    strip "$f"; done; \
    mkdir /dist; \
    mv "${root}" "/dist/apptainer-${VERSION}"; \
    tar --numeric-owner --owner=0 --group=0 -C /dist \
    -czf "/apptainer-${VERSION}-linux-${TARGETARCH}.tar.gz" "apptainer-${VERSION}"

FROM scratch
COPY --from=build /apptainer-*.tar.gz /
