#!/bin/bash
# Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
#
# Runs inside a golang alpine container, in %post of static.def,
# to build a statically linked, relocatable apptainer installation tarball.
# Use dist/static/build.sh rather than invoking this directly.
#
# Usage: build-in-container.sh packages
#        build-in-container.sh build <srcdir> <outdir> <version>

set -e

PREFIX=/opt/apptainer

install_packages() {
	apk add -q --no-cache bash alpine-sdk linux-headers gawk sed curl file \
		tar autoconf automake libtool pkgconf go-md2man \
		zlib-dev zlib-static xz-dev xz-static zstd-dev zstd-static \
		lz4-dev lz4-static lzo-dev bzip2-dev bzip2-static \
		fuse3-dev fuse3-static libarchive-dev libarchive-static \
		acl-dev acl-static expat-dev expat-static \
		openssl-dev openssl-libs-static talloc-dev talloc-static \
		bsd-compat-headers protobuf-c-dev libseccomp-dev libseccomp-static
}

build() {
	src=$1
	out=$2
	version=$3
	work=$(mktemp -d)

	# Work on a copy, so that the source tree is not modified
	mkdir "${work}/src" "${work}/deps"
	tar -C "${src}" --exclude=./.git --exclude=./builddir \
		--exclude='./apptainer-*.tar.gz' -cf - . | tar -C "${work}/src" -xf -
	cd "${work}/src"
	echo "${version}" >VERSION

	# The bundled dependencies, with versions taken from the rpm spec
	scripts/download-dependencies "${work}/deps"
	(cd "${work}/deps" && "${work}/src/scripts/compile-dependencies" -s)

	./mconfig -s -S --without-suid --without-libsubid \
		-b "${work}/builddir" -p "${PREFIX}"
	make -C "${work}/builddir" -j"$(nproc)"
	make -C "${work}/builddir" install DESTDIR="${work}/out"

	root="${work}/out${PREFIX}"
	(cd "${work}/deps" && "${work}/src/scripts/install-dependencies" "${root}/libexec")

	# offsetpreload.so cannot be preloaded into the static fuse2fs, which
	# does not need it. Every remaining executable must be static.
	rm -rf "${root}/libexec/apptainer/lib"
	find "${root}" -type f -perm -u+x | while read -r f; do
		if ! file "$f" | grep -q ELF; then
			continue
		fi
		if ! file "$f" | grep -Eq "(statically|static-pie) linked"; then
			echo "error: $f is not statically linked" >&2
			exit 1
		fi
		strip "$f"
	done

	arch=$(go env GOARCH)
	mkdir -p "${out}"
	mv "${root}" "${work}/apptainer-${version}"
	tar --numeric-owner --owner=0 --group=0 -C "${work}" \
		-czf "${out}/apptainer-${version}-linux-${arch}.tar.gz" \
		"apptainer-${version}"
	echo "Wrote ${out}/apptainer-${version}-linux-${arch}.tar.gz"
	rm -rf "${work}"
}

case "$1" in
packages)
	install_packages
	;;
build)
	if [ $# -ne 4 ]; then
		echo "Usage: $0 build <srcdir> <outdir> <version>" >&2
		exit 2
	fi
	build "$2" "$3" "$4"
	;;
*)
	echo "Usage: $0 packages|build" >&2
	exit 2
	;;
esac
