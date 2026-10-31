#!/bin/sh
# Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
#
# Build tarballs containing a statically linked, relocatable apptainer
# installation, for each architecture given as argument (default: amd64
# arm64). The tarballs are written to the top of the source tree.
#
# Usage: dist/static/build.sh [amd64] [arm64]

set -e

here=$(cd "$(dirname "$0")" && pwd)
toplevel=$(cd "${here}/../.." && pwd)

cd "${toplevel}"

version=$(scripts/get-version)
golang_version=$(scripts/get-min-go-version)

if [ $# -eq 0 ]; then
	set -- amd64 arm64
fi

for arch in "$@"; do
	echo "Building static apptainer ${version} for linux/${arch}"
	docker buildx build \
		--platform "linux/${arch}" \
		--build-arg "GOLANG_VERSION=${golang_version}" \
		--build-arg "VERSION=${version}" \
		--file "${here}/Dockerfile" \
		--output "type=local,dest=${toplevel}" \
		.
	echo "Wrote ${toplevel}/apptainer-${version}-linux-${arch}.tar.gz"
done
