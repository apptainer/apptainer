#!/bin/sh
# Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
#
# Smoke test a static apptainer tarball built by dist/static/build.sh, by
# exercising each of the bundled tools as an unprivileged user.
#
# Usage: dist/static/test.sh <apptainer-version-linux-arch.tar.gz>

set -e

if [ $# -ne 1 ] || [ ! -f "$1" ]; then
	echo "Usage: $0 <tarball>" >&2
	exit 2
fi
tarball=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")

work=$(mktemp -d)
trap 'chmod -R u+w "${work}"; rm -rf "${work}"' EXIT
cd "${work}"
tar -xzf "${tarball}"
apptainer=$(echo "${work}"/apptainer-*/bin/apptainer)
export APPTAINER_CACHEDIR="${work}/cache"

step() {
	echo
	echo "=== $*"
}

step "version"
"${apptainer}" --version

step "every executable is static"
find "${work}"/apptainer-* -type f -perm -u+x | while read -r f; do
	if file "$f" | grep -q ELF && ! file "$f" | grep -Eq "(statically|static-pie) linked"; then
		echo "error: $f is not statically linked" >&2
		exit 1
	fi
done

step "pull and exec a SIF image (mksquashfs, squashfuse_ll)"
"${apptainer}" pull base.sif docker://alpine:3.22
"${apptainer}" exec base.sif cat /etc/alpine-release

step "ext3 overlay inside a SIF (fuse2fs offset)"
cp base.sif overlay.sif
"${apptainer}" overlay create --size 64 overlay.sif
"${apptainer}" exec --writable overlay.sif sh -c 'echo persisted >/persisted'
test "$("${apptainer}" exec overlay.sif cat /persisted)" = persisted

step "directory overlay (fuse-overlayfs)"
mkdir overlay-dir
"${apptainer}" exec --overlay overlay-dir base.sif touch /in-overlay
test -e overlay-dir/upper/in-overlay

step "archive mount (archivemount)"
mkdir -p archive/sub
echo from-archive >archive/sub/file
tar -czf archive.tar.gz -C archive .
test "$("${apptainer}" exec --bind archive.tar.gz:/data:archive-src=/ \
	base.sif cat /data/sub/file)" = from-archive

step "encrypted SIF (gocryptfs)"
openssl genrsa -traditional -out key.pem 2048 2>/dev/null
openssl rsa -in key.pem -RSAPublicKey_out -out key.pub 2>/dev/null
"${apptainer}" build --pem-path=key.pub encrypted.sif base.sif
"${apptainer}" exec --pem-path=key.pem encrypted.sif cat /etc/alpine-release

step "seccomp profile"
cat >deny-mkdir.json <<'EOF'
{"defaultAction":"SCMP_ACT_ALLOW","syscalls":[{"names":["mkdir","mkdirat"],"action":"SCMP_ACT_ERRNO","errnoRet":1}]}
EOF
if "${apptainer}" exec --security "seccomp:${work}/deny-mkdir.json" \
	base.sif mkdir /tmp/denied; then
	echo "error: seccomp profile did not deny mkdir" >&2
	exit 1
fi

step "fakeroot build creating a device node (seccomp)"
cat >mknod.def <<'EOF'
Bootstrap: localimage
From: base.sif

%post
    mknod /dev-null-test c 1 3
EOF
"${apptainer}" build --fakeroot mknod.sif mknod.def

echo
echo "All static apptainer tests passed"
