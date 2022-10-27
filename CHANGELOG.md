# Apptainer Changelog

The Singularity Project has been
[adopted by the Linux Foundation](https://www.linuxfoundation.org/press-release/new-linux-foundation-project-accelerates-collaboration-on-container-systems-between-enterprise-and-high-performance-computing-environments/)
and re-branded as Apptainer.
For older changes see the [archived Singularity change log](https://github.com/apptainer/singularity/blob/release-3.8/CHANGELOG.md).

## Changes Since Last Release

### New features / functionalities

- The `--no-mount` flag now accepts the value `bind-paths` to disable mounting
  of all `bind path` entries in `apptainer.conf`.
- Instances started by a non-root user can use `--apply-cgroups` to apply
  resource limits. Requires cgroups v2, and delegation configured via systemd.
- The `instance stats` command displays the resource usage every second. The
  `--no-stream` option disables this interactive mode and shows the
  point-in-time usage.
- Support for `DOCKER_HOST` parsing when using `docker-daemon://`
- `DOCKER_USERNAME` and `DOCKER_PASSWORD` supported without `APPTAINER_` prefix.
- Add new Linux capabilities: `CAP_PERFMON`, `CAP_BPF`, `CAP_CHECKPOINT_RESTORE`.

### Bug fixes

- Set the `--net` flag if `--network` or `--network-args` is set rather
  than silently ignoring them if `--net` was not set.

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

### Changes since last release

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
  from the bind destination so it will be succesfully re-bound into a
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
  added wihtout the `--insecure` flag. Specifying https in the remote URI
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
  they must be explicitely requested via command line.
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
