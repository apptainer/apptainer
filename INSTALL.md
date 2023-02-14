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
export GOVERSION=1.18.4 OS=linux ARCH=amd64  # change this as you need

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
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.43.0
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
git checkout v1.1.6
```

## Compiling Apptainer

You can configure, build, and install Apptainer using the following commands:

```sh
./mconfig
cd ./builddir
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
performance version of `squashfuse_ll`.

As of this writing there is a patch pending to the squashfuse project to
add multithreading support that significantly improves performance for
applications that access a lot of small files from many cores at once.
There's also another `squashfuse_ll` patch for supporting options to make
files appear to be owned by the user
(the same options that exist in `squashfuse`).

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
SQUASHFUSEVERSION=0.1.1.6
SQUASHFUSEPRS="70 77 81"
curl -L -O https://github.com/vasi/squashfuse/archive/$SQUASHFUSEVERSION/squashfuse-$SQUASHFUSEVERSION.tar.gz
for PR in $SQUASHFUSEPRS; do
    curl -L -O https://github.com/vasi/squashfuse/pull/$PR.patch
done
```

Then to compile and install do this:

```sh
tar xzf squashfuse-$SQUASHFUSEVERSION.tar.gz
cd squashfuse-$SQUASHFUSEVERSION
for PR in $SQUASHFUSEPRS; do
    patch -p1 <../$PR.patch
done
./autogen.sh
FLAGS=-std=c99 ./configure --enable-multithreading
make squashfuse_ll
sudo cp squashfuse_ll /usr/local/libexec/apptainer/bin
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
VERSION=1.1.6  # this is the apptainer version, change as you need
# Fetch the source
wget https://github.com/apptainer/apptainer/releases/download/v${VERSION}/apptainer-${VERSION}.tar.gz
```

At this point if you don't care about improved squashfuse performance
then skip down to the rpmbuild command below.
If you do want it then you need to modify the tarball.
First unpack it and uncomment one line like this:

```sh
tar xf apptainer-${VERSION}.tar.gz
cd apptainer-${VERSION}
sed -i 's/^# %\(%global squashfuse_version\)/\1/' apptainer.spec
```

Then install the extra packages and download the source code as shown at
[the above link](#installing-improved-performance-squashfuse_ll)
and recreate the tarball like this:

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
VERSION=1.1.6 # this is the latest apptainer version, change as you need
./mconfig
make -C builddir rpm
sudo rpm -ivh ~/rpmbuild/RPMS/x86_64/apptainer-$(echo $VERSION|tr - \~)*.x86_64.rpm 
# (Optionally) Install the setuid-root portion
sudo rpm -ivh ~/rpmbuild/RPMS/x86_64/apptainer-suid-$(echo $VERSION|tr - \~)*.x86_64.rpm 
```

<!-- markdownlint-enable MD013 -->

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
