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

usage()
{
	(
	echo "Usage: install-unprivileged.sh [-d dist] [-a arch] [-v version] [-o ] installpath"
	echo "Installs apptainer version and its dependencies from EPEL+baseOS"
	echo "   or Fedora."
	echo " dist can start with el or fc and ends with the major version number,"
	echo "   e.g. el9 or fc37, default based on the current system."
	echo "   As a convenience, active debian and ubuntu versions (not counting 18.04)"
	echo "   get translated into a compatible el version for downloads."
	echo "   OpenSUSE based distributions are also mapped to el, or native openSUSE"
	echo "   binaries can be used via the -o switch."
	echo " arch can be any architecture supported by EPEL or Fedora,"
	echo "   default based on the current system."
	echo " version selects a specific apptainer version, default latest release,"
	echo "   although if it ends in '.rpm' then apptainer will come from there."
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
while true; do
	case "$1" in
		-d) DIST="$2"; shift 2;;
		-a) ARCH="$2"; shift 2;;
		-v) VERSION="$2"; shift 2;;
		-o) NOOPENSUSE="false"; shift 1;;
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

simple_dist() {
source /etc/os-release
   	case " $ID $ID_LIKE " in
		*" rhel "*) echo "el${VERSION_ID%.*}";;
		*" fedora "*) echo "fc${VERSION_ID%.*}";;
		# ubuntu has to be before debian because it has
		# ID_LIKE=debian
		*" ubuntu "*) echo "ubuntu${VERSION_ID%.*}";;
		# testing & unstable debian have no VERSION_ID
		*" debian "*) echo "debian${VERSION_ID%.*}";;
   		# tumbleweed is  rolling release so a extra entry for it
   	 	*" opensuse-tumbleweed"*) echo "opensuse-tumbleweed";;
		*" suse "*) echo "suse${VERSION_ID%.*}";;
		*" sles "*) echo "suse${VERSION_ID%.*}";;
    esac
}

if [ -z "$DIST" ]; then
	if [ ! -f /etc/os-release ]; then
		fatal "There's no /etc/os-release so cannot determine distribution"
	fi
	# shellcheck disable=SC2311
	DIST=$(simple_dist)
	if [ -z "$DIST" ]; then
		fatal "Operating system in /etc/os-release not supported"
	fi
fi

if [ -z "$ARCH" ]; then
	ARCH="$(arch)"
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
OSSPLIT=true

case $DIST in
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
	    EL="${DIST#el}"
	    if [ $EL = 8 ] && { [ "$ARCH" = s390x ] || [ "$ARCH" = ppc64le ] ; } ; then
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
	    FC="${DIST#fc}"
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
	*) fatal "$DIST distribution not supported";;
esac

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
		# optimization: re-use last list if it hasn't changed
		LASTURL="$URL"
		LASTPKGS="$(curl -Ls "$URL")"
	elif [ $RETRY -gt 0 ]; then
		LASTPKGS="$(curl -Ls "$URL")"
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
		latesturl "$URL" "$2" false "$4"
		return $?
	else
		RETRY=0
		echo "$URL"
		return 1
	fi
}

LOCALAPPTAINER=false
if [ -z "$VERSION" ] || ! $NOOPENSUSE; then
	# shellcheck disable=SC2310,SC2311
	if ! APPTAINERURL="$(latesturl "$EPELREPOURL" apptainer $NOOPENSUSE false)"; then
		fatal "Could not find apptainer version from $APPTAINERURL"
	fi
elif [[ "$VERSION" == *.rpm ]]; then
	# use a local rpm
	if [ ! -f "$VERSION" ]; then
		fatal "$VERSION not found"
	fi
	LOCALAPPTAINER=true
	if [[ "$VERSION" != /* ]]; then
		# not a complete path
		VERSION="$PWD/$VERSION"
	fi
else
	KOJIURL="https://kojipkgs.fedoraproject.org/packages/apptainer/$VERSION"
	REL="$(curl -Ls "$KOJIURL"|sed 's/.*href="//;s/".*//'|grep "\.$DIST/"|tail -1|sed 's,/$,,')"
	if [ -z "$REL" ]; then
		fatal "Could not find latest release in $KOJIURL"
	fi
	APPTAINERURL="$KOJIURL/$REL/$ARCH/apptainer-$VERSION-$REL.$ARCH.rpm"
fi

cd "$DEST"
mkdir "$ARCH"
cd "$ARCH"
if $LOCALAPPTAINER; then
	echo "Extracting $VERSION"
	if ! (rpm2cpio "$VERSION"|cpio -idum); then
		fatal "Error extracting $VERSION"
	fi
else
	echo "Extracting $APPTAINERURL"
	if ! (curl -Ls "$APPTAINERURL"|rpm2cpio -|cpio -idum); then
		fatal "Error extracting $APPTAINERURL"
	fi
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

mkdir tmp
cd tmp

APPTAINERURL="https://kojipkgs.fedoraproject.org/packages/apptainer/"
OSUTILS=""
EXTRASUTILS="fuse-overlayfs"
EPELUTILS="fakeroot"
if [ "$DIST" = el7 ]; then
	OSUTILS="$OSUTILS lzo libseccomp squashfs-tools fuse-libs fuse"
	EPELUTILS="$EPELUTILS libzstd fuse2fs fuse3-libs fakeroot-libs"
elif [ "$DIST" = el8 ]; then
	OSUTILS="$OSUTILS lzo libseccomp squashfs-tools fuse-libs fuse libzstd e2fsprogs-libs e2fsprogs fuse3-libs"
	EPELUTILS="$EPELUTILS fakeroot-libs"
elif [ "$DIST" = "opensuse-tumbleweed" ]; then
	OSUTILS="$OSUTILS libseccomp2 squashfs liblzo2-2 libzstd1 e2fsprogs fuse3 libfuse3-3 fakeroot fuse2fs $EXTRASUTILS $EPELUTILS"
	EXTRASUTILS=""
	EPELUTILS=""
elif [ "$DIST" = "suse15" ]; then
	OSUTILS="$OSUTILS libseccomp2 squashfs liblzo2-2 libzstd1 e2fsprogs fuse3 libfuse3-3 fakeroot $EPELUTILS"
	EXTRASUTILS="$EXTRASUTILS fuse2fs"
	EPELUTILS=""
else
	OSUTILS="$OSUTILS lzo libseccomp squashfs-tools fuse-libs fuse libzstd e2fsprogs-libs e2fsprogs"
	EXTRASUTILS="$EXTRASUTILS fuse3-libs"
	EPELUTILS="$EPELUTILS fakeroot-libs"
fi
FEDORA=false
if [[ "$DIST" == fc* ]]; then
	OSUTILS="$OSUTILS $EXTRASUTILS $EPELUTILS"
	EXTRASUTILS=""
	EPELUTILS=""
	FEDORA=true
fi

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
		echo "Extracting $URL"
		if ! (curl -Ls "$URL"|rpm2cpio -|cpio -idum); then
			fatal "failure extracting $URL"
		fi
	done
}
# shellcheck disable=SC2086
extractutils "$OSREPOURL" "$OSSPLIT" $OSUTILS
extractutils "$EXTRASREPOURL" "$OSSPLIT" $EXTRASUTILS
extractutils "$EPELREPOURL" true $EPELUTILS

echo "Patching fakeroot-sysv to make it relocatable"
# shellcheck disable=SC2016
if ! sed -i -e 's,^FAKEROOT_PREFIX=/.*,FAKEROOT_BINDIR=${0%/*},' \
	-e 's,FAKEROOT_BINDIR=/.*,FAKEROOT_PREFIX=${FAKEROOT_BINDIR%/*},' \
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
mv tmp/usr/*bin/fuse* tmp/usr/*bin/*squashfs utils/libexec
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
LD_LIBRARY_PATH=$GGPARENT/utils/lib PATH=$PATH:$GGPARENT/utils/bin ${_WRAPPER_EXEC_CMD:-exec -a "$ARG0"} $REALME "$@"
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
PATH=$PATH:$APPTDIR/utils/bin LD_LIBRARY_PATH="${LD_LIBRARY_PATH}$APPTDIR/utils/lib" exec $APPTDIR/bin/apptainer "$@"
!EOF!

chmod +x bin/apptainer
ln -sf apptainer bin/singularity
echo "Installation complete in $DEST"
