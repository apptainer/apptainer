#!/bin/bash
## Copyright (c) Contributors to the Apptainer project, established as
#   Apptainer a Series of LF Projects LLC.
#   For website terms of use, trademark policy, privacy policy and other
#   project policies see https://lfprojects.org/policies
#
# This installs relocatable released versions of unprivileged apptainer 
# and all dependencies for a single architecture, from either Fedora or
# EPEL+CentOS7 or EPEL+RockyLinux (or AlmaLinux) distributions.  Supports
# the architectures that are available in those distributions.

# Works with apptainer versions 1.1.4 and higher.

REQUIREDCMDS="curl rpm2cpio cpio"

# assumes at least a 1 Mbit connection
CURLOPTS="--connect-timeout 5 -Y 125000 -y 10 -Ls"

usage()
{
	(
	echo "Usage: install-unprivileged.sh [-e] [-d dist] [-o] [-a arch] [-v version] installpath"
	echo
	echo "Installs an apptainer version and its dependencies from EPEL+baseOS or Fedora."
	echo "By default packages are installed from the oldest supported EPEL distribution"
	echo "   for the architecture.  That usually works best for any distribution"
	echo "   including when building containers for any distribution."
	echo "If that is a problem you can explicitly select a distribution with the -d"
	echo "   option, where dist can start with el or fc and end with a major version"
	echo "   number, e.g. el9 or fc37. If dist begins with debian, ubuntu, or suse"
	echo "   followed by a version number, it will be translated to the most comparable"
	echo "   el version, except that if -o is used with a dist beginning with suse,"
	echo "   suse binaries will be used when available."
	echo " arch can be any architecture supported by EPEL or Fedora, default based on"
	echo "   the current system."
	echo " version selects a specific apptainer version, default latest release, and"
	echo "   if it ends in '.rpm' then apptainer will come from a file of that name."
	echo "The -e option shows download errors and retries which are otherwise hidden."
	) >&2
	exit 1
}

fatal()
{
	if [ "$#" != 0 ]; then
		echo "$*" >&2
	fi
	exit 2
}

DIST=""
ARCH=""
VERSION=""
NOOPENSUSE=true
SHOWERRORS=false
while true; do
	case "$1" in
		-d) DIST="$2"; shift 2;;
		-a) ARCH="$2"; shift 2;;
		-v) VERSION="$2"; shift 2;;
		-o) NOOPENSUSE="false"; shift 1;;
		-e) SHOWERRORS="true"
		    CURLOPTS="$CURLOPTS -S"
		    shift 1;;
		-*) usage;;
		*) break;;
	esac
done

if [ $# != 1 ]; then
	usage
fi

MISSINGCMDS=""
for CMD in $REQUIREDCMDS; do 
	if [ -z "$(type -p "$CMD")" ]; then
		MISSINGCMDS="$MISSINGCMDS $CMD"
	fi
done
if [ -n "$MISSINGCMDS" ]; then
	fatal "Required command(s)$MISSINGCMDS not found"
fi

if [ -z "$ARCH" ]; then
	ARCH="$(arch)"
fi

if [ -z "$DIST" ]; then
	DIST=el8
fi

if ! $NOOPENSUSE && [[ "$DIST" != *suse* ]]; then
	fatal "The -o option is only relevant to suse-based distributions"
fi

DEST="$1"
if ! mkdir -p "$DEST"; then
	fatal "Error making $DEST"
fi

if [ ! -w "$DEST" ]; then
	fatal "$DEST is not writable"
fi

if [ -d "$DEST/$ARCH" ]; then
	fatal "$DEST/$ARCH is not empty"
fi

case "$DIST" in
	ubuntu1*) fatal "$DIST not supported, sorry";;
	ubuntu20*|debian10*|debian11*) DIST=el8;;
	ubuntu*|debian1*|debian) DIST=el9;;
	el*|fc*);;
	opensuse-tumbleweed)
		if $NOOPENSUSE; then
			DIST=el9
		fi
	;;
	suse15)
		if $NOOPENSUSE; then
			DIST=el8
		fi
	;;
	suse12) DIST=el7;;
	*)	fatal "Unrecognized distribution $DIST"
esac

OSREPOURL=""
EXTRASREPOURL=""
EPELREPOURL=""

setrepourls()
{
OSSPLIT=true
case $1 in
	el7)
	    OSREPOURL="https://linux-mirrors.fnal.gov/linux/centos/7/os/$ARCH/Packages"
	    # there's no extra directory level with the first name of the
	    OSSPLIT=false
	    EXTRASREPOURL="https://linux-mirrors.fnal.gov/linux/centos/7/extras/$ARCH/Packages"
	    # download.fedoraproject.org is preferred but unreliable for epel7
	    # The extras depend on the fnal.gov mirror anyway so use that again
	    EPELREPOURL="https://linux-mirrors.fnal.gov/linux/fedora/epel/7/$ARCH/Packages"
	    ;;
	el*) 
	    EL="${1#el}"
	    if [ "$EL" = 8 ] && { [ "$ARCH" = s390x ] || [ "$ARCH" = ppc64le ] ; } ; then
		# these archs not yet available on rocky8
		OSREPOURL="https://repo.almalinux.org/almalinux/$EL/BaseOS/$ARCH/os/Packages"
		OSSPLIT=false
		EXTRASREPOURL="https://repo.almalinux.org/almalinux/$EL/AppStream/$ARCH/os/Packages"
	    else
		OSREPOURL="https://dl.rockylinux.org/pub/rocky/$EL/BaseOS/$ARCH/os/Packages"
		EXTRASREPOURL="https://dl.rockylinux.org/pub/rocky/$EL/AppStream/$ARCH/os/Packages"
	    fi
	    EPELREPOURL="https://download.fedoraproject.org/pub/epel/$EL/Everything/$ARCH/Packages"
	    ;;
	fc*)
	    FC="${1#fc}"
	    OSREPOURL="https://dl.fedoraproject.org/pub/fedora/linux/updates/$FC/Everything/$ARCH/Packages/"
	    EPELREPOURL="https://dl.fedoraproject.org/pub/fedora/linux/updates$TESTING/$FC/Everything/$ARCH/Packages/"
	    ;;
	opensuse-tumbleweed)
	    OSREPOURL="https://download.opensuse.org/tumbleweed/repo/oss/$ARCH"
	    EPELREPOURL="https://download.opensuse.org/repositories/network:/cluster/openSUSE_Tumbleweed/$ARCH"
	    EXTRASREPOURL=$OSREPOURL
	    OSSPLIT=false
	    ;;
	suse15)
	    OSREPOURL="https://download.opensuse.org/distribution/leap/15.4/repo/oss/$ARCH"
	    EPELREPOURL="https://download.opensuse.org/repositories/network:/cluster/15.4/$ARCH"
	    EXTRASREPOURL="https://download.opensuse.org/repositories/filesystems/15.4/$ARCH"
	    OSSPLIT=false
	    ;;
	*) fatal "$1 distribution not supported";;
esac
}
setrepourls "$DIST"

# $1 -- base URL
# $2 -- package name
# $3 -- if true, add first character of package to the end of the base URL
# $4 -- if true, try replacing "/updates/" with "/releases/" if nothing found
# If return value 0, succeeded and stdout contains latest url
# If return value not zero, failed and stdout contains final directory url
# If a package is not found, the listing will be silently retried up to 3 times,
# because sometimes not all mirrors are up to date
LASTURL=""
LASTPKGS=""
RETRY=0
latesturl()
{
	typeset URL="$1"
	if [ "$3" = true ]; then
		URL="${URL%/}"
		URL="$URL/${2:0:1}"
	fi
	if [ "$URL" != "$LASTURL" ]; then
		# optimization: reuse last list if it hasn't changed
		LASTURL="$URL"
		LASTPKGS="$(curl $CURLOPTS "$URL")"
	elif [ $RETRY -gt 0 ]; then
		LASTPKGS="$(curl $CURLOPTS "$URL")"
	fi
	typeset LATEST="$(echo "$LASTPKGS"|sed -e 's/.*href="//;s/".*//' -e 's/\.mirrorlist//' -e 's/\-32bit//' -e 's@^\.\/@@' |grep "^$2-[0-9].*$ARCH"|tail -1)"
	if [ -n "$LATEST" ]; then
		RETRY=0
		echo "$URL/$LATEST"
	elif [ "$4" = true ]; then
		RETRY=0
		URL="${URL/\/updates\///releases/}"
		URL="${URL/\/Packages\///os/Packages/}"
		latesturl "$URL" "$2" false false
		return $?
	elif [ $RETRY -lt 3 ]; then
		RETRY=$((RETRY+1))
		if $SHOWERRORS; then
			echo "Retrying..." >&2
		fi
		latesturl "$URL" "$2" false "$4"
		return $?
	else
		RETRY=0
		echo "$URL"
		return 1
	fi
}

RPMDIST=""
LOCALAPPTAINER=false
if [[ "$VERSION" == *.rpm ]]; then
	# use a local rpm
	if [ ! -f "$VERSION" ]; then
		fatal "$VERSION not found"
	fi
	LOCALAPPTAINER=true
	if [[ "$VERSION" != /* ]]; then
		# not a complete path
		VERSION="$PWD/$VERSION"
	fi
	# take third from last field separated by dots as the rpm dist
	# shellcheck disable=SC2001
	RPMDIST="$(echo "$VERSION"|sed 's/.*\.\(.*\)\.[^.]*\..*$/\1/')"
	# but if it starts with a number, the dist is missing so just try
	# to use the default DIST (which happens if RPMDIST is empty)
	if [[ "$RPMDIST" == [0-9]* ]]; then
		RPMDIST=""
	fi
elif [ -z "$VERSION" ] || ! $NOOPENSUSE; then
	# shellcheck disable=SC2310,SC2311
	if ! APPTAINERURL="$(latesturl "$EPELREPOURL" apptainer $NOOPENSUSE false)"; then
		fatal "Could not find apptainer version from $APPTAINERURL"
	fi
else
	KOJIURL="https://kojipkgs.fedoraproject.org/packages/apptainer/$VERSION"
	REL="$(curl $CURLOPTS "$KOJIURL"|sed 's/.*href="//;s/".*//'|grep "\.$DIST/"|tail -1|sed 's,/$,,')"
	if [ -z "$REL" ]; then
		fatal "Could not find latest release in $KOJIURL"
	fi
	APPTAINERURL="$KOJIURL/$REL/$ARCH/apptainer-$VERSION-$REL.$ARCH.rpm"
fi

# Retry url to 3 times if download fails
extracturl()
{
	typeset URL="$1"
	typeset RETRY=0

	echo "Extracting $URL"
	while true; do
		if curl $CURLOPTS "$URL"|rpm2cpio -|cpio -idum; then
			break
		fi
		RETRY=$((RETRY+1))
		if [ "$RETRY" -ge 3 ]; then
			fatal "Failure extracting $URL"
		fi
		if $SHOWERRORS; then
			echo "Retrying..." >&2
		fi
	done
}

cd "$DEST"
mkdir "$ARCH"
cd "$ARCH"
if $LOCALAPPTAINER; then
	echo "Extracting $VERSION"
	if ! (rpm2cpio "$VERSION"|cpio -idum); then
		fatal "Error extracting $VERSION"
	fi
else
	extracturl "$APPTAINERURL"
fi
if [ ! -f usr/bin/apptainer ]; then
	fatal "Required file usr/bin/apptainer missing"
fi
if ! mv usr/* .; then
	fatal "error renaming usr/*"
fi
if ! rmdir usr; then
	fatal "error removing usr"
fi

OSUTILS=""
NEEDSFUSE2FS=true
if [ -f libexec/apptainer/bin/fuse2fs ]; then
	# apptainer-1.3.0 or newer
	NEEDSFUSE2FS=false
fi
EXTRASUTILS=""
if [ ! -f libexec/apptainer/bin/fuse-overlayfs ]; then
	# older than apptainer-1.3.0
	EXTRASUTILS="fuse-overlayfs"
fi
EPELUTILS="fakeroot"
if [[ "$DIST" != *suse* ]]; then
	EPELUTILS="fakeroot-libs $EPELUTILS"
fi
RPMUTILS=""

mkdir tmp
cd tmp

if [ "$DIST" = el7 ]; then
	OSUTILS="$OSUTILS lzo squashfs-tools"
	EPELUTILS="$EPELUTILS libzstd fuse3-libs"
	if $NEEDSFUSE2FS; then
		OSUTILS="$OSUTILS fuse-libs"
		EPELUTILS="$EPELUTILS fuse2fs"
	fi
elif [ "$DIST" = el8 ]; then
	OSUTILS="$OSUTILS lzo squashfs-tools libzstd fuse3-libs libsepol bzip2-libs audit-libs libcap-ng libattr libacl pcre2 libxcrypt libselinux libsemanage shadow-utils-subid"
	if $NEEDSFUSE2FS; then
		OSUTILS="$OSUTILS fuse-libs e2fsprogs-libs e2fsprogs"
	fi
elif [ "$DIST" = "opensuse-tumbleweed" ]; then
	OSUTILS="$OSUTILS squashfs liblzo2-2 libzstd1 libfuse3-3 $EXTRASUTILS $EPELUTILS"
	if $NEEDSFUSE2FS; then
		OSUTILS="$OSUTILS fuse2fs"
	fi
	EXTRASUTILS=""
	EPELUTILS=""
elif [ "$DIST" = "suse15" ]; then
	OSUTILS="$OSUTILS squashfs liblzo2-2 libzstd1 libfuse3-3 $EPELUTILS"
	if $NEEDSFUSE2FS; then
		EXTRASUTILS="$EXTRASUTILS libext2fs2 libfuse2 fuse2fs"
	fi
	EPELUTILS=""
else
	# el9 & fc*
	OSUTILS="$OSUTILS lzo squashfs-tools libzstd libsepol bzip2-libs audit-libs libcap-ng libattr libacl pcre2 libxcrypt libselinux libsemanage shadow-utils-subid"
	if $NEEDSFUSE2FS; then
		OSUTILS="$OSUTILS fuse-libs e2fsprogs-libs e2fsprogs"
	fi
	EXTRASUTILS="$EXTRASUTILS fuse3-libs"
fi
if [[ "$RPMDIST" == *suse* ]]; then
	RPMUTILS=libseccomp2
elif [ -n "$RPMDIST" ]; then
	# Note that libseccomp also requires libc, so if $RPMDIST is newer than
	# the host's distribution then this can cause GLIBC version errors.
	RPMUTILS=libseccomp
elif [[ "$DIST" == *suse* ]]; then
	OSUTILS="$OSUTILS libseccomp2"
else
	OSUTILS="$OSUTILS libseccomp"
fi
FEDORA=false
if [[ "$DIST" == fc* ]]; then
	OSUTILS="$OSUTILS $EXTRASUTILS $EPELUTILS"
	EXTRASUTILS=""
	EPELUTILS=""
	FEDORA=true
fi

# Extract rpms from given repo url with a given split type
extractutils()
{
	typeset REPOURL="$1"
	typeset SPLIT="$2"
	shift 2
	for PKG; do
		# shellcheck disable=SC2310,SC2311
		if ! URL="$(latesturl "$REPOURL" "$PKG" "$SPLIT" "$FEDORA")"; then
			fatal "$PKG not found in $URL"
		fi
		extracturl "$URL"
	done
}
# shellcheck disable=SC2086
extractutils "$OSREPOURL" "$OSSPLIT" $OSUTILS
extractutils "$EXTRASREPOURL" "$OSSPLIT" $EXTRASUTILS
extractutils "$EPELREPOURL" true $EPELUTILS
if [ -n "$RPMDIST" ]; then
	setrepourls "$RPMDIST"
	extractutils "$OSREPOURL" "$OSSPLIT" $RPMUTILS
fi

echo "Patching fakeroot-sysv to make it relocatable"
# shellcheck disable=SC2016
if ! sed -i -e 's,^FAKEROOT_PREFIX=/.*,FAKEROOT_BINDIR=${0%/*},' \
	-e 's,FAKEROOT_BINDIR=/.*,FAKEROOT_PREFIX=${FAKEROOT_BINDIR%/*},' \
	-e 's,^PATHS=/usr/lib[^/]*/libfakeroot:,PATHS=,' \
	usr/bin/fakeroot-sysv
then
	fatal "failure patching fakeroot-sysv"
fi

cd ..

set -e

# remove .build-id files seen on el8 or later
rm -rf lib/.build-id
rmdir lib 2>/dev/null || true

# make lib and libexec equivalent for openSUSE binaries
[ -e lib/apptainer ] && (mkdir -p libexec ; ln -s ../lib/apptainer libexec/apptainer )

# move everything needed out of tmp to utils
mkdir -p utils/bin utils/lib utils/libexec
mv tmp/usr/lib*/* utils/lib
mv tmp/lib*/* utils/lib 2>/dev/null || true # optional
mv tmp/usr/*bin/*squashfs utils/libexec
mv tmp/usr/*bin/fuse* utils/libexec 2>/dev/null || true # optional
mv tmp/usr/bin/fake*sysv utils/bin
cat >utils/bin/.wrapper <<'!EOF!'
#!/bin/bash
BASEME=${0##*/}
HERE="${0%/*}"
if [ "$HERE" = "." ]; then
	HERE="$PWD"
elif [[ "$HERE" != /* ]]; then
	HERE="$PWD/$HERE"
fi
PARENT="${HERE%/*}"
#_WRAPPER_EXEC_CMD and _WRAPPER_ARG0 are sometimes used by apptainer
REALME=$PARENT/libexec/$BASEME
ARG0="${_WRAPPER_ARG0:-$REALME}"
LD_LIBRARY_PATH=$PARENT/lib ${_WRAPPER_EXEC_CMD:-exec -a "$ARG0"} $REALME "$@"
!EOF!
chmod +x utils/bin/.wrapper
for TOOL in utils/libexec/*; do
	ln -s .wrapper "utils/bin/${TOOL##*/}"
done
rm -rf tmp

# use similar wrapper for libexec/apptainer/bin scripts
mkdir libexec/apptainer/libexec
cat >libexec/apptainer/bin/.wrapper <<'!EOF!'
#!/bin/bash
BASEME=${0##*/}
HERE="${0%/*}"
if [ "$HERE" = "." ]; then
	HERE="$PWD"
elif [[ "$HERE" != /* ]]; then
	HERE="$PWD/$HERE"
fi
PARENT="${HERE%/*}"
GGPARENT="${PARENT%/*/*}"
REALME=$PARENT/libexec/$BASEME
ARG0="${_WRAPPER_ARG0:-$REALME}"
LD_LIBRARY_PATH=$GGPARENT/utils/lib PATH=$GGPARENT/utils/bin:$PATH ${_WRAPPER_EXEC_CMD:-exec -a "$ARG0"} $REALME "$@"
!EOF!
chmod +x libexec/apptainer/bin/.wrapper
for TOOL in libexec/apptainer/bin/*; do
	mv "$TOOL" libexec/apptainer/libexec
	ln -s .wrapper "$TOOL"
done

cd ..
echo "Creating bin/apptainer and bin/singularity"
mkdir -p bin
cat >bin/apptainer <<'!EOF!'
#!/bin/bash
ME="$(/usr/bin/realpath $0)"
HERE="${ME%/*}"
BASEPATH="${HERE%/*}"
ARCH="$(uname -m)"
if [ -z "$ARCH" ]; then
	echo "$0: cannot determine arch" 2>&1
	exit 1
fi
APPTDIR="$BASEPATH/$ARCH"
if [ ! -d "$APPTDIR" ]; then
	echo "$0: $APPTDIR not found" 2>&1
	exit 1
fi
if [ -n "$LD_LIBRARY_PATH" ]; then
	LD_LIBRARY_PATH="$LD_LIBRARY_PATH:"
fi
PATH=$APPTDIR/utils/bin:$PATH LD_LIBRARY_PATH="$LD_LIBRARY_PATH$APPTDIR/utils/lib" exec $APPTDIR/bin/apptainer "$@"
!EOF!

chmod +x bin/apptainer
ln -sf apptainer bin/singularity
echo "Installation complete in $DEST"
