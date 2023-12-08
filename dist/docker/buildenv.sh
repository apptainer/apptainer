#!/bin/sh

export DEBIAN_FRONTEND=noninteractive
export GOOS=linux

echo "Target platform: ${TARGETPLATFORM}"

case "${TARGETPLATFORM##*/}" in
"amd64")
    export TARGETARCH=amd64
    export DEBIANARCH=x86_64
    export GOARCH=amd64
    ;;
"arm64")
    export TARGETARCH=arm64
    export DEBIANARCH=aarch64
    export GOARCH=arm64
    ;;
"ppc64le")
    export TARGETARCH=ppc64el
    export DEBIANARCH=powerpc64le
    export GOARCH=ppc64le
    ;;
"s390x")
    export TARGETARCH=s390x
    export DEBIANARCH=s390x
    export GOARCH=s390x
    ;;
*)
    echo "${TARGETPLATFORM##*/} not supported, see dist/docker/build.sh to add it"
    exit 1
    ;;
esac

BUILDARCH=$(dpkg --print-architecture)
export GREP_TARGETARCH=$(test ${TARGETARCH} = ${BUILDARCH} || echo ${TARGETARCH})
