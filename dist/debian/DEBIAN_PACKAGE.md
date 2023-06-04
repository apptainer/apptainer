# Creating the Debian package

## Preparation

First, as long as the debian directory is in the `dist` sub-directory
you need to link or copy it into the top directory.

In the top directory do this:

```sh
rm -rf debian
cp -r dist/debian .
```

Next, make sure all the dependencies are met. See
[INSTALL.md](../../INSTALL.md#install-system-dependencies)
for the basic apt-get install list.
Also install these additional dependencies for the packaging:

```sh
sudo apt-get install \
    debhelper \
    dh-autoreconf \
    devscripts \
    help2man \
    libarchive-dev \
    libssl-dev \
    python2 \
    uuid-dev \
    golang-go
```

If the golang-go version isn't at least the minimum shown in
`scripts/get-min-go-version` (which is the case on Debian 11)
then download a copy of the corresponding go source tarball into the
debian directory like this:

```sh
TARBALL=go$(scripts/get-min-go-version).src.tar.gz
wget -O debian/$TARBALL https://dl.google.com/go/$TARBALL
```

Finally, install the additional dependencies in the section on the
[improved performance squashfuse](../../INSTALL.md##installing-improved-performance-squashfuse_ll)
and download the additional files listed there into the debian directory.
Also install the additional file from the section on
[installing gocryptfs)(../../INSTALL.md##installing-gocryptfs)
into the debian directory.

## Configuration

To do some configuration for the build, some environment variables can
be used.

Due to the fact, that `debuild` filters out some variables, all the
configuration variables need to be prefixed by `DEB_`

### mconfig

See `mconfig --help` for details about the configuration options.

`export DEB_NONETWORK=1` adds --without-network

`export DEB_NOSECCOMP=1` adds --without-seccomp

`export DEB_NOALL=1`     adds all of the above

To select a specific profile for `mconfig` set `DEB_SC_PROFILE`.
For real production environment use this configuration:

```sh
export DEB_SC_PROFILE=release-stripped
```

or if debugging is needed use this:

```sh
export DEB_SC_PROFILE=debug
```

In case a different build directory is needed:

```sh
export DEB_SC_BUILDDIR=builddir
```

### debchange

One way to update the changelog would be that the developer of apptainer
update the Debian changelog on every commit. As this is double work, because
of the CHANGELOG.md in the top directory, the changelog is automatically
updated with the version of the source which is currently checked out.
Which means you can easily build Debian packages for all the different tagged
versions of the software. See `INSTALL.md` on how to checkout a specific
version.

Be aware, that `debchange` will complain about a lower version as the top in
the current changelog. Which means you have to cleanup the changelog if needed.
If you did not change anything in the debian directory manually, it might
be easiest to [start from scratch](#preparation).
Be aware, that the Debian install directory as you see it now might not
be available in older versions (branches, tags). Make sure you have a
clean copy of the debian directory before you switch to (checkout) an
older version.

Usually `debchange` is configured by the environment variables
`DEBFULLNAME` and `DEBEMAIL`. As `debuild` creates a clean environment it
filters out many of the environment variables, so to set `DEBFULLNAME` for
the `debchange` command in the makefile, you have to set `DEB_FULLNAME`.
`DEBEMAIL` is not filtered, so you an use that directly.
If these variables are not set, `debchange` will try to find appropriate
values from the system configuration, usually by using the login name
and the domain-name.

```sh
export DEB_FULLNAME="Your Name"
export DEBEMAIL="you@example.org"
```

## Building

As usual for creating a Debian package you can use `dpkg-buildpackage`
or `debuild` which is a kind of wrapper for the first and includes the start
of `lintian`, too.

```sh
dpkg-buildpackage --build=binary --no-sign
lintian --verbose --display-info --show-overrides
```

or all in one

```sh
debuild --build=binary --no-sign --lintian-opts --display-info --show-overrides
```

After successful build the Debian packages can be found in the parent directory.

To clean up the temporary files created by `debuild` use the command:

```sh
dh clean
```

To cleanup the copy of the debian directory, make sure you saved your
changes (if any) and remove it.

```sh
rm -rf debian
```

For details on Debian package building see the man-page of `debuild` and
`dpkg-buildpackage` and `lintian`

## Debian Repository

In the current version this is not ready for use in official
Debian Repositories.

This might change in future. I updated the old debian directory to make
it just work, for people needing it.

Any help is welcome to provide a Debian installer which can be used for
building a Debian package,
that can be used in official Debian Repositories.
