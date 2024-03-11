# Apptainer Changelog

The Singularity Project has been
[adopted by the Linux Foundation](https://www.linuxfoundation.org/press-release/new-linux-foundation-project-accelerates-collaboration-on-container-systems-between-enterprise-and-high-performance-computing-environments/)
and re-branded as Apptainer.
For older changes see the [archived Singularity change log](https://github.com/apptainer/singularity/blob/release-3.8/CHANGELOG.md).

## v1.3.0 - \[2024-03-12\]

Changes since v1.2.5

### Changed defaults / behaviours

- FUSE mounts are now supported in setuid mode, enabling full
  functionality even when kernel filesystem mounts are insecure
  due to unprivileged users having write access to raw filesystems
  in containers.

  When `allow setuid-mount extfs = no` (the default) in apptainer.conf,
  then the fuse2fs image driver will be used to mount ext3 images in setuid
  mode instead of the kernel driver (ext3 images are primarily used for the
  `--overlay` feature), restoring functionality that was removed by
  default in Apptainer 1.1.8 because of the security risk.

  The `allow setuid-mount squashfs` configuration option in
  apptainer.conf now has a new default called `iflimited` which allows
  kernel squashfs mounts only if there is at least one `limit container`
  option set or if Execution Control Lists are activated in ecl.toml.
  If kernel squashfs mounts are are not allowed, then the squashfuse
  image driver will be used instead.
  `iflimited` is the default because if one of those limits are used
  the system administrator ensures that unprivileged users do not have
  write access to the containers, but on the other hand using FUSE would
  enable a user to theoretically bypass the limits via ptrace() because
  the FUSE process runs as that user.

  The `fuse-overlayfs` image driver will also now be tried in setuid mode
  if the kernel overlayfs driver does not work (for example if one of
  the layers is a FUSE filesystem).

  In addition,
  if `allow setuid-mount encrypted = no` then the unprivileged gocryptfs
  format will be used for encrypting SIF files instead of the kernel
  device-mapper.  If a SIF file was encrypted using the gocryptfs
  format, it can now be mounted in setuid mode in addition to
  non-setuid mode.
- The four dependent FUSE programs for various reasons all now need to
  be compiled from source and included in Apptainer installations and
  packages.
  Scripts are provided to make this easy; see the updated instructions
  in [INSTALL.md](INSTALL.md).
  The bundled squashfuse_ll is updated to version 0.5.1.
- Change the default in user namespace mode to use either kernel
  overlayfs or fuse-overlayfs instead of the underlay feature for the
  purpose of adding bind mount points.  That was already the default in
  setuid mode; this change makes it consistent.  The underlay feature can
  still be used with the `--underlay` option, but it is deprecated because
  the implementation is complicated and measurements have shown that the
  performance of underlay is similar to overlayfs and fuse-overlayfs.
  For now the underlay feature can be made the default again with a new
  `preferred` value on the `enable underlay` configuration option.
  Also the `--underlay` option can be used in setuid mode or as the root
  user, although it was ignored previously.
- Prefer again to use kernel overlayfs over fuse-overlayfs when a lower
  layer is FUSE and there's no writable upper layer, undoing the change
  from 1.2.0.  Another workaround was found for the problem that change
  addressed.  This applies in both setuid mode and in user namespace
  mode (except the latter not on CentOS7 where it isn't supported).
- `--cwd` is now the preferred form of the flag for setting the container's
  working directory, though `--pwd` is still supported for compatibility.
- When building RPM, we will now use `/var/lib/apptainer` (rather than
  `/var/apptainer`) to store local state files.
- The way --home is handled when running as root (e.g. `sudo apptainer`) or
  with `--fakeroot` has changed. Previously, we were only modifying the `HOME`
  environment variable in these cases, while leaving the container's
  `/etc/passwd` file unchanged (with its homedir field pointing to `/root`,
  regardless of the value passed to `--home`). With this change, both value of
  `HOME` and the contents of `/etc/passwd` in the container will reflect the
  value passed to `--home` if the container is readonly.  If the container
  is writable, the `/etc/passwd` file is left alone because it can interfere
  with commands that want to modify it.
- The `--vm` and related flags to start apptainer inside a VM have been
  removed. This functionality was related to the retired Singularity Desktop /
  SyOS projects.
- The keyserver-related commands that were under `remote` have been moved to
  their own, dedicated `keyserver` command. Run `apptainer help keyserver` for
  more information.
- The commands related to OCI/Docker registries that were under `remote` have
  been moved to their own, dedicated `registry` command. Run
  `apptainer help registry` for more information.
- The the `remote list` subcommand now outputs only remote endpoints (with
  keyservers and OCI/Docker registries having been moved to separate commands),
  and the output has been streamlined.
- Adding a new remote endpoint using the `apptainer remote add` command will
  now set the new endpoint as default. This behavior can be suppressed by
  supplying the `--no-default` (or `-n`) flag to `remote add`.
- Skip parsing build definition file template variables after comments
  beginning with a hash symbol.
- Improved the clarity of `apptainer key list` output.
- The global /tmp directory is no longer used for gocryptfs mountpoints.  
- Updated minimum go version to 1.20

### New Features & Functionality

- The `remote status` command will now print the username, realname, and email
  of the logged-in user, if available.
- Add monitoring feature support, which requires the usage of an additional tool
  named `apptheus`, this tool will put apptainer starter into a newly created
  cgroup and collect system metrics.
- A new `--no-pid` flag for `apptainer run/shell/exec` disables the PID namespace
  inferred by `--containall` and `--compat`.
- Added `--config` option to`keyserver` commands.
- Honor an optional remoteName argument to the `keyserver list` command.
- Added the `APPTAINER_ENCRYPTION_PEM_DATA` env var to allow for encrypting and
  running encrypted containers without a PEM file.  
- Adding `--sharens` mode for `apptainer exec/run/shell`, which enables to run multiple
  apptainer instances created by the same parent using the same image in the same
  user namespace.

### Developer / API

- Changes in pkg/build/types.Definition struct. New `.FullRaw` field introduced,
  which always contains the raw data for the entire definition file. Behavior of
  `.Raw` field has changed: for multi-stage builds parsed with
  pkg/build/types/parser.All(), `.Raw` contains the raw content of a single
  build stage. Otherwise, it is equal to `.FullRaw`.

### Bug fixes

- Don't bind `/var/tmp` on top of `/tmp` in the container, where `/var/tmp`
  resolves to same location as `/tmp`.
- Support parentheses in `test` / `[` commands in container startup scripts,
  via dependency update of mvdan.cc/sh.
- Fix regression introduced in v1.2.0 that led to an empty user's shell field
  in the `/etc/passwd` file.
- Prevent container builds from failing when `$HOME` points to a non-readable
  directory.
- Fix the use of `nvidia-container-cli` on Ubuntu 22.04 where an
  `ldconfig` wrapper script gets in the way. Instead, we use
  `ldconfig.real` directly.
- Run image drivers with CAP_DAC_OVERRIDE in user namespace mode. This
  fixes --nvccli with NVIDIA_DRIVER_CAPABILITIES=graphics, which
  previously failed when using fuse-overlayfs.

### Release change

- Releases will generate apptainer Docker images for the Linux amd64 and arm64
  architectures.

## Changes for v1.2.x

## v1.2.5 - \[2023-11-21\]

- Added `libnvidia-nvvm` to `nvliblist.conf`. Newer
  NVIDIA Drivers (known with >= 525.85.05) require this lib to compile
  OpenCL programs against NVIDIA GPUs, i.e. `libnvidia-opencl` depends on
  `libnvidia-nvvm.`
- Disable the usage of cgroup in instance creation when `--fakeroot` is passed.
- Disable the usage of cgroup in instance creation when `hidepid` mount option
  on /proc is set.

## v1.2.4 - \[2023-10-10\]

- Fixed a problem with relocating an unprivileged installation of
  apptainer on el8 and a mounted remote filesystem when using the
  `--fakeroot` option without `/etc/subuid` mapping.  The fix was to
  change the switch to an unprivileged root-mapped namespace to be the
  equivalent of `unshare -r` instead of `unshare -rm` on action commands,
  to work around a bug in the el8 kernel.
- Fixed a regression introduced in 1.2.0 where the user's password file
  information was not copied in to the container when there was a
  parent root-mapped user namespace (as is the case for example in
  [cvmfsexec](https://github.com/cvmfs/cvmfsexec)).
- Added the upcoming NVIDIA driver library `libnvidia-gpucomp.so` to the
  list of libraries to add to NVIDIA GPU-enabled containers.
- Fixed missing error handling during the creation of an encrypted
  image that lead to the generation of corrupted images.
- Use `APPTAINER_TMPDIR` for temporary files during privileged image
  encryption.
- If rootless unified cgroups v2 is available when starting an image but
  `XDG_RUNTIME_DIR` or `DBUS_SESSION_BUS_ADDRESS` is not set, print an
  info message that stats will not be available instead of exiting with
  a fatal error.
- Allow templated build arguments to definition files to have empty values.

## v1.2.3 - \[2023-09-14\]

- The `apptainer push/pull` commands now show a progress bar for the oras
  protocol like there was for docker and library protocols.
- The `--nv` and `--rocm` flags can now be used simultaneously.
- Fix the use of `APPTAINER_CONFIGDIR` with `apptainer instance start`
  and action commands that refer to `instance://`.
- Ignore undefined macros, to fix yum bootstrap agent on el7.
- Fix the issue that apptainer would not read credentials from the Docker
  fallback path `~/.docker/config.json` if missing in the apptainer
  credentials.

## v1.2.2 - \[2023-07-27\]

- Fix `$APPTAINER_MESSAGELEVEL` to correctly set the logging level.
- Fix build failures when in setuid mode and unprivileged user namespaces
  are unavailable and the `--fakeroot` option is not selected.
- Remove `Requires: fuse` from rpm packaging.

## v1.2.1 - \[2023-07-24\]

### Security fix

- Included a fix for
  [security advisory GHSA-mmx5-32m4-wxvx](https://github.com/apptainer/apptainer/security/advisories/GHSA-mmx5-32m4-wxvx)
  which describes an ineffective privilege drop when requesting a
  container network with a setuid installation of Apptainer.
  The vulnerability allows an attacker to delete any directory on the
  host filesystems with a crafted starter config.
  Only affects v1.2.0-rc.2 and v1.2.0.

## v1.2.0 - \[2023-07-18\]

Changes since v1.1.9

### Changed defaults / behaviours

- Create the current working directory in a container when it doesn't exist.
  This restores behavior as it was before singularity 3.6.0.
  As a result, using `--no-mount home` won't have any effect when running
  apptainer from a home directory and will require `--no-mount home,cwd` to
  avoid mounting that directory.
- Handle current working directory paths containing symlinks both on the
  host and in a container but pointing to different destinations.
  If detected, the current working directory is not mounted when the
  destination directory in the container exists.
- Destination mount points are now sorted by shortest path first to ensure that
  a user bind doesn't override a previous bind path when set in arbitrary order
  on the CLI.  This is also applied to image binds.
- When the kernel supports unprivileged overlayfs mounts in a user namespace,
  the container will be constructed by default using an overlay instead
  of an underlay layout for bind mounts.
  A new `--underlay` action option can be used to prefer underlay instead
  of overlay.
- Use fuse-overlayfs instead of the kernel overlayfs when a lower dir is
  a FUSE filesystem, even when the overlay layer is not writable.  That
  always used to be done when the overlay layer was writable, but this
  fixes a problem seen when squashfuse (which is read-only) was used for
  the overlay layer.
- Fix the `enable overlay = driver` configuration option to always use
  the overlay image driver (that is, fuse-overlayfs) even when the kernel
  overlayfs is usable.
- Overlay is blocked on the `panfs` filesystem, allowing sandbox directories
  to be run from `panfs` without error.
- `sessiondir maxsize` in `apptainer.conf` now defaults to 64 MiB for new
  installations. This is an increase from 16 MiB in prior versions.
- The apptainer cache is now architecture aware, so the same home directory
  cache can be shared by machines with different architectures.
- Show standard output of yum bootstrap if log level is verbose or higher
  while building a container.
- Lookup and store user/group information in stage one prior to entering any
  namespaces, to fix an issue with winbind not correctly looking up user/group
  information when using user namespaces.
- A new `--reproducible` flag for `./mconfig` will configure Apptainer so that
  its binaries do not contain non-reproducible paths. This disables plugin
  functionality.

### New features / functionalities

- Support for unprivileged encryption of SIF files using gocryptfs.  The
  gocryptfs command is included in rpm and debian packaging.
  This is not compatible with privileged encryption, so containers encrypted
  by root need to be rebuilt by an unprivileged user.
- Templating support for definition files. Users can now define variables in
  definition files via a matching pair of double curly brackets.
  Variables of the form `{{ variable }}` will be replaced by a value defined
  either by a `variable=value` entry in the `%arguments` section of the
  definition file or through new build options
  `--build-arg` or `--build-arg-file`.
  By default any unused variables given in `--build-arg` or `--build-arg-file`
  result in a fatal error but the option `--warn-unused-build-args` changes
  that to a warning rather than a fatal error.
- Add a new `instance run` command that will execute the runscript when an
  instance is initiated instead of executing the startscript.
- The `sign` and `verify` commands now support signing and verification
  with non-PGP key material by specifying the path to a private key via
  the `--key` flag.
- The `verify` command now supports verification with X.509 certificates by
  specifying the path to a certificate via the `--certificate` flag. By
  default, the system root certificate pool is used as trust anchors unless
  overridden via the `--certificate-roots` flag. A pool of intermediate
  certificates that are not trust anchors, but can be used to form a
  certificate chain, can also be specified via the `--certificate-intermediates`
  flag.
- Support for online verification checks of X.509 certificates using OCSP
  protocol via the new `verify --ocsp-verify` option.
- The `instance stats` command displays the resource usage every second. The
  `--no-stream` option disables this interactive mode and shows the
  point-in-time usage.
- Instances are now started in a cgroup by default, when run as root or when
  unified cgroups v2 with systemd as manager is configured.  This allows
  `apptainer instance stats` to be supported by default when possible.
- The `instance start` command now accepts an optional `--app <name>` argument
  which invokes a start script within the `%appstart <name>` section in the
  definition file.
  The `instance stop` command still only requires the instance name.
- The instance name is now available inside an instance via the new
  `APPTAINER_INSTANCE` environment variable.
- Add ability to set a custom config directory via the new
  `APPTAINER_CONFIGDIR` environment variable.
- Add ability to change log level through environment variables,
  `APPTAINER_SILENT`, `APPTAINER_QUIET`, and `APPTAINER_VERBOSE`.
  Also add `APPTAINER_NOCOLOR` for the `--nocolor` option.
- Add discussion of using TMPDIR or APPTAINER_TMPDIR in the build help.
- The `--no-mount` flag now accepts the value `bind-paths` to disable mounting
  of all `bind path` entries in `apptainer.conf`.
- Support for `DOCKER_HOST` parsing when using `docker-daemon://`
- `DOCKER_USERNAME` and `DOCKER_PASSWORD` supported without `APPTAINER_` prefix.
- Add new Linux capabilities `CAP_PERFMON`, `CAP_BPF`, and
  `CAP_CHECKPOINT_RESTORE`.
- Add `setopt` definition file header for the `yum` bootstrap agent. The
  `setopt` value is passed to `yum / dnf` using the `--setopt` flag. This
  permits setting e.g. `install_weak_deps=False` to bootstrap recent versions of
  Fedora, where `systemd` (a weak dependency) cannot install correctly in the
  container. See `examples/Fedora` for an example definition file.
- Warn user that a `yum` bootstrap of an older distro may fail if the host rpm
  `_db_backend` is not `bdb`.
- The `remote get-login-password` command allows users to retrieve a remote's
  token. This enables piping the secret directly into docker login while
  preventing it from showing up in a shell's history.
- Define EUID in %environment alongside UID.
- In `--rocm` mode, the whole of `/dev/dri` is now bound into the container when
  `--contain` is in use. This makes `/dev/dri/render` devices available,
  required for later ROCm versions.

### Other changes

- Update minimum go version to 1.19.
- Upgrade squashfuse_ll to version 0.2.0, removing the need for applying
  patches during compilation.  The new version includes a fix to prevent
  it from triggering 'No data available errors' on overlays of SIF files that
  were built on machines with SELinux enabled.
- Fix non-root instance join with unprivileged systemd-managed cgroups v2,
  when join is from outside a user-owned cgroup.
- Fix joining cgroup of instance started as root, with cgroups v1,
  non-default cgroupfs manager, and no device rules.
- Avoid UID / GID / EUID readonly var warnings with `--env-file`.
- Ensure consistent binding of libraries under `--nv/--rocm` when duplicate
  `<library>.so[.version]` files are listed by `ldconfig -p`.
- Ensure `DOCKER_HOST` is honored in non-build flows.
- Corrected `apptainer.conf` comment, to refer to correct file as source
  of default capabilities when `root default capabilities = file`.
- Fix memory usage calculation during apptainer compilation on RaspberryPi.
- Fix misleading error when an overlay is requested by the root user while the
  overlay kernel module is not loaded.
- Fix interaction between `--workdir` and `--scratch` options when the
  former is given a relative path.
- Remove the warning about a missing signature when building an image based
  on a local unsigned SIF file.
- Set real UID to zero when escalating privileges for CNI plugins, to fix
  issue appeared with RHEL 9.X.
- Fix seccomp filters to allow mknod/mknodat syscalls to create pipe/socket
  and character devices with device number 0 for fakeroot builds.
- Add 32-bit compatibility mode for 64-bit architectures in the fakeroot
  seccomp filter.

## v1.2.0-rc.2 - \[2023-07-05\]

## Changes since last pre-release

- Upgrade gocryptfs to version 2.4.0, removing the need for fusermount from
  the fuse package.
- Upgrade squashfuse_ll to version 0.2.0, removing the need for applying
  patches during compilation.  The new version includes a fix to prevent
  it from triggering 'No data available errors' on overlays of SIF files that
  were built on machines with SELinux enabled.
- Add ability to set a custom config directory via the new
  `APPTAINER_CONFIGDIR` environment variable.
- Add ability to change log level through environment variables,
  `APPTAINER_SILENT`, `APPTAINER_QUIET`, and `APPTAINER_VERBOSE`.
  Also add `APPTAINER_NOCOLOR` for the `--nocolor` option.
- Add discussion of using TMPDIR or APPTAINER_TMPDIR in the build help.
- Add new option `--warn-unused-build-args` to output warnings rather than
  fatal errors for any additional variables given in --build-arg or
  --build-arg-file.
- Use fuse-overlayfs instead of the kernel overlayfs when a lower dir is
  a FUSE filesystem, even when the overlay layer is not writable.  That
  always used to be done when the overlay layer was writable, but this
  fixes a problem seen when squashfuse (which is read-only) was used for
  the overlay layer.
- Fix the `enable overlay = driver` configuration option to always use
  the overlay image driver (that is, fuse-overlayfs) even when the kernel
  overlayfs is usable.
- Fix a minor regression in 1.2.0-rc.1 where starting up under `unshare -r`
  stopped mapping the user's home directory to the fake root's home directory.
- Fix interaction between `--workdir` and `--scratch` options when the
  former is given a relative path.
- Remove the warning about a missing signature when building an image based
  on a local unsigned SIF file.
- Set real UID to zero when escalating privileges for CNI plugins to fix
  issue appeared with RHEL 9.X.
- Fix seccomp filters to allow mknod/mknodat syscalls to create pipe/socket
  and character devices with device number 0 for fakeroot builds.
- Add 32-bit compatibility mode for 64-bit architectures in the fakeroot
  seccomp filter.

## v1.2.0-rc.1 - \[2023-06-07\]

### Changed defaults / behaviours

- Create the current working directory in a container when it doesn't exist.
  This restores behavior as it was before singularity 3.6.0.
  As a result, using `--no-mount home` won't have any effect when running
  apptainer from a home directory and will require `--no-mount home,cwd` to
  avoid mounting that directory.
- Handle current working directory paths containing symlinks both on the
  host and in a container but pointing to different destinations.
  If detected, the current working directory is not mounted when the
  destination directory in the container exists.
- Destination mount points are now sorted by shortest path first to ensure that
  a user bind doesn't override a previous bind path when set in arbitrary order
  on the CLI.  This is also applied to image binds.
- When the kernel supports unprivileged overlay mounts in a user namespace,
  the container will be constructed by default using an overlay instead
  of an underlay layout for bind mounts.
  A new `--underlay` action option can be used to prefer underlay instead
  of overlay.
- `sessiondir maxsize` in `apptainer.conf` now defaults to 64 MiB for new
  installations. This is an increase from 16 MiB in prior versions.
- The apptainer cache is now architecture aware, so the same home directory
  cache can be shared by machines with different architectures.
- Overlay is blocked on the `panfs` filesystem, allowing sandbox directories
  to be run from `panfs` without error.
- Show standard output of yum bootstrap if log level is verbose or higher
  while building a container.
- Lookup and store user/group information in stage one prior to entering any
  namespaces, to fix an issue with winbind not correctly looking up user/group
  information when using user namespaces.
- A new `--reproducible` flag for `./mconfig` will configure Apptainer so that
  its binaries do not contain non-reproducible paths. This disables plugin
  functionality.

### New features / functionalities

- Support for unprivileged encryption of SIF files using gocryptfs.  The
  gocryptfs command is included in rpm and debian packaging.
  This is not compatible with privileged encryption, so containers encrypted
  by root need to be rebuilt by an unprivileged user.
- Templating support for definition files. Users can now define variables in
  definition files via a matching pair of double curly brackets.
  Variables of the form `{{ variable }}` will be replaced by a value defined
  either by a `variable=value` entry in the `%arguments` section of the
  definition file or through new build options
  `--build-arg` or `--build-arg-file`.
- Add a new `instance run` command that will execute the runscript when an
  instance is initiated instead of executing the startscript.
- The `sign` and `verify` commands now support signing and verification
  with non-PGP key material by specifying the path to a private key via
  the `--key` flag.
- The `verify` command now supports verification with X.509 certificates by
  specifying the path to a certificate via the `--certificate` flag. By
  default, the system root certificate pool is used as trust anchors unless
  overridden via the `--certificate-roots` flag. A pool of intermediate
  certificates that are not trust anchors, but can be used to form a
  certificate chain, can also be specified via the `--certificate-intermediates`
  flag.
- Support for online verification checks of X.509 certificates using OCSP
  protocol via the new `verify --ocsp-verify` option.
- The `instance stats` command displays the resource usage every second. The
  `--no-stream` option disables this interactive mode and shows the
  point-in-time usage.
- Instances are now started in a cgroup by default, when run as root or when
  unified cgroups v2 with systemd as manager is configured.  This allows
  `apptainer instance stats` to be supported by default when possible.
- The `instance start` command now accepts an optional `--app <name>` argument
  which invokes a start script within the `%appstart <name>` section in the
  definition file.
  The `instance stop` command still only requires the instance name.
- The instance name is now available inside an instance via the new
  `APPTAINER_INSTANCE` environment variable.
- The `--no-mount` flag now accepts the value `bind-paths` to disable mounting
  of all `bind path` entries in `apptainer.conf`.
- Support for `DOCKER_HOST` parsing when using `docker-daemon://`
- `DOCKER_USERNAME` and `DOCKER_PASSWORD` supported without `APPTAINER_` prefix.
- Add new Linux capabilities `CAP_PERFMON`, `CAP_BPF`, and
  `CAP_CHECKPOINT_RESTORE`.
- Add `setopt` definition file header for the `yum` bootstrap agent. The
  `setopt` value is passed to `yum / dnf` using the `--setopt` flag. This
  permits setting e.g. `install_weak_deps=False` to bootstrap recent versions of
  Fedora, where `systemd` (a weak dependency) cannot install correctly in the
  container. See `examples/Fedora` for an example definition file.
- Warn user that a `yum` bootstrap of an older distro may fail if the host rpm
  `_db_backend` is not `bdb`.
- The `remote get-login-password` command allows users to retrieve a remote's
  token. This enables piping the secret directly into docker login while
  preventing it from showing up in a shell's history.
- Define EUID in %environment alongside UID.
- In `--rocm` mode, the whole of `/dev/dri` is now bound into the container when
  `--contain` is in use. This makes `/dev/dri/render` devices available,
  required for later ROCm versions.

### Other changes

- Update minimum go version to 1.19.
- Fix non-root instance join with unprivileged systemd-managed cgroups v2,
  when join is from outside a user-owned cgroup.
- Fix joining cgroup of instance started as root, with cgroups v1,
  non-default cgroupfs manager, and no device rules.
- Avoid UID / GID / EUID readonly var warnings with `--env-file`.
- Ensure consistent binding of libraries under `--nv/--rocm` when duplicate
  `<library>.so[.version]` files are listed by `ldconfig -p`.
- Ensure `DOCKER_HOST` is honored in non-build flows.
- Corrected `apptainer.conf` comment, to refer to correct file as source
  of default capabilities when `root default capabilities = file`.
- Fix memory usage calculation during apptainer compilation on RaspberryPi.
- Fix misleading error when an overlay is requested by the root user while the
  overlay kernel module is not loaded.
- Fix gocryptfs build procedures for deb package.

## v1.1.9 - \[2023-06-07\]

- Remove warning about unknown `xino=on` option from fuse-overlayfs,
  introduced in 1.1.8.
- Ignore extraneous warning from fuse-overlayfs about a readonly `/proc`.
- Fix dropped "n" characters on some platforms in definition file stored as part
  of SIF metadata.
- Remove duplicated group ids.
- Fix not being able to handle multiple entries in `LD_PRELOAD` when
  binding fakeroot into container during apptainer startup for --fakeroot
  with fakeroot command.

## v1.1.8 - \[2023-04-25\]

### Security fix

- Included a fix for [CVE-2023-30549](https://github.com/apptainer/apptainer/security/advisories/GHSA-j4rf-7357-f4cg)
  which is a vulnerability in setuid-root installations of Apptainer
  and Singularity that causes an elevation in severity of an existing
  ext4 filesystem driver vulnerability that is unpatched in several
  older but still actively supported operating systems including RHEL7,
  Debian 10, Ubuntu 18.04 and Ubuntu 20.04.
  The fix adds `allow setuid-mount` configuration options `encrypted`,
  `squashfs`, and `extfs`, and makes the default for `extfs` be "no".
  That disables the use of extfs mounts including for overlays or
  binds while in the setuid-root mode, while leaving it enabled for
  unprivileged user namespace mode.
  The default for `encrypted` and `squashfs` is "yes".

### Other changes

- Fix loop device 'no such device or address' spurious errors when using shared
  loop devices.
- Remove unwanted colors to STDERR.
- Add `xino=on` mount option for writable kernel overlay mount points to fix
  inode numbers consistency after kernel cache flush (not applicable to
  fuse-overlayfs).

## v1.1.7 - \[2023-03-28\]

### Changes since last release

- Allow gpu options such as `--nv` to be nested by always inheriting all
  libraries bound in to a parent container's `/.singularity.d/libs`.
- Map the user's home directory to the root home directory by default in the
  non-subuid fakeroot mode like it was in the subuid fakeroot mode, for both
  action commands and building containers from definition files.
- Avoid `unknown option` error when using a bare squashfs image with
  an unpatched `squashfuse_ll`.
- Fix `GOCACHE` settings for golang build on PPA build environment.
- Make the error message more helpful in another place where a remote is found
  to have no library client.
- Allow symlinks to the compiled prefix for suid installations.  Fixes a
  regression introduced in 1.1.4.
- Build via zypper on SLE systems will use repositories of host via
  suseconnect-container.
- Avoid incorrect error when requesting fakeroot network.
- Pass computed `LD_LIBRARY_PATH` to wrapped unsquashfs. Fixes issues where
  `unsquashfs` on host uses libraries in non-default paths.

## v1.1.6 - \[2023-02-14\]

### Security fix

- Included a fix for [CVE-2022-23538](https://github.com/sylabs/scs-library-client/security/advisories/GHSA-7p8m-22h4-9pj7)
  which potentially leaked user credentials to a third-party S3 storage
  service when using the `library://` protocol.  See the link for details.

### Other changes

- Restored the ability for running instances to be tracked when apptainer
  is installed with tools/install-unprivileged.sh.  Instance tracking
  depends on argument 0 of the starter, which was not getting preserved.
- Fix `GOCACHE` environment variable settings when building debian source
  package on PPA build environment.
- Make `PS1` environment variable changeable via `%environment` section on
  definition file that used to be only changeable via `APPTAINERENV_PS1`
  outside of container. This makes the container's prompt customizable.
- Fix the passing of nested bind mounts when there are multiple binds
  separated by commas and some of them have colons separating sources
  and destinations.
- Added `Provides: bundled(golang())` statements to the rpm packaging
  for each bundled golang module.
- Hide messages about SINGULARITY variables if corresponding APPTAINER
  variables are defined. Fixes a regression introduced in 1.1.4.
- Print a warning if extra arguments are given to a shell action, and
  show in the run action usage that arguments may be passed.
- Check for the existence of the runtime executable prefix, to avoid
  issues when running under Slurm's srun. If it doesn't exist, fall
  back to the compile-time prefix.
- Increase the timeout on image driver (that is, FUSE) mounts from 2
  seconds to 10 seconds.  Instead, print an INFO message if it takes
  more than 2 seconds.
- If a `remote` is defined both globally (i.e. system-wide) and
  individually, change `apptainer remote` commands to print an info message
  instead of exiting with a fatal error and to give precedence to the
  individual configuration.

## v1.1.5 - \[2023-01-10\]

- Update the rpm packaging to (a) move the Obsoletes of singularity to
  the apptainer-suid packaging, (b) remove the Provides of singularity,
  (c) add a Provides and Conflicts for sif-runtime,
  (d) add "formerly known as Singularity" to the Summary,
  and (e) add a Conflicts of singularity to the apptainer package.
  Also update the debian and nfpm packaging with (d).
- Change rpm packaging to automatically import any modified configuration
  files in `/etc/singularity` when updating from singularity to apptainer,
  including importing `singularity.conf` using a new hidden `confgen`
  command.
- Fix the use of `fakeroot`, `faked`, and `libfakeroot.so` if they are not
  suffixed by `-sysv`, as is for instance the case on Gentoo Linux.
- Prevent the use of a `--libexecdir` or `--bindir` mconfig option from
  making apptainer think it was relocated and so preventing use of suid
  mode.  The bug was introduced in v1.1.4.
- Add helpful error message for build `--remote` option.
- Add more helpful error message when no library endpoint found.
- Avoid cleanup errors on exit when mountpoints are busy by doing a lazy
  unmount if a regular unmount doesn't work after 10 tries.
- Make messages about using SINGULARITY variables less scary.

## v1.1.4 - \[2022-12-12\]

- Added tools/install-unprivileged.sh to download and install apptainer
  binaries and all dependencies into a directory of the user's choice.
  Works on all currently active el, fedora, debian, and ubuntu versions
  except ubuntu 18.04, with all architectures supported by epel and fedora.
  Defaults to the latest version released in epel and fedora.
  Other apptainer versions can be selected but it only works with apptainer
  1.1.4 and later.
- Make the binaries built in the unprivileged `apptainer` package relocatable.
  When moving the binaries to a new location, the `/usr` at the top of some
  of the paths needs to be removed.  Relocation is disallowed when the
  `starter-suid` is present, for security reasons.
- Change the warning when an overlay image is not writable, introduced
  in v1.1.3, back into a (more informative) fatal error because it doesn't
  actually enter the container environment.
- Set the `--net` flag if `--network` or `--network-args` is set rather
  than silently ignoring them if `--net` was not set.
- Do not hang on pull from http(s) source that doesn't provide a content-length.
- Avoid hang on fakeroot cleanup under high load seen on some
  distributions / kernels.
- Remove obsolete pacstrap `-d` in Arch packer.
- Adjust warning message for deprecated environment variables usage.
- Enable the `--security uid:N` and `--security gid:N` options to work
  when run in non-suid mode.  In non-suid mode they work with any user,
  not just root.  Unlike with root and suid mode, however, only one gid
  may be set in non-suid mode.

## v1.1.3 - \[2022-10-25\]

- Prefer the `fakeroot-sysv` command over the `fakeroot` command because
  the latter can be linked to either `fakeroot-sysv` or `fakeroot-tcp`,
  but `fakeroot-sysv` is much faster.
- Update the included `squashfuse_ll` to have `-o uid=N` and `-o gid=N`
  options and changed the corresponding image driver to use them when
  available.  This makes files inside sif files appear to be owned by the
  user instead of by the nobody id 65534 when running in non-setuid mode.
- Fix the locating of shared libraries when running `unsquashfs` from a
  non-standard location.
- Properly clean up temporary files if `unsquashfs` fails.
- Fix the creation of missing bind points when using image binding with
  underlay.
- Change the error when an overlay image is not writable into a warning
  that suggests adding `:ro` to make it read only or using `--fakeroot`.
- Avoid permission denied errors during unprivileged builds without
  `/etc/subuid`-based fakeroot when `/var/lib/containers/sigstore` is
  readable only by root.
- Avoid failures with `--writable-tmpfs` in non-setuid mode when using
  fuse-overlayfs versions 1.8 or greater by adding the fuse-overlayfs
  `noacl` mount option to disable support for POSIX Access Control Lists.
- Fix the `--rocm` flag in combination with `-c` / `-C` by forwarding all
  `/dri/render*` devices into the container.

## v1.1.2 - \[2022-10-06\]

- [CVE-2022-39237](https://github.com/sylabs/sif/security/advisories/GHSA-m5m3-46gj-wch8):
  The sif dependency included in Apptainer before this release does not
  verify that the hash algorithm(s) used are cryptographically secure
  when verifying digital signatures. This release updates to sif v2.8.1
  which corrects this issue. See the linked advisory for references and
  a workaround.

## v1.1.1 - \[2022-10-06\]

Accidentally included no code changes.

## v1.1.0 - \[2022-09-27\]

### Changed defaults / behaviours

- The most significant change is that Apptainer no longer installs a
  setuid-root portion by default.
  This is now reasonable to do because most operations can be done with
  only unprivileged user namespaces (see additional changes below).
  If installing from rpm or debian packages, the setuid portion can be
  included by installing the `apptainer-suid` package, or if installing
  from source it can be included by compiling with the mconfig
  `--with-suid` option.
  For those that are concerned about kernel vulnerabilities with user
  namespaces, we recommend disabling network namespaces if you can.
  See the [discussion in the admin guide](https://apptainer.org/docs/admin/main/user_namespace.html#disabling-network-namespaces).
- Added a squashfuse image driver that enables mounting SIF files without
  using setuid-root.  Uses either a squashfuse_ll command or a
  squashfuse command and requires unprivileged user namespaces.
  For better parallel performance, a patched multithreaded version of
  `squashfuse_ll` is included in rpm and debian packaging in
  `${prefix}/libexec/apptainer/bin`.
- Added an `--unsquash` action flag to temporarily convert a SIF file to a
  sandbox before running.  In previous versions this was the default when
  running a SIF file without setuid or with fakeroot, but now the default
  is to mount with squashfuse_ll or squashfuse.
- Added a fuse2fs image driver that enables mounting EXT3 files and EXT3
  SIF overlay partitions without using setuid-root.  Requires the fuse2fs
  command and unprivileged user namespaces.
- Added the ability to use persistent overlay (`--overlay`) and
  `--writable-tmpfs` without using setuid-root.
  This requires unprivileged user namespaces and either a new enough
  kernel (>= 5.11) or the fuse-overlayfs command.
  Persistent overlay works when the overlay path points to a regular
  filesystem (known as "sandbox" mode, which is not allowed when in
  setuid mode), or when it points to an EXT3 image.
- Extended the `--fakeroot` option to be useful when `/etc/subuid` and
  `/etc/subgid` mappings have not been set up.
  If they have not been set up, a root-mapped unprivileged user namespace
  (the equivalent of `unshare -r`) and/or the fakeroot command from the
  host will be tried.
  Together they emulate the mappings pretty well but they are simpler to
  administer.
  This feature is especially useful with the `--overlay` and
  `--writable-tmpfs` options and for building containers unprivileged,
  because they allow installing packages that assume they're running
  as root.
  A limitation on using it with `--overlay` and `--writable-tmpfs`
  however is that when only the fakeroot command can be used (because
  there are no user namespaces available, in suid mode) then the base
  image has to be a sandbox.
  This feature works nested inside of an apptainer container, where
  another apptainer command will also be in the fakeroot environment
  without requesting the `--fakeroot` option again, or it can be used
  inside an apptainer container that was not started with `--fakeroot`.
  However, the fakeroot command uses LD_PRELOAD and so needs to be bound
  into the container which requires a compatible libc.
  For that reason it doesn't work when the host and container operating
  systems are of very different vintages.
  If that's a problem and you want to use only an unprivileged
  root-mapped namespace even when the fakeroot command is installed,
  just run apptainer with `unshare -r`.
- Made the `--fakeroot` option be implied when an unprivileged user
  builds a container from a definition file.
  When `/etc/subuid` and `/etc/subgid` mappings are not available,
  all scriptlets are run in a root-mapped unprivileged namespace (when
  possible) and the %post scriptlet is additionally run with the fakeroot
  command.
  When unprivileged user namespaces are not available, such that only
  the fakeroot command can be used, the `--fix-perms` option is implied
  to allow writing into directories.
- Added additional hidden options to action and build commands for testing
  different fakeroot modes: `--ignore-subuid`, `--ignore-fakeroot-command`,
  and `--ignore-userns`.
  Also added `--userns` to the build command to ignore setuid-root mode
  like action commands do.
- Added a `--fakeroot` option to the `apptainer overlay create` command
  to make an overlay EXT3 image file that works with the fakeroot that
  comes from unprivileged root-mapped namespaces.
  This is not needed with the fakeroot that comes with `/etc/sub[ug]id`
  mappings nor with the fakeroot that comes with only the fakeroot
  command in suid flow.
- Added a `--sparse` flag to `overlay create` command to allow generation of
  a sparse EXT3 overlay image.
- Added a `binary path` configuration variable as the default path to use
  when searching for helper executables.  May contain `$PATH:` which gets
  substituted with the user's PATH except when running a program that may
  be run with elevated privileges in the suid flow.
  Defaults to `$PATH:` followed by standard system paths.
  `${prefix}/libexec/apptainer/bin` is also implied as the first component,
  either as the first directory of `$PATH` if present or simply as the
  first directory if `$PATH` is not included.
  Configuration variables for paths to individual programs that were in
  apptainer.conf (`cryptsetup`, `go`, `ldconfig`, `msquashfs`, `unsquashfs`,
  and `nvidia-container-cli`) have been removed.
- The `--nvccli` option now works without `--fakeroot`.  In that case the
  option can be used with `--writable-tmpfs` instead of `--writable`,
  and `--writable-tmpfs` is implied if neither option is given.
  Note that also `/usr/bin` has to be writable by the user, so without
  `--fakeroot` that probably requires a sandbox image that was built with
  `--fix-perms`.
- The `--nvccli` option now implies `--nv`.
- $HOME is now used to find the user's configuration and cache by default.
  If that is not set it will fall back to the previous behavior of looking
  up the home directory in the password file.  The value of $HOME inside
  the container still defaults to the home directory in the password file
  and can still be overridden by the ``--home`` option.
- When starting a container, if the user has specified the cwd by using
  the `--pwd` flag, if there is a problem an error is returned instead
  of defaulting to a different directory.
- Nesting of bind mounts now works even when a `--bind` option specified
  a different source and destination with a colon between them.  Now the
  APPTAINER_BIND environment variable makes sure the bind source is
  from the bind destination so it will be successfully re-bound into a
  nested apptainer container.
- The warning about more than 50 bind mounts required for an underlay bind
  has been changed to an info message.
- `oci mount` sets `Process.Terminal: true` when creating an OCI `config.json`,
  so that `oci run` provides expected interactive behavior by default.
- The default hostname for `oci mount` containers is now `apptainer` instead of
  `mrsdalloway`.
- systemd is now supported and used as the default cgroups manager. Set
  `systemd cgroups = no` in `apptainer.conf` to manage cgroups directly via
  the cgroupfs.
- Plugins must be compiled from inside the Apptainer source directory,
  and will use the main Apptainer `go.mod` file. Required for Go 1.18
  support.
- Apptainer now requires squashfs-tools >=4.3, which is satisfied by
  current EL / Ubuntu / Debian and other distributions.
- Added a new action flag `--no-eval` which:
  - Prevents shell evaluation of `APPTAINERENV_ / --env / --env-file`
    environment variables as they are injected in the container, to match OCI
    behavior. *Applies to all containers*.
  - Prevents shell evaluation of the values of `CMD / ENTRYPOINT` and command
    line arguments for containers run or built directly from an OCI/Docker
    source. *Applies to newly built containers only, use `apptainer inspect`
    to check version that container was built with*.
- Added `--no-eval` to the list of flags set by the OCI/Docker `--compat` mode.
- `sinit` process has been renamed to `appinit`.
- Added `--keysdir` to `key` command to provide an alternative way of setting
  local keyring path. The existing reading of the keyring path from
  environment variable 'APPTAINER_KEYSDIR' is untouched.
- `apptainer key push` will output the key server's response if included in
  order to help guide users through any identity verification the server may
  require.
- ECL no longer requires verification for all signatures, but only
  when signature verification would alter the expected behavior of the
  list:
  - At least one matching signature included in a whitelist must be
    validated, but other unvalidated signatures do not cause ECL to
    fail.
  - All matching signatures included in a whitestrict must be
    validated, but unvalidated signatures not in the whitestrict do
    not cause ECL to fail.
  - Signature verification is not checked for a blacklist; unvalidated
    signatures can still block execution via ECL, and unvalidated
    signatures not in the blacklist do not cause ECL to fail.
- Improved wildcard matching in the %files directive of build definition
  files by replacing usage of sh with the mvdan.cc library.

### New features / functionalities

- Non-root users can now use `--apply-cgroups` with `run/shell/exec` to limit
  container resource usage on a system using cgroups v2 and the systemd cgroups
  manager.
- Native cgroups v2 resource limits can be specified using the `[unified]` key
  in a cgroups toml file applied via `--apply-cgroups`.
- Added `--cpu*`, `--blkio*`, `--memory*`, `--pids-limit` flags to apply cgroups
  resource limits to a container directly.
- Added instance stats command.
- Added support for a custom hashbang in the `%test` section of an Apptainer
  recipe (akin to the runscript and start sections).
- The `--no-mount` flag & `APPTAINER_NO_MOUNT` env var can now be used to
  disable a `bind path` entry from `apptainer.conf` by specifying the
  absolute path to the destination of the bind.
- Apptainer now supports the `riscv64` architecture.
- `remote add --insecure` may now be used to configure endpoints that are only
  accessible via http. Alternatively the environment variable
  `APPTAINER_ADD_INSECURE` can be set to true to allow http remotes to be
  added without the `--insecure` flag. Specifying https in the remote URI
  overrules both `--insecure` and `APPTAINER_ADD_INSECURE`.
- Gpu flags `--nv` and `--rocm` can now be used from an apptainer nested
  inside another apptainer container.
- Added `--public`, `--secret`, and `--both` flags to the `key remove` command
  to support removing secret keys from the apptainer keyring.
- Debug output can now be enabled by setting the `APPTAINER_DEBUG` env var.
- Debug output is now shown for nested `apptainer` calls, in wrapped
  `unsquashfs` image extraction, and build stages.
- Added EL9 package builds to CI for GitHub releases.
- Added confURL & Include parameters to the Arch packer for alternate
  `pacman.conf` URL and alternate installed (meta)package.

### Bug fixes

- Remove warning message about SINGULARITY and APPTAINER variables having
  different values when the SINGULARITY variable is not set.
- Fixed longstanding bug in the underlay logic when there are nested bind
  points separated by more than one path level, for example `/var` and
  `/var/lib/yum`, and the path didn't exist in the container image.
  The bug only caused an error when there was a directory in the container
  image that didn't exist on the host.
- Add specific error for unreadable image / overlay file.
- Pass through a literal `\n` in host environment variables to the container.
- Allow `newgidmap / newuidmap` that use capabilities instead of setuid root.
- Fix compilation on `mipsel`.
- Fix test code that implied `%test -c <shell>` was supported - it is not.
- Fix loop device creation with loop-control when running inside docker
  containers.
- Fix the issue that the oras protocol would ignore the `--no-https/--nohttps`
  flag.
- Fix oras image push to registries with authorization servers not supporting
  multiple scope query parameter.
- Improved error handling of unsupported password protected PEM files with
  encrypted containers.
- Ensure bootstrap_history directory is populated with previous definition
  files, present in source containers used in a build.

## v1.0.3 - \[2022-07-06\]

### Bug fixes

- Process redirects that can come from sregistry with a `library://` URL.
- Fix `inspect --deffile` and `inspect --all` to correctly show definition
  files in sandbox container images instead of empty output.
  This has a side effect of also fixing the storing of definition files in
  the metadata of sif files built by Apptainer, because that metadata is
  constructed by doing `inspect --all`.

## v1.0.2 - \[2022-05-09\]

### Bug fixes

- Fixed `FATAL` error thrown by user configuration migration code that caused
  users with inaccessible home directories to be unable to use `apptainer`
  commands.
- The Debian package now conflicts with the singularity-container package.
- Do not truncate environment variables with commas.
- Use HEAD request when checking digest of remote OCI image sources, with GET as
  a fall-back. Greatly reduces Apptainer's impact on Docker Hub API limits.

## v1.0.1 - \[2022-03-15\]

### Bug fixes

- Don't prompt for y/n to overwrite an existing file when build is
  called from a non-interactive environment. Fail with an error.
- Preload NSS libraries prior to mountspace name creation to avoid
  circumstances that can cause loading those libraries from the
  container image instead of the host, for example in the startup
  environment.
- Fix race condition where newly created loop devices can sometimes not
  be opened.
- Support nvidia-container-cli v1.8.0 and above, via fix to capability set.

## v1.0.0 - \[2022-03-02\]

### Comparison to SingularityCE

This release candidate has most of the new features, bug fixes, and
changes that went into SingularityCE up through their version 3.9.5,
except where the maintainers of Apptainer disagreed with what went into
SingularityCE since the project fork.  The biggest difference is that
Apptainer does not support the --nvccli option in privileged mode.  This
release also has the additional major feature of instance checkpointing
which isn't in SingularityCE.  Other differences due to re-branding are
in the next section.

### Changes due to the project re-branding

- The primary executable has been changed from `singularity` to `apptainer`.
  However, a `singularity` command symlink alias has been created pointing
  to the `apptainer` command.  The contents of containers are unchanged
  and continue to use the singularity name for startup scripts, etc.
- The configuration directory has changed from `/etc/singularity` to
  `/etc/apptainer` within packages, and the primary configuration
  file name has changed from `singularity.conf` to `apptainer.conf`.
  As long as a `singularity` directory still exists next to an
  `apptainer` directory, running the `apptainer` command will print
  a warning saying that migration is not complete.  If no changes had
  been made to the configuration then an rpm package upgrade should
  automatically remove the old directory, otherwise the system
  administrator needs to take care of migrating the configuration
  and removing the old directory.  Old configuration can be removed
  for a Debian package with `apt-get purge singularity` or
  `dpkg -P singularity`.
- The per-user configuration directory has changed from `~/.singularity`
  to `~/.apptainer`.  The first time the `apptainer` command accesses the
  user configuration directory, relevant configuration is automatically
  imported from the old directory to the new one.
- Environment variables have all been changed to have an `APPTAINER`
  prefix instead of a `SINGULARITY` prefix.  However, `SINGULARITY`
  prefix variables are still recognized.  If only a `SINGULARITY`
  prefix variable exists, a warning will be printed about deprecated
  usage and then the value will be used.  If both prefixes exist and
  the value is the same, no warning is printed; this is the recommended
  method to set environment variables for those who need to support both
  `apptainer` and `singularity`.  If both prefixes exist for the same
  variable and the value is different then a warning is also printed.
- The default SylabsCloud remote endpoint has been removed and replaced
  by one called DefaultRemote which has no defined server for the
  `library://` URI.  The previous default can be restored by following
  the directions in the
  [documentation](https://apptainer.org/docs/user/1.0/endpoint.html#restoring-pre-apptainer-library-behavior).
- The DefaultRemote's key server is `https://keys.openpgp.org`
  instead of the Sylabs key server.
- The `apptainer build --remote` option has been removed because there
  is no standard protocol or non-commercial service that supports it.

### Other changed defaults / behaviours since Singularity 3.8.x

- Auto-generate release assets including the distribution tarball and
  rpm (built on CentOS 7) and deb (built on Debian 11) x86_64 packages.
- LABELs from Docker/OCI images are now inherited. This fixes a longstanding
  regression from Singularity 2.x. Note that you will now need to use `--force`
  in a build to override a label that already exists in the source Docker/OCI
  container.
- Removed `--nonet` flag, which was intended to disable networking for in-VM
  execution, but has no effect.
- `--nohttps` flag has been deprecated in favour of `--no-https`. The old flag
  is still accepted, but will display a deprecation warning.
- Paths for `cryptsetup`, `go`, `ldconfig`, `mksquashfs`, `nvidia-container-cli`,
  `unsquashfs` are now found at build time by `mconfig` and written into
  `apptainer.conf`. The path to these executables can be overridden by
  changing the value in `apptainer.conf`.
- When calling `ldconfig` to find GPU libraries, apptainer will *not* fall back
  to `/sbin/ldconfig` if the configured `ldconfig` errors. If installing in a
  Guix/Nix on environment on top of a standard host distribution you *must* set
  `ldconfig path = /sbin/ldconfig` to use the host distribution `ldconfig` to
  find GPU libraries.
- `--nv` will not call `nvidia-container-cli` to find host libraries, unless
  the new experimental GPU setup flow that employs `nvidia-container-cli`
  for all GPU related operations is enabled (see more below).
- If a container is run with `--nvccli` and `--contain`, only GPU devices
  specified via the `NVIDIA_VISIBLE_DEVICES` environment variable will be
  exposed within the container. Use `NVIDIA_VISIBLE_DEVICES=all` to access all
  GPUs inside a container run with `--nvccli`.  See more on `--nvccli` under
  New features below.
- Example log-plugin rewritten as a CLI callback that can log all commands
  executed, instead of only container execution, and has access to command
  arguments.
- The bundled reference CNI plugins are updated to v1.0.1. The `flannel` plugin
  is no longer included, as it is maintained as a separate plugin at:
  <https://github.com/flannel-io/cni-plugin>. If you use the flannel CNI plugin
  you should install it from this repository.
- Instances are no longer created with an IPC namespace by default. An IPC
  namespace can be specified with the `-i|--ipc` flag.
- The behaviour of the `allow container` directives in `apptainer.conf` has
  been modified, to support more intuitive limitations on the usage of SIF and non-SIF
  container images. If you use these directives, *you may need to make changes
  to apptainer.conf to preserve behaviour*.
  - A new `allow container sif` directive permits or denies usage of
    *unencrypted* SIF images, irrespective of the filesystem(s) inside the SIF.
  - The `allow container encrypted` directive permits or denies usage of SIF
    images with an encrypted root filesystem.
  - The `allow container squashfs/extfs` directives in `apptainer.conf`
    permit or deny usage of bare SquashFS and EXT image files only.
  - The effect of the `allow container dir` directive is unchanged.
- `--bind`, `--nv` and `--rocm` options for `build` command can't be set through
  environment variables `APPTAINER_BIND`, `APPTAINER_BINDPATH`, `APPTAINER_NV`,
  `APPTAINER_ROCM` anymore due to side effects reported by users in this
  [issue](https://github.com/apptainer/singularity/pull/6211),
  they must be explicitly requested via command line.
- Build `--bind` option allows to set multiple bind mounts without specifying
  the `--bind` option for each bindings.
- Honor image binds and user binds in the order they're given instead of
  always doing image binds first.
- Remove subshell overhead when processing large environments on container
  startup.
- `make install` now installs man pages. A separate `make man` is not
  required.  As a consequence, man pages are now included in deb packages.

### New features / functionalities

- Experimental support for checkpointing of instances using DMTCP has been
  added.  Additional flags `--dmtcp-launch` and `--dmtcp-restart` has
  been added to the `apptainer instance start` command, and a `checkpoint`
  command group has been added to manage the checkpoint state.  A new
  `/etc/apptainer/dmtcp-conf.yaml` configuration file is also added.
  Limitations are that it can only work with dynamically linked
  applications and the container has to be based on `glibc`.
- `--writable-tmpfs` can be used with `apptainer build` to run the `%test`
  section of the build with a ephemeral tmpfs overlay, permitting tests that
  write to the container filesystem.
- The `--compat` flag for actions is a new short-hand to enable a number of
  options that increase OCI/Docker compatibility. Infers `--containall,
  --no-init, --no-umask, --writable-tmpfs`. Does not use user, uts, or
  network namespaces as these may not be supported on many installations.
- The experimental `--nvccli` flag will use `nvidia-container-cli` to setup the
  container for Nvidia GPU operation. Apptainer will not bind GPU libraries
  itself. Environment variables that are used with Nvidia's `docker-nvidia`
  runtime to configure GPU visibility / driver capabilities & requirements are
  parsed by the `--nvccli` flag from the environment of the calling user. By
  default, the `compute` and `utility` GPU capabilities are configured. The `use
  nvidia-container-cli` option in `apptainer.conf` can be set to `yes` to
  always use `nvidia-container-cli` when supported.
  `--nvccli` is not supported in the setuid workflow,
  and it requires being used in combination with `--writable` in user
  namespace mode.
  Please see documentation for more details.
- The `--apply-cgroups` flag can be used to apply cgroups resource and device
  restrictions on a system using the v2 unified cgroups hierarchy. The resource
  restrictions must still be specified in the v1 / OCI format, which will be
  translated into v2 cgroups resource restrictions, and eBPF device
  restrictions.
- A new `--mount` flag and `APPTAINER_MOUNT` environment variable can be used
  to specify bind mounts in
  `type=bind,source=<src>,destination=<dst>[,options...]` format. This improves
  CLI compatibility with other runtimes, and allows binding paths containing
  `:` and `,` characters (using CSV style escaping).
- Perform concurrent multi-part downloads for `library://` URIs. Uses 3
  concurrent downloads by default, and is configurable in `apptainer.conf` or
  via environment variables.

### Bug fixes

- The `oci` commands will operate on systems that use the v2 unified cgroups
  hierarchy.
- Ensure invalid values passed to `config global --set` cannot lead to an empty
  configuration file being written.
- `--no-https` now applies to connections made to library services specified
  in `library://<hostname>/...` URIs.
- Ensure `gengodep` in build uses vendor dir when present.
- Correct documentation for sign command r.e. source of key index.
- Restructure loop device discovery to address `EAGAIN` issue.
- Ensure a local build does not fail unnecessarily if a keyserver
  config cannot be retrieved from the remote endpoint.
- Update dependency to correctly unset variables in container startup
  environment processing. Fixes regression introduced in singularity-3.8.5.
- Correct library bindings for `unsquashfs` containment. Fixes errors where
  resolved library filename does not match library filename in binary
  (e.g. EL8, POWER9 with glibc-hwcaps).
- Remove python as a dependency of the debian package.
- Increase the TLS Handshake Timeout for the busybox bootstrap agent in
  build definition files to 60 seconds.
- Add binutils-gold to the build requirements on SUSE rpm builds.

### Changes for Testing / Development

- `E2E_DOCKER_MIRROR` and `E2E_DOCKER_MIRROR_INSECURE` were added to allow
  to use a registry mirror (or a pull through cache).
- A `tools` source directory was added with a Dockerfile for doing local
  e2e testing.
