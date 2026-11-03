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
#
# The build runs with apptainer in %post of static.def, in a golang alpine
# container.

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
	# The build happens in %post; the sandbox it leaves is not needed
	sandbox=$(mktemp -d)
	# The go module cache in the sandbox is read-only
	trap 'chmod -R u+w "${sandbox}"; rm -rf "${sandbox}"' EXIT
	apptainer build --fakeroot --fix-perms --force --sandbox \
		--arch "${arch}" \
		--bind "${toplevel}:/src" \
		--build-arg "GOLANG_VERSION=${golang_version}" \
		--build-arg "VERSION=${version}" \
		"${sandbox}" "${here}/static.def"
	chmod -R u+w "${sandbox}"
	rm -rf "${sandbox}"
	trap - EXIT
	echo "Wrote ${toplevel}/apptainer-${version}-linux-${arch}.tar.gz"
done
