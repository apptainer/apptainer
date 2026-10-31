# Static Apptainer build

This directory builds a statically linked, non-SUID Apptainer installation
packed into a relocatable tarball. It has no runtime library dependencies, so
it can be used on hosts where installing Apptainer from packages or building
it from source is not possible, such as minimal or appliance-style systems.

Each tarball contains a single top level `apptainer-<version>` directory with
the usual `bin`, `etc`, `libexec`, `share` and `var` installation tree. Static
`mksquashfs` and `unsquashfs` binaries are included in
`libexec/apptainer/bin`, so that images can be built and pulled without
squashfs-tools installed on the host.

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

Docker with BuildKit (`docker buildx`) is required. From the top of the
Apptainer source tree run:

```sh
dist/static/build.sh amd64 arm64
```

This writes `apptainer-<version>-linux-<arch>.tar.gz` for each architecture to
the top of the source tree.

The build runs natively for the target platform inside a
`golang:<version>-alpine` container. To build `arm64` tarballs on an `amd64`
host, register QEMU user-mode emulation first, for example with:

```sh
docker run --privileged --rm tonistiigi/binfmt --install arm64
```

## Limitations

- There is no SUID starter, so unprivileged users need unprivileged user
  namespaces to be enabled on the host.
- For users listed in `/etc/subuid`, `--fakeroot` uses the host's setuid
  `newuidmap` and `newgidmap`. Other users get a root-mapped user namespace
  instead.
- Built without seccomp support, so `--security seccomp:...` is unavailable.
- Built without `libsubid` support, so subordinate IDs are read from
  `/etc/subuid` and `/etc/subgid` only.
- `squashfuse`, `fuse2fs`, `fuse-overlayfs` and `gocryptfs` are not included.
  If they are found on the host they are used, otherwise unprivileged runs of
  SIF images extract them to a temporary sandbox.
