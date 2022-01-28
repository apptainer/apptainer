# Apptainer Changelog

The Singularity Project has been
[adopted by the Linux Foundation](https://www.linuxfoundation.org/press-release/new-linux-foundation-project-accelerates-collaboration-on-container-systems-between-enterprise-and-high-performance-computing-environments/)
and re-branded as Apptainer.
For older changes see the [archived Singularity change log](https://github.com/apptainer/singularity/blob/release-3.8/CHANGELOG.md).

## Changes since last release

- The `SINGULARITY_LABELS` environment variable within build definitions has
  been restored.
- Fix mount ordering between image bind and user binds.

## v1.0.0 Release Candidate 1 - \[22-01-19\]

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
  `library://` URI.  System administrators may restore the old default
  if they wish by adding it to `/etc/apptainer/remote.yaml` with a URI
  of `cloud.sylabs.io` and setting it there as the `Active` remote, or
  users can add it to their own configuration with the commands
  `apptainer remote add SylabsCloud cloud.sylabs.io` and
  `apptainer remote use SylabsCloud`.
- The DefaultRemote's key server is `https://keys.openpgp.org`
  instead of the Sylabs key server.
- The `apptainer build --remote` option has been removed because there
  is no standard protocol or non-commercial service that supports it.

### Other changed defaults / behaviours since Singularity 3.8.x

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
  container images. If you use these directives, _you may need to make changes
  to apptainer.conf to preserve behaviour_.
  - A new `allow container sif` directive permits or denies usage of
    _unencrypted_ SIF images, irrespective of the filesystem(s) inside the SIF.
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

### Changes for Testing / Development

- `E2E_DOCKER_MIRROR` and `E2E_DOCKER_MIRROR_INSECURE` were added to allow
  to use a registry mirror (or a pull through cache).
- A `tools` source directory was added with a Dockerfile for doing local
  e2e testing.
