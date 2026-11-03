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
		bsd-compat-headers protobuf-c-dev
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

	# The bundled dependencies, with versions taken from the rpm spec.
	# A pkg-config wrapper makes builds that call it directly link
	# against all the static libraries they need.
	scripts/download-dependencies "${work}/deps"
	# compile-dependencies applies <package>-*.patch files. PRoot calls
	# basename(3) without including libgen.h, which musl requires.
	cat >"${work}/deps/PRoot-musl-basename.patch" <<'EOF'
--- a/src/cli/cli.c
+++ b/src/cli/cli.c
@@ -24,6 +24,9 @@
 #include <stdbool.h>       /* bool, true, false,  */
 #include <linux/limits.h>  /* ARG_MAX, PATH_MAX, */
 #include <string.h>        /* str*(3), basename(3),  */
+#if !defined(__GLIBC__)
+#include <libgen.h>        /* basename(3), */
+#endif
 #include <talloc.h>        /* talloc*,  */
 #include <stdlib.h>        /* exit(3), EXIT_*, strtol(3), {g,s}etenv(3), */
 #include <assert.h>        /* assert(3),  */
EOF
	mkdir "${work}/bin"
	cat >"${work}/bin/pkg-config" <<'EOF'
#!/bin/sh
exec /usr/bin/pkg-config --static "$@"
EOF
	chmod +x "${work}/bin/pkg-config"
	(
		cd "${work}/deps"
		export PATH="${work}/bin:${PATH}"
		export LDFLAGS=-static EXTRA_LDFLAGS=-static
		"${work}/src/scripts/compile-dependencies"
		# libtool ignores -static for programs, so relink squashfuse_ll
		# with its own flag for a fully static binary
		cd squashfuse-*/
		rm -f squashfuse_ll
		make squashfuse_ll LDFLAGS=-all-static
	)

	./mconfig -s -S --without-suid --without-seccomp --without-libsubid \
		-b "${work}/builddir" -p "${PREFIX}"
	make -C "${work}/builddir" -j"$(nproc)"
	make -C "${work}/builddir" install DESTDIR="${work}/out"

	root="${work}/out${PREFIX}"
	(cd "${work}/deps" && "${work}/src/scripts/install-dependencies" "${root}/libexec")

	# offsetpreload.so is preloaded into host programs, which a musl build
	# cannot be, so it is dropped; the static fuse2fs handles offsets
	# itself. Every remaining executable must be static.
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
