# Static Apptainer build

This directory builds a statically linked, non-SUID Apptainer installation
packed into a relocatable tarball. It has no runtime library dependencies, so
it can be used on hosts where installing Apptainer from packages or building
it from source is not possible, such as minimal or appliance-style systems.

Each tarball contains a single top level `apptainer-<version>` directory with
the usual `bin`, `etc`, `libexec`, `share` and `var` installation tree. Static
builds of the same tools that the rpm and deb packages bundle are included in
`libexec/apptainer/bin`: `mksquashfs`, `unsquashfs`, `squashfuse_ll`,
`fuse2fs`, `fuse-overlayfs`, `gocryptfs`, `archivemount` and, where it is
supported, `proot`. Their versions are taken from
`dist/rpm/apptainer.spec.in`, and they are built with
`scripts/download-dependencies` and `scripts/compile-dependencies`.

## Quick start

Build a tarball for the current architecture, here `amd64`, from the top of
the Apptainer source tree:

```sh
dist/static/build.sh amd64
```

Extract it anywhere, for example into your home directory, and add its `bin`
directory to your `PATH`:

```sh
tar -xzf apptainer-<version>-linux-amd64.tar.gz -C ~
export PATH=~/apptainer-<version>/bin:$PATH
```

Then pull and run the `lolcow` image from the
[user guide](https://apptainer.org/docs/user/main/quick_start.html):

```sh
apptainer pull docker://ghcr.io/apptainer/lolcow
apptainer run lolcow_latest.sif
apptainer exec lolcow_latest.sif cowsay moo
```

The installation can be moved to another directory or host at any time. Its
configuration is read from `etc/apptainer` below the extracted directory.

## Building

An existing Apptainer installation that can use `--fakeroot` is required.
From the top of the Apptainer source tree run:

```sh
dist/static/build.sh amd64 arm64
```

This writes `apptainer-<version>-linux-<arch>.tar.gz` for each architecture to
the top of the source tree.

The build runs `build-in-container.sh` in `%post` of `static.def`, inside a
`golang:<version>-alpine` container, because Alpine provides the musl C
library and static versions of the libraries that the bundled tools need.
The sandbox that `apptainer build` leaves behind is removed afterwards.

To build `arm64` tarballs on an `amd64` host, register QEMU user-mode
emulation first, for example with:

```sh
sudo apptainer run docker://multiarch/qemu-user-static --reset -p yes
```

## Limitations

- There is no SUID starter, so unprivileged users need unprivileged user
  namespaces to be enabled on the host.
- For users listed in `/etc/subuid`, `--fakeroot` uses the host's setuid
  `newuidmap` and `newgidmap`. Other users get a root-mapped user namespace
  instead.
- Built without `libsubid` support, so subordinate IDs are read from
  `/etc/subuid` and `/etc/subgid` only.
- `offsetpreload.so` is not included, as a static build cannot be preloaded
  into host programs. EXT3 partitions in SIF files, such as persistent
  overlays, are instead mounted with the `offset` option of the bundled
  `fuse2fs`, which needs e2fsprogs 1.46.6 or later.
- The musl C library does not support NSS modules, so users and groups are
  looked up in `/etc/passwd` and `/etc/group`, and through `nscd` if it is
  running. Users and groups that come only from other sources, such as
  LDAP through SSSD, are not resolved by name. This is also what makes the
  build reliable: a static glibc build loads the host's NSS modules for
  these lookups, which can crash it, as in issue #2449.
