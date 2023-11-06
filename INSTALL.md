# Installing Apptainer

Since you are reading this from the Apptainer source code, it will be assumed
that you are building/compiling from source.

For more complete instructions on installation options, including options
that install Apptainer from pre-compiled binaries, please check the
[installation section of the admin guide](https://apptainer.org/docs/admin/main/installation.html).

## Install system dependencies

You must first install development tools and libraries to your host.

On Debian-based systems, including Ubuntu:

```sh
# Ensure repositories are up-to-date
sudo apt-get update
# Install debian packages for dependencies
sudo apt-get install -y \
    build-essential \
    libseccomp-dev \
    pkg-config \
    uidmap \
    squashfs-tools \
    squashfuse \
    fuse2fs \
    fuse-overlayfs \
    fakeroot \
    cryptsetup \
    curl wget git
```

On CentOS/RHEL:

```sh
# Install basic tools for compiling
sudo yum groupinstall -y 'Development Tools'
# Ensure EPEL repository is available
sudo yum install -y epel-release
# Install RPM packages for dependencies
sudo yum install -y \
    libseccomp-devel \
    squashfs-tools \
    squashfuse \
    fuse-overlayfs \
    fakeroot \
    /usr/*bin/fuse2fs \
    cryptsetup \
    wget git
```

On SLE/openSUSE

```sh
# Install RPM packages for dependencies
sudo zypper install -y \
  libseccomp-devel \
  libuuid-devel \
  openssl-devel \
  cryptsetup sysuser-tools \
  gcc go
```

## Install Go

Apptainer is written in Go, and may require a newer version of Go than is
available in the repositories of your distribution. We recommend installing the
latest version of Go from the [official binaries](https://golang.org/dl/).

First, download the Go tar.gz archive to `/tmp`, then extract the archive to
`/usr/local`.

_**NOTE:** if you are updating Go from a older version, make sure you remove
`/usr/local/go` before reinstalling it._

```sh
export GOVERSION=1.19.6 OS=linux ARCH=amd64  # change this as you need

wget -O /tmp/go${GOVERSION}.${OS}-${ARCH}.tar.gz \
  https://dl.google.com/go/go${GOVERSION}.${OS}-${ARCH}.tar.gz
sudo tar -C /usr/local -xzf /tmp/go${GOVERSION}.${OS}-${ARCH}.tar.gz
```

Finally, add `/usr/local/go/bin` to the `PATH` environment variable:

```sh
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

## Install golangci-lint

If you will be making changes to the source code, and submitting PRs, you should
install `golangci-lint`, which is the linting tool used in the Apptainer
project to ensure code consistency.

Every pull request must pass the `golangci-lint` checks, and these will be run
automatically before attempting to merge the code. If you are modifying
Apptainer and contributing your changes to the repository, it's faster to run
these checks locally before uploading your pull request.

In order to download and install the latest version of `golangci-lint`, you can
run:

<!-- markdownlint-disable MD013 -->

```sh
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.51.1
```

<!-- markdownlint-enable MD013 -->

Add `$(go env GOPATH)` to the `PATH` environment variable:

```sh
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
```

## Clone the repo

With the adoption of Go modules you no longer need to clone the Apptainer
repository to a specific location.

Clone the repository with `git` in a location of your choice:

```sh
git clone https://github.com/apptainer/apptainer.git
cd apptainer
```

By default your clone will be on the `main` branch which is where development
of Apptainer happens.
To build a specific version of Apptainer, check out a
[release tag](https://github.com/apptainer/apptainer/tags) before compiling,
for example:

```sh
git checkout v1.2.4
```

## Compiling Apptainer

You can configure, build, and install Apptainer using the following commands:

```sh
./mconfig
cd $(/bin/pwd)/builddir
make
sudo make install
```

And that's it! Now you can check your Apptainer version by running:

```sh
apptainer --version
```

The `mconfig` command accepts options that can modify the build and installation
of Apptainer. For example, to build in a different folder and to set the
install prefix to a different path:

```sh
./mconfig -b ./buildtree -p /usr/local
```

If you want a setuid-installation (formerly the default) use the
`--with-suid` option.

See the output of `./mconfig -h` for available options.

## Installing improved performance squashfuse_ll

If you want to have the best performance for unprivileged mounts of SIF
files for multi-core applications, you can optionally install an improved
performance version of `squashfuse_ll`.  That version has been released as
version 0.2.0 but as of this writing it is not yet very widely distributed,
and it does not have multithreading enabled in the default compilation options.
Instructions for installing it from source follow here.

First, make sure that additional required packages are installed.  On Debian:

```sh
apt-get install -y autoconf automake libtool pkg-config libfuse-dev zlib1g-dev
```

On CentOS/RHEL:

```sh
yum install -y autoconf automake libtool pkgconfig fuse3-devel zlib-devel
```

To download the source code do this:

```sh
SQUASHFUSEVERSION=0.2.0
curl -L -O https://github.com/vasi/squashfuse/archive/$SQUASHFUSEVERSION/squashfuse-$SQUASHFUSEVERSION.tar.gz
```

Then to compile and install do this:

```sh
tar xzf squashfuse-$SQUASHFUSEVERSION.tar.gz
cd squashfuse-$SQUASHFUSEVERSION
./autogen.sh
CFLAGS=-std=c99 ./configure --enable-multithreading
make squashfuse_ll
sudo cp squashfuse_ll /usr/local/libexec/apptainer/bin
```

## Installing gocryptfs

If you want to support SIF encryption and/or decryption in unprivileged
mode, then gocryptfs needs to installed.  It is available as a package
install on some operating systems as described in its
[documentation](https://nuetzlich.net/gocryptfs/quickstart/), but
otherwise to compile it from source follow these instructions.

To download the source code do this:

```sh
GOCRYPTFSVERSION=2.4.0
curl -L -O https://github.com/rfjakob/gocryptfs/archive/v$GOCRYPTFSVERSION/gocryptfs-$GOCRYPTFSVERSION.tar.gz
```

Then to compile and install do this:

```sh
tar xzf gocryptfs-$GOCRYPTFSVERSION.tar.gz
cd gocryptfs-$GOCRYPTFSVERSION
./build-without-openssl.bash
sudo cp gocryptfs /usr/local/libexec/apptainer/bin
```

## Building & Installing from RPM

On a RHEL / CentOS / Fedora machine you can build an Apptainer into rpm
packages, and install it from them. This is useful if you need to install
Apptainer across multiple machines, or wish to manage all software via
`yum/dnf`.

To build the rpms, in addition to the
[system dependencies](#install-system-dependencies),
also install these extra packages:

```sh
sudo yum install -y rpm-build golang
```

The rpm build will use the OS distribution or EPEL version of Go,
or it will use a different installation of Go, whichever is first in $PATH.
If the first `go` found in $PATH is too old,
then the rpm build uses that older version to compile the newer go
toolchain from source.
This mechanism is necessary for rpm build systems that do not allow
downloading anything from the internet.
In order to make use of this mechanism, use the `mconfig --only-rpm` option
to skip the minimum version check.
`mconfig` will then create a `.spec` file that looks for a go source
tarball in the rpm build's current directory.
If you need it, download the go tarball like this:

```sh
wget https://dl.google.com/go/go$(scripts/get-min-go-version).src.tar.gz
```

Then download the latest
[apptainer release tarball](https://github.com/apptainer/apptainer/releases)
like this:

<!-- markdownlint-disable MD013 -->

```sh
VERSION=1.2.4  # this is the apptainer version, change as you need
# Fetch the source
wget https://github.com/apptainer/apptainer/releases/download/v${VERSION}/apptainer-${VERSION}.tar.gz
```

Next we need to include the source of squashfuse_ll and gocryptfs.
The easiest way to do that is to modify the apptainer tarball and
include them inside of it.  First unpack it like this:

```sh
tar xf apptainer-${VERSION}.tar.gz
cd apptainer-${VERSION}
```

Then install the extra packages and download the source code into the
current directory as shown at
[the above squashfuse link](#installing-improved-performance-squashfuse_ll)
and [the above gocryptfs link](#installing-gocryptfs).
(If the rpm needs to be built offline from the internet see
additional instructions for the gocryptfs source code in
dist/rpm/apptainer.spec.in).
Then recreate the apptainer tarball like this:

```sh
cd ..
tar czf apptainer-${VERSION}.tar.gz apptainer-${VERSION}
rm -rf apptainer-${VERSION}
```

Then build the rpms from the tarball like this:

```sh
rpmbuild -tb apptainer-${VERSION}.tar.gz
# Install Apptainer using the resulting rpm
sudo rpm -Uvh ~/rpmbuild/RPMS/x86_64/apptainer-$(echo $VERSION|tr - \~)-1.el7.x86_64.rpm
# (Optionally) Install the setuid-root portion
sudo rpm -Uvh ~/rpmbuild/RPMS/x86_64/apptainer-suid-$(echo $VERSION|tr - \~)-1.el7.x86_64.rpm
# (Optionally) Remove the build tree and source to save space
rm -rf ~/rpmbuild apptainer-${VERSION}*.tar.gz
```

<!-- markdownlint-enable MD013 -->

Alternatively, to build RPMs from the latest main you can
[clone the repo as detailed above](#clone-the-repo), and run `./mconfig`.
Then use the `rpm` make target to build Apptainer as rpm packages,
for example like this if you already have a new enough golang first
in your PATH:

<!-- markdownlint-disable MD013 -->

```sh
VERSION=1.2.4 # this is the latest apptainer version, change as you need
./mconfig
make -C builddir rpm
sudo rpm -ivh ~/rpmbuild/RPMS/x86_64/apptainer-$(echo $VERSION|tr - \~)*.x86_64.rpm 
# (Optionally) Install the setuid-root portion
sudo rpm -ivh ~/rpmbuild/RPMS/x86_64/apptainer-suid-$(echo $VERSION|tr - \~)*.x86_64.rpm 
```

<!-- markdownlint-enable MD013 -->

That will not include squashfuse_ll and gocryptfs in the rpm unless you
uncomment the %global definitions of their version numbers in apptainer.spec
first and have their source tarballs available in the current directory.

By default, the rpms will be built so that Apptainer is installed in
standard Linux paths under ``/``.

To build rpms with an alternative install prefix set RPMPREFIX on the make
step, for example:

```sh
make -C builddir rpm RPMPREFIX=/opt/apptainer
```

For more information on installing/updating/uninstalling RPMs, check out our
[admin docs](https://apptainer.org/docs/admin/main/admin_quickstart.html).

## Debian Package

Information on how to build Debian packages can be found in
[dist/debian/DEBIAN_PACKAGE.md](dist/debian/DEBIAN_PACKAGE.md).

## Run E2E tests

The test suite is heavily relying on Docker Hub registry, since the introduction
of the rate pull limit, developers can quickly hit the quota limit leading to
the e2e tests randomly failed.

There is two possible approaches to minimize/avoid that:

1. if you have an account on Docker Hub you can specify and export your
credentials via environment variables `E2E_DOCKER_USERNAME` and
`E2E_DOCKER_PASSWORD` before running the test suite, however if you have
a free account the quota limit is simply doubled and may not work for you
2. or you can run a local pull through cache registry and use
`E2E_DOCKER_MIRROR`/`E2E_DOCKER_MIRROR_INSECURE` environment variables

### Run a local pull through cache registry

The most straightforward way to run it is to run in a terminal:

```sh
mkdir -p $HOME/.cache/registry
apptainer run --env REGISTRY_HTTP_ADDR=127.0.0.1:5001 \
                --env REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
                --bind $HOME/.cache/registry:/var/lib/registry \
                docker://registry:2.7
```

And run the test suite in another terminal:

```sh
export E2E_DOCKER_MIRROR=127.0.0.1:5001
export E2E_DOCKER_MIRROR_INSECURE=true
make -C builddir e2e-test
```
