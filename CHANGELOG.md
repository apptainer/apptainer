# Apptainer Changelog

The Singularity Project has been
[adopted by the Linux Foundation](https://www.linuxfoundation.org/press-release/new-linux-foundation-project-accelerates-collaboration-on-container-systems-between-enterprise-and-high-performance-computing-environments/)
and re-branded as Apptainer.
For older changes see the [archived Singularity change log](https://github.com/apptainer/singularity/blob/release-3.8/CHANGELOG.md).

## Changes since last release

- `--writable-tmpfs` can be used with `singularity build` to run
  the `%test` section of the build with a ephemeral tmpfs overlay,
  permitting tests that write to the container filesystem.
- `--compat` flag for actions is a new short-hand to enable a number of
  options that increase OCI/Docker compatibility. Infers `--containall,
  --no-init, --no-umask, --writable-tmpfs`. Does not use user, uts, or
  network namespaces as these may not be supported on many installations.
- `--no-https` now applies to connections made to library services specified
  in `--library://<hostname>/...` URIs.
- The experimental `--nvccli` flag will use `nvidia-container-cli` to setup the
  container for Nvidia GPU operation. Singularity will not bind GPU libraries
  itself. Environment variables that are used with Nvidia's `docker-nvidia`
  runtime to configure GPU visibility / driver capabilities & requirements are
  parsed by the `--nvccli` flag from the environment of the calling user. By
  default, the `compute` and `utility` GPU capabilities are configured. The `use
  nvidia-container-cli` option in `singularity.conf` can be set to `yes` to
  always use `nvidia-container-cli` when supported.
  `--nvccli` is not supported in the setuid workflow,
  and it requires being used in combination with `--writable` in user
  namespace mode.
  Please see documentation for more details.
- A new `--mount` flag and `SINGULARITY_MOUNT` environment variable can be used
  to specify bind mounts in
  `type=bind,source=<src>,destination=<dst>[,options...]` format. This improves
  CLI compatibility with other runtimes, and allows binding paths containing
  `:` and `,` characters (using CSV style escaping).
- Perform concurrent multi-part downloads for `library://` URIs. Uses 3
  concurrent downloads by default, and is configurable in `singularity.conf` or
  via environment variables.

### Changed defaults / behaviours

- Building Singularity from source requires go >=1.16. We now aim to support
  the two most recent stable versions of Go. This corresponds to the Go
  [Release Maintenance Policy](https://github.com/golang/go/wiki/Go-Release-Cycle#release-maintenance)
  and [Security Policy](https://golang.org/security), ensuring critical bug
  fixes and security patches are available for all supported language versions.
  However, rpm and debian packaging apply patches to support older native go
  installations.
- LABELs from Docker/OCI images are now inherited. This fixes a longstanding
  regression from Singularity 2.x. Note that you will now need to use
  `--force` in a build to override a label that already exists in the source
  Docker/OCI container.
- Instances are no longer created with an IPC namespace by default. An IPC
  namespace can be specified with the `-i|--ipc` flag.
- `--bind`, `--nv` and `--rocm` options for `build` command can't be set through
  environment variables `SINGULARITY_BIND`, `SINGULARITY_BINDPATH`, `SINGULARITY_NV`,
  `SINGULARITY_ROCM` anymore due to side effects reported by users in this
  [issue](https://github.com/hpcng/singularity/pull/6211), they must be explicitely
  requested via command line.
- `--nohttps` flag has been deprecated in favour of `--no-https`. The old flag
  is still accepted, but will display a deprecation warning.
- Removed `--nonet` flag, which was intended to disable networking for in-VM
  execution, but has no effect.
- Paths for `cryptsetup`, `go`, `ldconfig`, `mksquashfs`, `nvidia-container-cli`,
  `unsquashfs` are now found at build time by `mconfig` and written into
  `singularity.conf`. The path to these executables can be overridden by
  changing the value in `singularity.conf`. If the path for any of them other
  than `cryptsetup` or `ldconfig` is not set in `singularity.conf` then the
  executable will be found by searching `$PATH`.
- When calling `ldconfig` to find GPU libraries, singularity will *not* fall back
  to `/sbin/ldconfig` if the `ldconfig` on `$PATH` errors. If installing in a
  Guix/Nix on environment on top of a standard host distribution you *must* set
  `ldconfig path = /sbin/ldconfig` to use the host distribution `ldconfig` to
  find GPU libraries.
- Example log-plugin rewritten as a CLI callback that can log all commands
  executed, instead of only container execution, and has access to command
  arguments.
- The bundled reference CNI plugins are updated to v1.0.1. The `flannel` plugin
  is no longer included, as it is maintained as a separate plugin at:
  <https://github.com/flannel-io/cni-plugin>. If you use the flannel CNI plugin
  you should install it from this repository.
- `--nv` will not call `nvidia-container-cli` to find host libraries, unless
  the new experimental GPU setup flow that employs `nvidia-container-cli`
  for all GPU related operations is enabled (see above).
- If a container is run with `--nvccli` and `--contain`, only GPU devices
  specified via the `NVIDIA_VISIBLE_DEVICES` environment variable will be
  exposed within the container. Use `NVIDIA_VISIBLE_DEVICES=all` to access all
  GPUs inside a container run with `--nvccli`.
- Build `--bind` option allows to set multiple bind mount without specifying
  the `--bind` option for each bindings.
- The behaviour of the `allow container` directives in `singularity.conf` has
  been modified, to support more intuitive limitations on the usage of SIF and non-SIF
  container images. If you use these directives, _you may need to make changes
  to singularity.conf to preserve behaviour_.
  - A new `allow container sif` directive permits or denies usage of
    _unencrypted_ SIF images, irrespective of the filesystem(s) inside the SIF.
  - The `allow container encrypted` directive permits or denies usage of SIF
    images with an encrypted root filesystem.
  - The `allow container squashfs/extfs` directives in `singularity.conf`
    permit or deny usage of bare SquashFS and EXT image files only.
  - The effect of the `allow container dir` directive is unchanged.
