// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package launch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/cgroups"
	"github.com/apptainer/apptainer/internal/pkg/checkpoint/dmtcp"
	"github.com/apptainer/apptainer/internal/pkg/fakeroot"
	"github.com/apptainer/apptainer/internal/pkg/image/driver"
	"github.com/apptainer/apptainer/internal/pkg/image/unpacker"
	"github.com/apptainer/apptainer/internal/pkg/instance"
	"github.com/apptainer/apptainer/internal/pkg/plugin"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/apptainer/apptainer/internal/pkg/security"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/fs/squashfs"
	"github.com/apptainer/apptainer/internal/pkg/util/gpu"
	"github.com/apptainer/apptainer/internal/pkg/util/shell/interpreter"
	"github.com/apptainer/apptainer/internal/pkg/util/starter"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/apptainer/apptainer/pkg/build/types"
	imgutil "github.com/apptainer/apptainer/pkg/image"
	clicallback "github.com/apptainer/apptainer/pkg/plugin/callback/cli"
	apptainercallback "github.com/apptainer/apptainer/pkg/plugin/callback/runtime/engine/apptainer"
	apptainerConfig "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
	"github.com/apptainer/apptainer/pkg/syfs"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/cryptkey"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
	"github.com/apptainer/apptainer/pkg/util/rlimit"
	lccgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

func NewLauncher(opts ...Option) (*Launcher, error) {
	lo := launchOptions{}
	for _, opt := range opts {
		if err := opt(&lo); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}

	// Initialize empty default Apptainer Engine and OCI configuration
	engineConfig := apptainerConfig.NewConfig()
	imageArg := os.Getenv("IMAGE_ARG")
	os.Unsetenv("IMAGE_ARG")
	engineConfig.SetImageArg(imageArg)
	engineConfig.File = apptainerconf.GetCurrentConfig()
	if engineConfig.File == nil {
		return nil, fmt.Errorf("unable to get apptainer configuration")
	}
	ociConfig := &oci.Config{}
	generator := generate.New(&ociConfig.Spec)
	engineConfig.OciConfig = ociConfig

	l := Launcher{
		uid:          uint32(os.Getuid()),
		gid:          uint32(os.Getgid()),
		cfg:          lo,
		engineConfig: engineConfig,
		generator:    generator,
	}

	return &l, nil
}

// Exec prepares an EngineConfig defining how a container should be launched, then calls the starter binary to execute it.
// This includes interactive containers, instances, and joining an existing instance.
//
//nolint:maintidx
func (l *Launcher) Exec(ctx context.Context, image string, args []string, instanceName string) error {
	var err error

	var fakerootPath string
	if l.cfg.Fakeroot {
		if (l.uid == 0) && namespaces.IsUnprivileged() {
			// Already running root-mapped unprivileged
			l.cfg.Fakeroot = false
			// Setting the following line with `false` value
			// will prevent Apptainer from allocating an additional user namespace.
			// Here, Apptainer is already running inside a root-mapped namespace,
			// i.e., similar to running with `unshare -r`, setting the following
			// line with `true` value will make Apptainer run inside a nested
			// root-mapped namespace, similar to `unshare -r unshare -r`
			l.cfg.Namespaces.User = false
			sylog.Debugf("running root-mapped unprivileged")
			var err error
			if l.cfg.IgnoreFakerootCmd {
				err = errors.New("fakeroot command is ignored because of --ignore-fakeroot-command")
			} else {
				fakerootPath, err = fakeroot.FindFake()
			}
			if err != nil {
				sylog.Infof("fakeroot command not found, using only root-mapped namespace")
			} else {
				sylog.Infof("Using fakeroot command combined with root-mapped namespace")
			}
		} else if (l.uid != 0) && (!fakeroot.IsUIDMapped(l.uid) || l.cfg.IgnoreSubuid) {
			sylog.Infof("User not listed in %v, trying root-mapped namespace", fakeroot.SubUIDFile)
			l.cfg.Fakeroot = false
			var err error
			if l.cfg.IgnoreUserns {
				err = errors.New("could not start root-mapped namespace because --ignore-userns is set")
			} else {
				err = fakeroot.UnshareRootMapped(os.Args, false)
			}
			if err == nil {
				// All good
				os.Exit(0)
			}
			sylog.Debugf("UnshareRootMapped failed: %v", err)
			if l.cfg.IgnoreFakerootCmd {
				err = errors.New("fakeroot command is ignored because of --ignore-fakeroot-command")
			} else {
				fakerootPath, err = fakeroot.FindFake()
			}
			if err != nil {
				sylog.Fatalf("--fakeroot requires either being in %v, unprivileged user namespaces, or the fakeroot command", fakeroot.SubUIDFile)
			}
			notSandbox := false
			if strings.Contains(image, "://") {
				notSandbox = true
			} else {
				info, err := os.Stat(image)
				if err == nil && !info.Mode().IsDir() {
					notSandbox = true
				}
			}
			if notSandbox {
				sylog.Infof("No user namespaces available")
				sylog.Infof("The fakeroot command by itself is only useful with sandbox images")
				sylog.Infof(" which can be built with 'apptainer build --sandbox'")
				sylog.Fatalf("--fakeroot used without sandbox image or user namespaces")
			}
			sylog.Infof("No user namespaces available, using only the fakeroot command")
		}
	}

	// Set arguments to pass to contained process.
	l.generator.SetProcessArgs(args)

	// NoEval means we will not shell evaluate args / env in action scripts and environment processing.
	// This replicates OCI behavior and differs from historic Apptainer behavior.
	if l.cfg.NoEval {
		l.engineConfig.SetNoEval(true)
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NO_EVAL", "1")
	}

	// Set container Umask w.r.t. our own, before any umask manipulation happens.
	l.setUmask()

	insideUserNs, _ := namespaces.IsInsideUserNamespace(os.Getpid())

	// Will we use the suid starter? If not we need to force the user namespace.
	useSuid := l.useSuid(insideUserNs)
	// IgnoreUserns is a hidden control flag
	l.cfg.Namespaces.User = l.cfg.Namespaces.User && !l.cfg.IgnoreUserns

	// Get our effective uid and gid for container execution.
	// If user requests a target uid, gid via --security options, handle them now.
	err = l.setTargetIDs(useSuid)
	if err != nil {
		sylog.Fatalf("Could not configure target UID/GID: %s", err)
	}

	// Set image to run, or instance to join, and APPTAINER_CONTAINER/APPTAINER_NAME env vars.
	if err := l.setImageOrInstance(image, instanceName); err != nil {
		sylog.Fatalf("While setting image/instance: %s", err)
	}

	// Overlay or writable image requested?
	l.engineConfig.SetOverlayImage(l.cfg.OverlayPaths)
	l.engineConfig.SetWritableImage(l.cfg.Writable)

	// Prefer underlay for bind
	l.engineConfig.SetUnderlay(l.cfg.Underlay)

	// Check key is available for encrypted image, if applicable.
	// If we are joining an instance, then any encrypted image is already mounted.
	if !l.engineConfig.GetInstanceJoin() {
		err = l.checkEncryptionKey()
		if err != nil {
			sylog.Fatalf("While checking container encryption: %s", err)
		}
	}

	// In the setuid workflow, set RLIMIT_STACK to its default value, keeping the
	// original value to restore it before executing the container process.
	if useSuid {
		soft, hard, err := rlimit.Get("RLIMIT_STACK")
		if err != nil {
			sylog.Warningf("can't retrieve stack size limit: %s", err)
		}
		l.generator.AddProcessRlimits("RLIMIT_STACK", hard, soft)
	}

	// Handle requested binds, fuse mounts.
	if err := l.setBinds(fakerootPath); err != nil {
		sylog.Fatalf("While setting bind mount configuration: %s", err)
	}
	if err := l.setFuseMounts(); err != nil {
		sylog.Fatalf("While setting FUSE mount configuration: %s", err)
	}

	// Set the home directory that should be effective in the container.
	if err := l.setHome(); err != nil {
		sylog.Fatalf("While setting home directory: %s", err)
	}
	// Allow user to disable the home mount via --no-home.
	l.engineConfig.SetNoHome(l.cfg.NoHome)
	// Allow user to disable binds via --no-mount.
	l.setNoMountFlags()

	// GPU configuration may add library bind to /.singularity.d/libs.
	// Note: --nvccli may implicitly add --writable-tmpfs, so handle that *after* GPUs.
	if err := l.SetGPUConfig(); err != nil {
		sylog.Fatalf("While setting GPU configuration: %s", err)
	}

	if err := l.SetCheckpointConfig(); err != nil {
		sylog.Fatalf("while setting checkpoint configuration: %s", err)
	}

	// --writable-tmpfs is for an ephemeral overlay, doesn't make sense if also asking to write to image itself.
	if l.cfg.Writable && l.cfg.WritableTmpfs {
		sylog.Warningf("Disabling --writable-tmpfs flag, mutually exclusive with --writable")
		l.engineConfig.SetWritableTmpfs(false)
	} else {
		l.engineConfig.SetWritableTmpfs(l.cfg.WritableTmpfs)
	}

	// Additional user requested library binds into /.singularity.d/libs.
	l.engineConfig.AppendLibrariesPath(l.cfg.ContainLibs...)

	// Additional directory overrides.
	l.engineConfig.SetScratchDir(l.cfg.ScratchDirs)
	l.engineConfig.SetWorkdir(l.cfg.WorkDir)
	l.engineConfig.SetConfigDir(syfs.ConfigDir())

	// Container networking configuration.
	l.engineConfig.SetNetwork(l.cfg.Network)
	l.engineConfig.SetDNS(l.cfg.DNS)
	l.engineConfig.SetNetworkArgs(l.cfg.NetworkArgs)

	// If user wants to set a hostname, it requires the UTS namespace.
	if l.cfg.Hostname != "" {
		l.cfg.Namespaces.UTS = true
		l.engineConfig.SetHostname(l.cfg.Hostname)
	}

	// Set requested capabilities (effective for root, or if sysadmin has permitted to another user).
	l.engineConfig.SetAddCaps(l.cfg.AddCaps)
	l.engineConfig.SetDropCaps(l.cfg.DropCaps)

	// Custom --config file (only effective in non-setuid or as root).
	l.engineConfig.SetConfigurationFile(l.cfg.ConfigFile)

	l.engineConfig.SetUseBuildConfig(l.cfg.UseBuildConfig)

	// When running as root, the user can optionally allow setuid with container.
	err = withPrivilege(l.uid, l.cfg.AllowSUID, "--allow-setuid", func() error {
		l.engineConfig.SetAllowSUID(l.cfg.AllowSUID)
		return nil
	})
	if err != nil {
		sylog.Fatalf("Could not configure --allow-setuid: %s", err)
	}

	// When running as root, the user can optionally keep all privs in the container.
	err = withPrivilege(l.uid, l.cfg.KeepPrivs, "--keep-privs", func() error {
		l.engineConfig.SetKeepPrivs(l.cfg.KeepPrivs)
		return nil
	})
	if err != nil {
		sylog.Fatalf("Could not configure --keep-privs: %s", err)
	}

	// User can optionally force dropping all privs from root in the container.
	l.engineConfig.SetNoPrivs(l.cfg.NoPrivs)

	// Set engine --security options (selinux, apparmor, seccomp functionality).
	l.engineConfig.SetSecurity(l.cfg.SecurityOpts)

	// User can override shell used when entering container.
	l.engineConfig.SetShell(l.cfg.ShellPath)
	if l.cfg.ShellPath != "" {
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "SHELL", l.cfg.ShellPath)
	}

	// Are we running with userns and subuid / subgid fakeroot functionality?
	l.engineConfig.SetFakeroot(l.cfg.Fakeroot)
	if l.cfg.Fakeroot {
		l.cfg.Namespaces.User = !l.cfg.IgnoreUserns
	}

	err = l.setCgroups(instanceName)
	if err != nil {
		sylog.Fatalf("Error while setting cgroups, err: %s", err)
	}

	// --boot flag requires privilege, so check for this.
	err = withPrivilege(l.uid, l.cfg.Boot, "--boot", func() error { return nil })
	if err != nil {
		sylog.Fatalf("Could not configure --boot: %s", err)
	}

	// --containall or --boot infer --contain.
	if l.cfg.Contain || l.cfg.ContainAll || l.cfg.Boot {
		l.engineConfig.SetContain(true)
		// --containall infers PID/IPC isolation and a clean environment.
		if l.cfg.ContainAll {
			l.cfg.Namespaces.PID = true
			l.cfg.Namespaces.IPC = true
			l.cfg.CleanEnv = true
		}
	}

	// Setup instance specific configuration if required.
	if instanceName != "" {
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "INSTANCE", instanceName)
		// instance are always using PID namespace by default
		pidNamespace := true
		if l.cfg.ShareNSMode && !l.cfg.Namespaces.PID {
			// sharens disable PID namespace by default
			pidNamespace = false
		}
		l.cfg.Namespaces.PID = pidNamespace
		l.engineConfig.SetInstance(true)
		l.engineConfig.SetBootInstance(l.cfg.Boot)

		if useSuid && !l.cfg.Namespaces.User && hidepidProc() {
			return fmt.Errorf("hidepid option set on /proc mount, require 'hidepid=0' to start instance with setuid workflow")
		}

		_, err := instance.Get(instanceName, instance.AppSubDir)
		if err == nil {
			return fmt.Errorf("instance %s already exists", instanceName)
		}

		if l.cfg.Boot {
			l.cfg.Namespaces.UTS = true
			l.cfg.Namespaces.Net = true
			if l.cfg.Hostname == "" {
				l.engineConfig.SetHostname(instanceName)
			}
			if !l.cfg.KeepPrivs {
				l.engineConfig.SetDropCaps("CAP_SYS_BOOT,CAP_SYS_RAWIO")
			}
			l.generator.SetProcessArgs([]string{"/sbin/init"})
		}

		// Set sharens mode
		l.engineConfig.SetShareNSMode(l.cfg.ShareNSMode)
		l.engineConfig.SetShareNSFd(l.cfg.ShareNSFd)
	}

	// Set runscript timeout
	l.engineConfig.SetRunscriptTimout(l.cfg.RunscriptTimeout)

	// Set the required namespaces in the engine config.
	l.setNamespaces()
	// Set the container environment.
	if err := l.setEnvVars(ctx, args); err != nil {
		return fmt.Errorf("while setting environment: %s", err)
	}
	// Set the container process work directory.
	l.setProcessCwd()

	l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "APPNAME", l.cfg.AppName)
	// set an additional environment APPTAINER_SHARENS_MASTER = 1 inside container
	if fd := l.engineConfig.GetShareNSFd(); fd != -1 && l.engineConfig.GetShareNSMode() {
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "SHARENS_MASTER", "1")
	}

	// Get image ready to run, if needed, via FUSE mount / extraction / image driver handling.
	if err := l.prepareImage(ctx, insideUserNs, image); err != nil {
		return fmt.Errorf("while preparing image: %s", err)
	}

	loadOverlay := false
	if !l.cfg.Namespaces.User && (buildcfg.APPTAINER_SUID_INSTALL == 1 || os.Getuid() == 0) {
		has, err := proc.HasFilesystem("overlay")
		if err != nil {
			return fmt.Errorf("while checking whether overlay filesystem is loaded: %w", err)
		}
		if !has {
			loadOverlay = true
		}
	}

	cfg := &config.Common{
		EngineName:   apptainerConfig.Name,
		ContainerID:  instanceName,
		EngineConfig: l.engineConfig,
	}

	// Allow any plugins with callbacks to modify the assembled Config
	runPluginCallbacks(cfg)

	// Call the starter binary using our prepared config.
	if l.engineConfig.GetInstance() && !l.cfg.ShareNSMode {
		err = l.starterInstance(loadOverlay, insideUserNs, instanceName, useSuid, cfg)
	} else {
		err = l.starterInteractive(loadOverlay, useSuid, cfg)
	}

	// Execution is finished.
	if err != nil {
		return fmt.Errorf("while executing starter: %s", err)
	}
	return nil
}

// setUmask saves the current umask, to be set for the process run in the container,
// unless the --no-umask option was specified.
// https://github.com/apptainer/singularity/issues/5214
func (l *Launcher) setUmask() {
	currMask := syscall.Umask(0o022)
	if !l.cfg.NoUmask {
		sylog.Debugf("Saving umask %04o for propagation into container", currMask)
		l.engineConfig.SetUmask(currMask)
		l.engineConfig.SetRestoreUmask(true)
	}
}

// setTargetIDs sets engine configuration for any requested target UID and GID
// when allowed
// The effective uid and gid we will run under are returned as uid and gid.
func (l *Launcher) setTargetIDs(useSuid bool) (err error) {
	// Identify requested uid/gif (if any) from --security options
	uidParam := security.GetParam(l.cfg.SecurityOpts, "uid")
	gidParam := security.GetParam(l.cfg.SecurityOpts, "gid")

	targetUID := 0
	targetGID := make([]int, 0)

	pseudoRoot := l.uid
	if !useSuid {
		// always allow when not using suid starter
		pseudoRoot = 0
	}

	// If a target uid was requested, and we are root or non-suid, handle that.
	err = withPrivilege(pseudoRoot, uidParam != "", "uid security feature with suid mode", func() error {
		u, err := strconv.ParseUint(uidParam, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to parse provided UID: %w", err)
		}
		targetUID = int(u)
		l.uid = uint32(targetUID)

		l.engineConfig.SetTargetUID(targetUID)
		return nil
	})
	if err != nil {
		return err
	}

	// If any target gids were requested, and we are root or non-suid, handle that.
	err = withPrivilege(pseudoRoot, gidParam != "", "gid security feature with suid mode", func() error {
		gids := strings.Split(gidParam, ":")
		for _, id := range gids {
			g, err := strconv.ParseUint(id, 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse provided GID: %w", err)
			}
			targetGID = append(targetGID, int(g))
		}
		if len(gids) > 0 {
			l.gid = uint32(targetGID[0])
		}

		l.engineConfig.SetTargetGID(targetGID)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// setImageOrInstance sets the image to start, or instance and it's image to be joined.
func (l *Launcher) setImageOrInstance(image string, name string) error {
	if strings.HasPrefix(image, "instance://") {
		if name != "" {
			return fmt.Errorf("starting an instance from another is not allowed")
		}
		instanceName := instance.ExtractName(image)
		file, err := instance.Get(instanceName, instance.AppSubDir)
		if err != nil {
			return err
		}
		l.cfg.Namespaces.User = file.UserNs
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", file.Image)
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NAME", filepath.Base(file.Image))
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "INSTANCE", instanceName)
		l.engineConfig.SetImage(image)
		l.engineConfig.SetInstanceJoin(true)

		// If we are running non-root, join the instance cgroup now, as we
		// can't manipulate the ppid cgroup in the engine prepareInstanceJoinConfig().
		// This flow is only applicable with the systemd cgroups manager.
		if file.Cgroup && l.uid != 0 {
			if !l.engineConfig.File.SystemdCgroups {
				return fmt.Errorf("joining non-root instance with cgroups requires systemd as cgroups manager")
			}

			pid := os.Getpid()

			// First, we create a new systemd managed cgroup for ourselves. This is so that we will be
			// under a common user-owned ancestor, allowing us to move into the instance cgroup next.
			// See: https://www.kernel.org/doc/html/v4.18/admin-guide/cgroup-v2.html#delegation-containment
			sylog.Debugf("Adding process %d to sibling cgroup", pid)
			manager, err := cgroups.NewManagerWithSpec(&specs.LinuxResources{}, pid, "", true)
			if err != nil {
				return fmt.Errorf("couldn't create cgroup manager: %w", err)
			}
			cgPath, _ := manager.GetCgroupRelPath()
			sylog.Debugf("In sibling cgroup: %s", cgPath)

			// Now we should be under the user-owned service directory in the cgroupfs,
			// so we can move into the actual instance cgroup that we want.
			sylog.Debugf("Moving process %d to instance cgroup", pid)
			manager, err = cgroups.GetManagerForPid(file.Pid)
			if err != nil {
				return fmt.Errorf("couldn't create cgroup manager: %w", err)
			}
			if err := manager.AddProc(pid); err != nil {
				return fmt.Errorf("couldn't add process to instance cgroup: %w", err)
			}
			cgPath, _ = manager.GetCgroupRelPath()
			sylog.Debugf("In instance cgroup: %s", cgPath)
		}
	} else {
		abspath, err := filepath.Abs(image)
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", abspath)
		l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NAME", filepath.Base(abspath))
		if err != nil {
			return fmt.Errorf("failed to determine image absolute path for %s: %w", image, err)
		}
		l.engineConfig.SetImage(abspath)
	}
	return nil
}

// checkEncryptionKey verifies key material is available if the image is encrypted.
// Allows us to fail fast if required key material is not available / usable.
func (l *Launcher) checkEncryptionKey() error {
	sylog.Debugf("Checking for encrypted system partition")
	img, err := imgutil.Init(l.engineConfig.GetImage(), false)
	if err != nil {
		return fmt.Errorf("could not open image %s: %w", l.engineConfig.GetImage(), err)
	}

	part, err := img.GetRootFsPartition()
	if err != nil {
		return fmt.Errorf("while getting root filesystem in %s: %w", l.engineConfig.GetImage(), err)
	}

	if part.Type == imgutil.ENCRYPTSQUASHFS || part.Type == imgutil.GOCRYPTFSSQUASHFS {
		sylog.Debugf("Encrypted container filesystem detected")

		if l.cfg.KeyInfo == nil {
			return fmt.Errorf("required option --passphrase or --pem-path missing")
		}

		plaintextKey, err := cryptkey.PlaintextKey(*l.cfg.KeyInfo, l.engineConfig.GetImage())
		if err != nil {
			sylog.Errorf("Please check you are providing the correct key for decryption")
			return fmt.Errorf("cannot decrypt %s: %w", l.engineConfig.GetImage(), err)
		}

		l.engineConfig.SetEncryptionKey(plaintextKey)
	}
	// don't defer this call as in all cases it won't be
	// called before execing starter, so it would leak the
	// image file descriptor to the container process
	img.File.Close()
	return nil
}

// useSuid checks whether to use the setuid starter binary, and if we need to force the user namespace.
func (l *Launcher) useSuid(insideUserNs bool) (useSuid bool) {
	// privileged installation by default
	useSuid = true
	if buildcfg.APPTAINER_SUID_INSTALL == 0 {
		// not a privileged installation
		useSuid = false

		if !l.cfg.Namespaces.User && l.uid != 0 {
			sylog.Verbosef("Unprivileged installation: using user namespace")
			l.cfg.Namespaces.User = true
		}
	}

	// use non privileged starter binary:
	// - if running as root
	// - if already running inside a user namespace
	// - if user namespace is requested
	// - if running as user and 'allow setuid = no' is set in apptainer.conf
	if l.uid == 0 || insideUserNs || l.cfg.Namespaces.User || !l.engineConfig.File.AllowSetuid {
		useSuid = false

		// fallback to user namespace:
		// - for non root user with setuid installation and 'allow setuid = no'
		// - for root user without effective capability CAP_SYS_ADMIN
		if l.uid != 0 && buildcfg.APPTAINER_SUID_INSTALL == 1 && !l.engineConfig.File.AllowSetuid {
			sylog.Verbosef("'allow setuid' set to 'no' by configuration, fallback to user namespace")
			l.cfg.Namespaces.User = true
		} else if l.uid == 0 && !l.cfg.Namespaces.User {
			caps, err := capabilities.GetProcessEffective()
			if err != nil {
				sylog.Fatalf("Could not get process effective capabilities: %s", err)
			}
			if caps&uint64(1<<unix.CAP_SYS_ADMIN) == 0 {
				sylog.Verbosef("Effective capability CAP_SYS_ADMIN is missing, fallback to user namespace")
				l.cfg.Namespaces.User = true
			}
		}
	}
	return useSuid
}

// setBinds sets engine configuration for requested bind mounts.
func (l *Launcher) setBinds(fakerootPath string) error {
	// First get binds from -B/--bind and env var
	binds, err := apptainerConfig.ParseBindPath(l.cfg.BindPaths)
	if err != nil {
		return fmt.Errorf("while parsing bind path: %w", err)
	}
	// Now add binds from one or more --mount and env var.
	// Note that these do not get exported for nested containers
	for _, m := range l.cfg.Mounts {
		bps, err := apptainerConfig.ParseMountString(m)
		if err != nil {
			return fmt.Errorf("while parsing mount %q: %w", m, err)
		}
		binds = append(binds, bps...)
	}

	if fakerootPath != "" {
		l.engineConfig.SetFakerootPath(fakerootPath)
		// Add binds for fakeroot command
		fakebindPaths, err := fakeroot.GetFakeBinds(fakerootPath)
		if err != nil {
			return fmt.Errorf("while getting fakeroot bindpoints: %w", err)
		}
		fakebinds, err := apptainerConfig.ParseBindPath(fakebindPaths)
		if err != nil {
			return fmt.Errorf("while parsing fakeroot bind paths: %w", err)
		}
		binds = append(binds, fakebinds...)
	}

	l.engineConfig.SetBindPath(binds)

	// Pass only the destinations to nested binds
	bindPaths := make([]string, len(binds))
	for i, bind := range binds {
		bindPaths[i] = bind.Destination
	}
	l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "BIND", strings.Join(bindPaths, ","))
	return nil
}

// setFuseMounts sets engine configuration for requested FUSE mounts.
func (l *Launcher) setFuseMounts() error {
	if len(l.cfg.FuseMount) > 0 {
		/* If --fusemount is given, imply --pid */
		l.cfg.Namespaces.PID = true
		if err := l.engineConfig.SetFuseMount(l.cfg.FuseMount); err != nil {
			return fmt.Errorf("while setting fuse mount: %w", err)
		}
	}
	return nil
}

// Set engine flags to disable mounts, to allow overriding them if they are set true
// in the apptainer.conf.
func (l *Launcher) setNoMountFlags() {
	skipBinds := []string{}
	for _, v := range l.cfg.NoMount {
		switch v {
		case "proc":
			l.engineConfig.SetNoProc(true)
		case "sys":
			l.engineConfig.SetNoSys(true)
		case "dev":
			l.engineConfig.SetNoDev(true)
		case "devpts":
			l.engineConfig.SetNoDevPts(true)
		case "home":
			l.engineConfig.SetNoHome(true)
		case "tmp":
			l.engineConfig.SetNoTmp(true)
		case "hostfs":
			l.engineConfig.SetNoHostfs(true)
		case "cwd":
			l.engineConfig.SetNoCwd(true)
		// All bind path apptainer.conf entries
		case "bind-paths":
			skipBinds = append(skipBinds, "*")
		default:
			// Single bind path apptainer.conf entry by abs path
			if filepath.IsAbs(v) {
				skipBinds = append(skipBinds, v)
				continue
			}
			sylog.Warningf("Ignoring unknown mount type '%s'", v)
		}
	}
	l.engineConfig.SetSkipBinds(skipBinds)
}

// setHome sets the correct home directory configuration for our circumstance.
// If it is not possible to mount a home directory then the mount will be disabled.
func (l *Launcher) setHome() error {
	l.engineConfig.SetCustomHome(l.cfg.CustomHome)
	// If we have fakeroot & the home flag has not been used then we have the standard
	// /root location for the root user $HOME in the container.
	// This doesn't count as a SetCustomHome(true), as we are mounting from the real
	// user's standard $HOME -> /root and we want to respect --contain not mounting
	// the $HOME in this case.
	// See https://github.com/apptainer/singularity/pull/5227
	// Note from dwd on 3/24/22: it's not clear to me that this has
	// any effect because getHomePaths() appears to ignore the
	// HomeDir settings if there is no CustomHome
	if !l.cfg.CustomHome && l.cfg.Fakeroot {
		l.cfg.HomeDir = fmt.Sprintf("%s:/root", l.cfg.HomeDir)
	}
	// If we are running apptainer as root, but requesting a target UID in the container,
	// handle set the home directory appropriately.
	targetUID := l.engineConfig.GetTargetUID()
	if l.cfg.CustomHome && targetUID != 0 {
		if targetUID > 500 {
			if pu, err := user.GetPwUID(uint32(targetUID)); err == nil {
				sylog.Debugf("Target UID requested, set home directory to %s", pu.Dir)
				l.cfg.HomeDir = pu.Dir
				l.engineConfig.SetCustomHome(true)
			} else {
				sylog.Verbosef("Home directory for UID %d not found, home won't be mounted", targetUID)
				l.engineConfig.SetNoHome(true)
				l.cfg.HomeDir = "/"
			}
		} else {
			sylog.Verbosef("System UID %d requested, home won't be mounted", targetUID)
			l.engineConfig.SetNoHome(true)
			l.cfg.HomeDir = "/"
		}
	}

	// Handle any user request to override the home directory source/dest
	homeSlice := strings.Split(l.cfg.HomeDir, ":")
	if len(homeSlice) > 2 || len(homeSlice) == 0 {
		return fmt.Errorf("home argument has incorrect number of elements: %v", len(homeSlice))
	}
	l.engineConfig.SetHomeSource(homeSlice[0])
	if len(homeSlice) == 1 {
		l.engineConfig.SetHomeDest(homeSlice[0])
	} else {
		l.engineConfig.SetHomeDest(homeSlice[1])
	}
	return nil
}

// SetGPUConfig sets up EngineConfig entries for NV / ROCm usage, if requested.
func (l *Launcher) SetGPUConfig() error {
	if l.engineConfig.File.AlwaysUseNv && !l.cfg.NoNvidia {
		l.cfg.Nvidia = true
		sylog.Verbosef("'always use nv = yes' found in apptainer.conf")
	}
	if l.engineConfig.File.AlwaysUseRocm && !l.cfg.NoRocm {
		l.cfg.Rocm = true
		sylog.Verbosef("'always use rocm = yes' found in apptainer.conf")
	}

	if l.cfg.NvCCLI && !l.cfg.Nvidia {
		sylog.Debugf("implying --nv from --nvccli")
		l.cfg.Nvidia = true
	}

	if l.cfg.Rocm {
		err := l.setRocmConfig()
		// This is currently unnecessary, but useful for not missing future errors
		if err != nil {
			return err
		}
	}

	if l.cfg.Nvidia {
		// If nvccli was not enabled by flag or config, drop down to legacy binds immediately
		if !l.engineConfig.File.UseNvCCLI && !l.cfg.NvCCLI {
			return l.setNVLegacyConfig()
		}

		// TODO: In privileged fakeroot mode we don't have the correct namespace context to run nvidia-container-cli
		// from  starter, so fall back to legacy NV handling until that workflow is refactored heavily.
		fakeRootPriv := l.cfg.Fakeroot && l.engineConfig.File.AllowSetuid && buildcfg.APPTAINER_SUID_INSTALL == 1
		if !fakeRootPriv {
			return l.setNvCCLIConfig()
		}
		return fmt.Errorf("--fakeroot does not support --nvccli in set-uid installations")
	}
	return nil
}

// setNvCCLIConfig sets up EngineConfig entries for NVIDIA GPU configuration via nvidia-container-cli.
func (l *Launcher) setNvCCLIConfig() (err error) {
	sylog.Debugf("Using nvidia-container-cli for GPU setup")
	l.engineConfig.SetNvCCLI(true)

	if os.Getenv("NVIDIA_VISIBLE_DEVICES") == "" {
		if l.cfg.Contain || l.cfg.ContainAll {
			// When we use --contain we don't mount the NV devices by default in the nvidia-container-cli flow,
			// they must be mounted via specifying with`NVIDIA_VISIBLE_DEVICES`. This differs from the legacy
			// flow which mounts all GPU devices, always... so warn the user.
			sylog.Warningf("When using nvidia-container-cli with --contain NVIDIA_VISIBLE_DEVICES must be set or no GPUs will be available in container.")
		} else {
			// In non-contained mode set NVIDIA_VISIBLE_DEVICES="all" by default, so MIGs are available.
			// Otherwise there is a difference vs legacy GPU binding. See Issue sylabs/singularity#471.
			sylog.Infof("Setting 'NVIDIA_VISIBLE_DEVICES=all' to emulate legacy GPU binding.")
			os.Setenv("NVIDIA_VISIBLE_DEVICES", "all")
		}
	}

	// Pass NVIDIA_ env vars that will be converted to nvidia-container-cli options
	nvCCLIEnv := []string{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "NVIDIA_") {
			nvCCLIEnv = append(nvCCLIEnv, e)
		}
	}
	l.engineConfig.SetNvCCLIEnv(nvCCLIEnv)

	if !l.cfg.Writable && !l.cfg.WritableTmpfs {
		sylog.Infof("Setting --writable-tmpfs (required by nvidia-container-cli)")
		l.cfg.WritableTmpfs = true
	}

	return nil
}

// setNvLegacyConfig sets up EngineConfig entries for NVIDIA GPU configuration via direct binds of configured bins/libs.
func (l *Launcher) setNVLegacyConfig() error {
	sylog.Debugf("Using legacy binds for nv GPU setup")
	l.engineConfig.SetNvLegacy(true)
	gpuConfFile := filepath.Join(buildcfg.APPTAINER_CONFDIR, "nvliblist.conf")
	// bind persistenced socket if found
	ipcs, err := gpu.NvidiaIpcsPath()
	if err != nil {
		sylog.Warningf("While finding nv ipcs: %v", err)
	}
	libs, bins, err := gpu.NvidiaPaths(gpuConfFile)
	if err != nil {
		sylog.Warningf("While finding nv bind points: %v", err)
	}
	l.addGPUBinds(libs, bins, ipcs, "nv")
	return nil
}

// setRocmConfig sets up EngineConfig entries for ROCm GPU configuration via direct binds of configured bins/libs.
func (l *Launcher) setRocmConfig() error {
	sylog.Debugf("Using rocm GPU setup")
	l.engineConfig.SetRocm(true)
	gpuConfFile := filepath.Join(buildcfg.APPTAINER_CONFDIR, "rocmliblist.conf")
	libs, bins, err := gpu.RocmPaths(gpuConfFile)
	if err != nil {
		sylog.Warningf("While finding ROCm bind points: %v", err)
	}
	l.addGPUBinds(libs, bins, []string{}, "rocm")
	return nil
}

// addGPUBinds adds EngineConfig entries to bind the provided list of libs, bins, ipc files.
func (l *Launcher) addGPUBinds(libs, bins, ipcs []string, gpuPlatform string) {
	files := make([]string, len(bins)+len(ipcs))
	if len(files) == 0 {
		sylog.Warningf("Could not find any %s files on this host!", gpuPlatform)
	} else {
		if l.cfg.Writable {
			sylog.Warningf("%s files may not be bound with --writable", gpuPlatform)
		}
		for i, binary := range bins {
			usrBinBinary := filepath.Join("/usr/bin", filepath.Base(binary))
			files[i] = strings.Join([]string{binary, usrBinBinary}, ":")
		}
		for i, ipc := range ipcs {
			files[i+len(bins)] = ipc
		}
		l.engineConfig.AppendFilesPath(files...)
	}
	if len(libs) == 0 {
		sylog.Warningf("Could not find any %s libraries on this host!", gpuPlatform)
	} else {
		l.engineConfig.AppendLibrariesPath(libs...)
	}
}

// setNamespaces sets namespace configuration for the engine.
func (l *Launcher) setNamespaces() {
	if !l.cfg.Namespaces.Net && l.cfg.Network != "" {
		sylog.Infof("Setting --net (required by --network)")
		l.cfg.Namespaces.Net = true
	}
	if !l.cfg.Namespaces.Net && len(l.cfg.NetworkArgs) != 0 {
		sylog.Infof("Setting --net (required by --network-args)")
		l.cfg.Namespaces.Net = true
	}
	if l.cfg.Namespaces.Net {
		if l.cfg.Network == "" {
			l.cfg.Network = "bridge"
			l.engineConfig.SetNetwork(l.cfg.Network)
		}
		if l.cfg.Fakeroot && l.cfg.Network != "none" {
			// unprivileged installation could not use fakeroot
			// network because it requires a setuid installation
			// so we fallback to none
			if buildcfg.APPTAINER_SUID_INSTALL == 0 || !l.engineConfig.File.AllowSetuid {
				sylog.Warningf(
					"fakeroot with unprivileged installation or 'allow setuid = no' " +
						"could not use 'fakeroot' network, fallback to 'none' network",
				)
				l.engineConfig.SetNetwork("none")
			}
		}
		l.generator.AddOrReplaceLinuxNamespace("network", "")
	}
	if l.cfg.Namespaces.UTS {
		l.generator.AddOrReplaceLinuxNamespace("uts", "")
	}
	if l.cfg.Namespaces.PID {
		l.generator.AddOrReplaceLinuxNamespace("pid", "")
		l.engineConfig.SetNoInit(l.cfg.NoInit)
	}
	if l.cfg.Namespaces.IPC {
		l.generator.AddOrReplaceLinuxNamespace("ipc", "")
	}
	if l.cfg.Namespaces.User {
		l.generator.AddOrReplaceLinuxNamespace("user", "")
		if !l.cfg.Fakeroot {
			l.generator.AddLinuxUIDMapping(uint32(os.Getuid()), l.uid, 1)
			l.generator.AddLinuxGIDMapping(uint32(os.Getgid()), l.gid, 1)
		}
	}
}

// setEnvVars sets the environment for the container, from the host environment, glads, env-file.
func (l *Launcher) setEnvVars(ctx context.Context, args []string) error {
	if l.cfg.EnvFile != "" {
		currentEnv := append(
			os.Environ(),
			"APPTAINER_IMAGE="+l.engineConfig.GetImage(),
		)

		content, err := os.ReadFile(l.cfg.EnvFile)
		if err != nil {
			return fmt.Errorf("could not read %q environment file: %w", l.cfg.EnvFile, err)
		}

		shellEnv, err := interpreter.EvaluateEnv(ctx, content, args, currentEnv)
		if err != nil {
			return fmt.Errorf("while processing %s: %w", l.cfg.EnvFile, err)
		}
		// --env variables will take precedence over variables
		// defined by the environment file
		sylog.Debugf("Setting environment variables from file %s", l.cfg.EnvFile)

		// Update Env with those from file
		for _, envar := range shellEnv {
			e := strings.SplitN(envar, "=", 2)
			if len(e) != 2 {
				sylog.Warningf("Ignore environment variable %q: '=' is missing", envar)
				continue
			}
			// Don't attempt to overwrite bash builtin readonly vars
			// https://github.com/sylabs/singularity/issues/1263
			if _, ok := env.ReadOnlyVars[e[0]]; ok {
				continue
			}
			// Ensure we don't overwrite --env variables with environment file
			if _, ok := l.cfg.Env[e[0]]; ok {
				sylog.Warningf("Ignore environment variable %s from %s: override from --env", e[0], l.cfg.EnvFile)
			} else {
				l.cfg.Env[e[0]] = e[1]
			}
		}
	}
	// process --env and --env-file variables for injection
	// into the environment by prefixing them with APPTAINERENV_
	for envName, envValue := range l.cfg.Env {
		// We can allow envValue to be empty (explicit set to empty) but not name!
		if envName == "" {
			sylog.Warningf("Ignore environment variable %s=%s: variable name missing", envName, envValue)
			continue
		}
		os.Setenv("APPTAINERENV_"+envName, envValue)
	}
	// Copy and cache environment
	environment := os.Environ()
	// Clean environment
	apptainerEnv := env.SetContainerEnv(l.generator, environment, l.cfg.CleanEnv, l.engineConfig.GetHomeDest())
	l.engineConfig.SetApptainerEnv(apptainerEnv)
	return nil
}

// setProcessCwd sets the container process working directory
func (l *Launcher) setProcessCwd() {
	if cwd, err := os.Getwd(); err == nil {
		l.engineConfig.SetCwd(cwd)
		if l.cfg.CwdPath != "" {
			l.generator.SetProcessCwd(l.cfg.CwdPath)
			if l.generator.Config.Annotations == nil {
				l.generator.Config.Annotations = make(map[string]string)
			}
			l.generator.Config.Annotations["CustomCwd"] = "true"
		} else {
			if l.engineConfig.GetContain() {
				l.generator.SetProcessCwd(l.engineConfig.GetHomeDest())
			} else {
				l.generator.SetProcessCwd(cwd)
			}
		}
	} else {
		sylog.Warningf("can't determine current working directory: %s", err)
	}
}

// setCgroups sets cgroup related configuration
func (l *Launcher) setCgroups(instanceName string) error {
	// If we are not root, we need to pass in XDG / DBUS environment so we can communicate
	// with systemd for any cgroups (v2) operations.
	if l.uid != 0 {
		sylog.Debugf("Recording rootless XDG_RUNTIME_DIR / DBUS_SESSION_BUS_ADDRESS")
		l.engineConfig.SetXdgRuntimeDir(os.Getenv("XDG_RUNTIME_DIR"))
		l.engineConfig.SetDbusSessionBusAddress(os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
	}

	if l.cfg.CGroupsJSON != "" {
		// Handle cgroups configuration (parsed from file or flags in CLI).
		l.engineConfig.SetCgroupsJSON(l.cfg.CGroupsJSON)
		return nil
	}

	if instanceName == "" {
		return nil
	}

	hidePid := hidepidProc()
	// If we are an instance, always use a cgroup if possible, to enable stats.
	// root can always create a cgroup.
	sylog.Debugf("During setting cgroups configuration, uid: %d, namespace.user: %t, fakeroot: %t, unprivileged: %t", l.uid, l.cfg.Namespaces.User, l.cfg.Fakeroot, namespaces.IsUnprivileged())
	useCG := l.uid == 0 && !namespaces.IsUnprivileged()
	// non-root needs cgroups v2 unified mode + systemd as cgroups manager.
	if !useCG && lccgroups.IsCgroup2UnifiedMode() && l.engineConfig.File.SystemdCgroups && !namespaces.IsUnprivileged() && !l.cfg.Fakeroot && !hidePid {
		if os.Getenv("XDG_RUNTIME_DIR") == "" || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
			sylog.Infof("Instance stats will not be available because XDG_RUNTIME_DIR")
			sylog.Infof("  or DBUS_SESSION_BUS_ADDRESS is not set")
			return nil
		}
		useCG = true
	}

	if useCG {
		sylog.Debugf("Using cgroup manager during setting cgroups configuration")
		cg := cgroups.Config{}
		cgJSON, err := cg.MarshalJSON()
		if err != nil {
			return err
		}
		l.engineConfig.SetCgroupsJSON(cgJSON)
		return nil
	}

	if l.cfg.Fakeroot {
		sylog.Debugf("Instance stats will not be available because of fakeroot mode")
		return nil
	}

	if hidePid {
		sylog.Debugf("Instance stats will not be available because of hidepid option is set on /proc mount")
		return nil
	}

	if l.cfg.ShareNSMode {
		sylog.Debugf("Instance stats will not be available - requires cgroups v2 with systemd as manager.")
	} else {
		sylog.Infof("Instance stats will not be available - requires cgroups v2 with systemd as manager.")
	}
	return nil
}

// PrepareImage performs any image preparation required before execution.
// This is currently limited to extraction or FUSE mount when using the user namespace,
// and activating any image driver plugins that might handle the image mount.
func (l *Launcher) prepareImage(c context.Context, insideUserNs bool, image string) error {
	// initialize internal image drivers
	var desiredFeatures imgutil.DriverFeature
	if fs.IsFile(image) {
		desiredFeatures = imgutil.ImageFeature
	}
	fileconf := l.engineConfig.File
	driver.InitImageDrivers(true, l.cfg.Namespaces.User || insideUserNs, fileconf, desiredFeatures)

	// convert image file to sandbox if either it was requested by
	// `--unsquash` or we cannot mount the image directly and there's
	// no image driver.
	if fs.IsFile(image) {
		convert := false
		if l.cfg.Unsquash {
			convert = true
		} else if l.cfg.Namespaces.User || insideUserNs ||
			!squashfs.SetuidMountAllowed(fileconf) {
			convert = true
			if fileconf.ImageDriver != "" {
				// load image driver plugins
				callbackType := (apptainercallback.RegisterImageDriver)(nil)
				callbacks, err := plugin.LoadCallbacks(callbackType)
				if err != nil {
					sylog.Debugf("Loading plugins callbacks '%T' failed: %s", callbackType, err)
				} else {
					for _, callback := range callbacks {
						if err := callback.(apptainercallback.RegisterImageDriver)(true); err != nil {
							sylog.Debugf("While registering image driver: %s", err)
						}
					}
				}
				driver := imgutil.GetDriver(fileconf.ImageDriver)
				if driver != nil && driver.Features()&imgutil.SquashFeature != 0 {
					// the image driver indicates support for squashfs so let's
					// proceed with the image driver without conversion
					convert = false
				}
			}
		}

		if convert {
			unsquashfsPath, err := bin.FindBin("unsquashfs")
			if err != nil {
				sylog.Fatalf("while extracting %s: %s", image, err)
			}
			sylog.Infof("Converting SIF file to temporary sandbox...")
			rootfsDir, imageDir, err := convertImage(image, unsquashfsPath, l.cfg.TmpDir)
			if err != nil {
				sylog.Fatalf("while extracting %s: %s", image, err)
			}
			l.engineConfig.SetImage(imageDir)
			l.engineConfig.SetDeleteTempDir(rootfsDir)
			l.generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", imageDir)
			// if '--disable-cache' flag, then remove original SIF after converting to sandbox
			if l.cfg.CacheDisabled {
				sylog.Debugf("Removing tmp image: %s", image)
				err := os.Remove(image)
				if err != nil {
					return fmt.Errorf("unable to remove tmp image: %s: %w", image, err)
				}
			}
		}
	}
	return nil
}

// starterInteractive executes the starter binary to run an image interactively, given the supplied engineConfig
func (l *Launcher) starterInteractive(loadOverlay bool, useSuid bool, cfg *config.Common) error {
	err := starter.Exec(
		"Apptainer runtime parent",
		cfg,
		starter.UseSuid(useSuid),
		starter.LoadOverlayModule(loadOverlay),
	)
	return err
}

// starterInstance executes the starter binary to run an instance given the supplied engineConfig
func (l *Launcher) starterInstance(loadOverlay bool, insideUserNs bool, name string, useSuid bool, cfg *config.Common) error {
	pu, err := user.GetPwUID(l.uid)
	if err != nil {
		return fmt.Errorf("failed to retrieve user information for UID %d: %w", l.uid, err)
	}
	procname, err := instance.ProcName(name, pu.Name)
	if err != nil {
		return err
	}

	var start int64

	stdout, stderr, err := instance.SetLogFile(name, l.cfg.Namespaces.User || insideUserNs, int(l.uid), instance.LogSubDir)
	if err != nil {
		return fmt.Errorf("failed to create instance log files: %w", err)
	}
	start, err = stderr.Seek(0, io.SeekEnd)
	if err != nil {
		sylog.Warningf("failed to get standard error stream offset: %s", err)
	}

	cmdErr := starter.Run(
		procname,
		cfg,
		starter.UseSuid(useSuid),
		starter.WithStdout(stdout),
		starter.WithStderr(stderr),
		starter.LoadOverlayModule(loadOverlay),
	)

	if sylog.GetLevel() != 0 {
		// starter can exit a bit before all errors has been reported
		// by instance process, wait a bit to catch all errors
		time.Sleep(100 * time.Millisecond)

		end, err := stderr.Seek(0, io.SeekEnd)
		if err != nil {
			sylog.Warningf("failed to get standard error stream offset: %s", err)
		}
		if end-start > 0 {
			output := make([]byte, end-start)
			stderr.ReadAt(output, start)
			fmt.Println(string(output))
		}
	}

	if cmdErr != nil {
		return fmt.Errorf("failed to start instance: %w", cmdErr)
	}
	sylog.Verbosef("you will find instance output here: %s", stdout.Name())
	sylog.Verbosef("you will find instance error here: %s", stderr.Name())
	sylog.Infof("instance started successfully")

	return nil
}

// runPluginCallbacks executes any plugin callbacks to manipulate the engine config passed in
func runPluginCallbacks(cfg *config.Common) error {
	callbackType := (clicallback.ApptainerEngineConfig)(nil)
	callbacks, err := plugin.LoadCallbacks(callbackType)
	if err != nil {
		return fmt.Errorf("while loading plugin callbacks '%T': %w", callbackType, err)
	}
	for _, c := range callbacks {
		c.(clicallback.ApptainerEngineConfig)(cfg)
	}
	return nil
}

// withPrivilege calls fn if cond is satisfied, and we are uid 0
func withPrivilege(uid uint32, cond bool, desc string, fn func() error) error {
	if !cond {
		return nil
	}
	if uid != 0 {
		return fmt.Errorf("%s requires root privileges", desc)
	}
	return fn()
}

// hidepidProc checks if hidepid is set on /proc mount point, when this
// option is an instance started with setuid workflow could not even be
// joined later or stopped correctly.
func hidepidProc() bool {
	entries, err := proc.GetMountInfoEntry("/proc/self/mountinfo")
	if err != nil {
		sylog.Warningf("while reading /proc/self/mountinfo: %s", err)
		return false
	}
	for _, e := range entries {
		if e.Point == "/proc" {
			for _, o := range e.SuperOptions {
				if strings.HasPrefix(o, "hidepid=") {
					return true
				}
			}
		}
	}
	return false
}

// convertImage extracts the image found at filename to directory dir within a temporary directory
// tempDir. If the unsquashfs binary is not located, the binary at unsquashfsPath is used. It is
// the caller's responsibility to remove rootfsDir when no longer needed.
func convertImage(filename string, unsquashfsPath string, tmpDir string) (rootfsDir string, imageDir string, err error) {
	img, err := imgutil.Init(filename, false)
	if err != nil {
		return "", "", fmt.Errorf("could not open image %s: %s", filename, err)
	}
	defer img.File.Close()

	part, err := img.GetRootFsPartition()
	if err != nil {
		return "", "", fmt.Errorf("while getting root filesystem in %s: %s", filename, err)
	}

	// Nice message if we have been given an older ext3 image, which cannot be extracted due to lack of privilege
	// to loopback mount.
	if part.Type == imgutil.EXT3 {
		sylog.Errorf("File %q is an ext3 format container image.", filename)
		sylog.Errorf("Only SIF and squashfs images can be extracted in unprivileged mode.")
		sylog.Errorf("Use `apptainer build` to convert this image to a SIF file using a setuid install of Apptainer.")
	}

	// Only squashfs can be extracted
	if part.Type != imgutil.SQUASHFS {
		return "", "", fmt.Errorf("not a squashfs root filesystem")
	}

	// create a reader for rootfs partition
	reader, err := imgutil.NewPartitionReader(img, "", 0)
	if err != nil {
		return "", "", fmt.Errorf("could not extract root filesystem: %s", err)
	}
	s := unpacker.NewSquashfs()
	if !s.HasUnsquashfs() && unsquashfsPath != "" {
		s.UnsquashfsPath = unsquashfsPath
	}

	// create temporary sandbox
	rootfsDir, err = os.MkdirTemp(tmpDir, "rootfs-")
	if err != nil {
		return "", "", fmt.Errorf("could not create temporary sandbox: %s", err)
	}
	// NOTE: can't depend on the rootfsDir variable inside this function
	// because it is a named return variable and so it gets overridden
	// by the return statements that set that value to the empty string.
	// So pass it as a parameter here instead.
	defer func(rootDir string) {
		if err != nil {
			sylog.Verbosef("Cleaning up %v", rootDir)
			err2 := types.FixPerms(rootDir)
			if err2 != nil {
				sylog.Debugf("FixPerms had a problem: %v", err2)
			}
			err2 = os.RemoveAll(rootDir)
			if err2 != nil {
				sylog.Debugf("RemoveAll had a problem: %v", err2)
			}
		}
	}(rootfsDir)

	// create an inner dir to extract to, so we don't clobber the secure permissions on the tmpDir.
	imageDir = filepath.Join(rootfsDir, "root")
	if err := os.Mkdir(imageDir, 0o755); err != nil {
		return "", "", fmt.Errorf("could not create root directory: %s", err)
	}

	// extract root filesystem
	if err := s.ExtractAll(reader, imageDir); err != nil {
		return "", "", fmt.Errorf("root filesystem extraction failed: %s", err)
	}

	return rootfsDir, imageDir, err
}

// SetCheckpointConfig sets EngineConfig entries to bind the provided list of libs and bins.
func (l *Launcher) SetCheckpointConfig() error {
	if l.cfg.DMTCPLaunch == "" && l.cfg.DMTCPRestart == "" {
		return nil
	}

	return l.injectDMTCPConfig()
}

func (l *Launcher) injectDMTCPConfig() error {
	sylog.Debugf("Injecting DMTCP configuration")
	dmtcp.QuickInstallationCheck()

	bins, libs, err := dmtcp.GetPaths()
	if err != nil {
		return err
	}

	var config apptainerConfig.DMTCPConfig
	if l.cfg.DMTCPRestart != "" {
		config = apptainerConfig.DMTCPConfig{
			Enabled:    true,
			Restart:    true,
			Checkpoint: l.cfg.DMTCPRestart,
			Args:       dmtcp.RestartArgs(),
		}
	} else {
		config = apptainerConfig.DMTCPConfig{
			Enabled:    true,
			Restart:    false,
			Checkpoint: l.cfg.DMTCPLaunch,
			Args:       dmtcp.LaunchArgs(),
		}
	}

	m := dmtcp.NewManager()
	e, err := m.Get(config.Checkpoint)
	if err != nil {
		return err
	}

	sylog.Debugf("Injecting checkpoint state bind: %q", config.Checkpoint)
	l.engineConfig.SetBindPath(append(l.engineConfig.GetBindPath(), e.BindPath()))
	l.engineConfig.AppendFilesPath(bins...)
	l.engineConfig.AppendLibrariesPath(libs...)
	l.engineConfig.SetDMTCPConfig(config)

	return nil
}
