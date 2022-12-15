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
	echo "Usage: install-unprivileged.sh [-d dist] [-a arch] [-v version] installpath"
	echo "Installs apptainer version and its dependencies from EPEL+baseOS"
	echo "   or Fedora."
	echo " dist can start with el or fc and ends with the major version number,"
	echo "   e.g. el9 or fc37, default based on the current system."
	echo "   As a convenience, active debian and ubuntu versions (not counting 18.04)"
	echo "   get translated into a compatible el version for downloads."
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
while true; do
	case "$1" in
		-d) DIST="$2"; shift 2;;
		-a) ARCH="$2"; shift 2;;
		-v) VERSION="$2"; shift 2;;
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

if [ -z "$DIST" ]; then
	if [ ! -f /etc/os-release ]; then
		fatal "There's no /etc/os-release so cannot determine distribution"
	fi
	DIST="$(source /etc/os-release
        	case " $ID $ID_LIKE " in
			*" rhel "*) echo "el${VERSION_ID%.*}";;
			*" fedora "*) echo "fc${VERSION_ID%.*}";;
			# ubuntu has to be before debian because it has
			# ID_LIKE=debian
			*" ubuntu "*) echo "ubuntu${VERSION_ID%.*}";;
			# testing & unstable debian have no VERSION_ID
			*" debian "*) echo "debian${VERSION_ID%.*}";;
        	esac)"
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
	*) fatal "$DIST distribution not supported";;
esac

# $1 -- base URL
# $2 -- package name
# $3 -- if true, add first character of package to the end of the base URL
# $4 -- if true, try replacing "/updates/" with "/releases/" if nothing found
# If return value 0, succeeded and stdout contains latest url
# If return value not zero, failed and stdout contains final directory url
LASTURL=""
LASTPKGS=""
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
	fi
	typeset LATEST="$(echo "$LASTPKGS"|sed 's/.*href="//;s/".*//'|grep "^$2-[0-9].*$ARCH"|tail -1)"
	if [ -n "$LATEST" ]; then
		echo "$URL/$LATEST"
	elif [ "$4" = true ]; then
		URL="${URL/\/updates\///releases/}"
		URL="${URL/\/Packages\///os/Packages/}"
		latesturl "$URL" "$2" false false
		return $?
	else
		echo "$URL"
		return 1
	fi
}

APPTAINERURL="https://kojipkgs.fedoraproject.org/packages/apptainer/"
LOCALAPPTAINER=false
if [ -z "$VERSION" ]; then
	# shellcheck disable=SC2310,SC2311
	if ! APPTAINERURL="$(latesturl "$EPELREPOURL" apptainer true false)"; then
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
OSUTILS="libseccomp lzo squashfs-tools fuse-libs"
EXTRASUTILS="fuse-overlayfs"
EPELUTILS="fakeroot fakeroot-libs"
if [ "$DIST" = el7 ]; then
	EPELUTILS="$EPELUTILS libzstd fuse2fs fuse3-libs"
elif [ "$DIST" = el8 ]; then
	OSUTILS="$OSUTILS libzstd e2fsprogs-libs e2fsprogs fuse3-libs"
else
	OSUTILS="$OSUTILS libzstd e2fsprogs-libs e2fsprogs"
	EXTRASUTILS="$EXTRASUTILS fuse3-libs"
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

# move everything needed out of tmp to utils
mkdir -p utils/bin utils/lib utils/libexec
mv tmp/usr/lib*/* utils/lib
mv tmp/usr/bin/fuse-overlayfs tmp/usr/*bin/fuse2fs tmp/usr/*bin/*squashfs utils/libexec
mv tmp/usr/bin/fake*sysv utils/bin
cat >utils/bin/.wrapper <<'!EOF!'
#!/bin/bash
ME=${0##*/}
HERE="${0%/*}"
if [ "$HERE" = "." ]; then
	HERE="$PWD"
elif [[ "$HERE" != /* ]]; then
	HERE="$PWD/$HERE"
fi
PARENT="${HERE%/*}"
#_WRAPPER_EXEC_CMD is sometimes used by apptainer
LD_LIBRARY_PATH=$PARENT/lib ${_WRAPPER_EXEC_CMD:-exec} $PARENT/libexec/$ME "$@"
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
ME=${0##*/}
HERE="${0%/*}"
if [ "$HERE" = "." ]; then
	HERE="$PWD"
elif [[ "$HERE" != /* ]]; then
	HERE="$PWD/$HERE"
fi
PARENT="${HERE%/*}"
GGPARENT="${PARENT%/*/*}"
LD_LIBRARY_PATH=$GGPARENT/utils/lib ${_WRAPPER_EXEC_CMD:-exec} $PARENT/libexec/$ME "$@"
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
HERE="${0%/*}"
if [ "$HERE" = "." ]; then
	HERE="$PWD"
elif [[ "$HERE" != /* ]]; then
	HERE="$PWD/$HERE"
fi
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
