// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
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
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/cryptkey"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
	"github.com/apptainer/apptainer/pkg/util/rlimit"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// execStarter prepares an EngineConfig defining how a container should be executed, then calls the starter binary to execute it.
// This includes interactive containers, instances, and joining an existing instance.
//
//nolint:maintidx
func execStarter(cobraCmd *cobra.Command, image string, args []string, instanceName string) {
	var err error

	uid := uint32(os.Getuid())
	gid := uint32(os.Getgid())

	var fakerootPath string
	if IsFakeroot {
		if (uid == 0) && namespaces.IsUnprivileged() {
			// Already running root-mapped unprivileged
			IsFakeroot = false
			sylog.Debugf("running root-mapped unprivileged")
			var err error
			if IgnoreFakerootCmd {
				err = errors.New("fakeroot command is ignored because of --ignore-fakeroot-command")
			} else {
				fakerootPath, err = fakeroot.FindFake()
			}
			if err != nil {
				sylog.Infof("fakeroot command not found, using only root-mapped namespace")
			} else {
				sylog.Infof("Using fakeroot command combined with root-mapped namespace")
			}
		} else if (uid != 0) && (!fakeroot.IsUIDMapped(uid) || IgnoreSubuid) {
			sylog.Infof("User not listed in %v, trying root-mapped namespace", fakeroot.SubUIDFile)
			IsFakeroot = false
			var err error
			if IgnoreUserns {
				err = errors.New("could not start root-mapped namespace because of --ignore-userns is set")
			} else {
				err = fakeroot.UnshareRootMapped(os.Args)
			}
			if err == nil {
				// All good
				os.Exit(0)
			}
			sylog.Debugf("UnshareRootMapped failed: %v", err)
			if IgnoreFakerootCmd {
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

	// Initialize a new configuration for the engine.
	engineConfig := apptainerConfig.NewConfig()
	imageArg := os.Getenv("IMAGE_ARG")
	os.Unsetenv("IMAGE_ARG")
	engineConfig.SetImageArg(imageArg)
	engineConfig.File = apptainerconf.GetCurrentConfig()
	if engineConfig.File == nil {
		sylog.Fatalf("Unable to get apptainer configuration")
	}
	ociConfig := &oci.Config{}
	generator := generate.New(&ociConfig.Spec)
	engineConfig.OciConfig = ociConfig

	// Set arguments to pass to contained process.
	generator.SetProcessArgs(args)

	// NoEval means we will not shell evaluate args / env in action scripts and environment processing.
	// This replicates OCI behavior and differs from historic Apptainer behavior.
	if NoEval {
		engineConfig.SetNoEval(true)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NO_EVAL", "1")
	}

	// Set container Umask w.r.t. our own, before any umask manipulation happens.
	setUmask(engineConfig)

	// Get our effective uid and gid for container execution.
	// If root user requests a target uid, gid via --security options, handle them now.
	uid, gid, err = setTargetIDs(uid, gid, engineConfig)
	if err != nil {
		sylog.Fatalf("Could not configure target UID/GID: %s", err)
	}

	// Set image to run, or instance to join, and APPTAINER_CONTAINER/APPTAINER_NAME env vars.
	if err := setImageOrInstance(image, instanceName, uid, engineConfig, generator); err != nil {
		sylog.Fatalf("While setting image/instance: %s", err)
	}

	// Overlay or writable image requested?
	engineConfig.SetOverlayImage(OverlayPath)
	engineConfig.SetWritableImage(IsWritable)
	// --writable-tmpfs is for an ephemeral overlay, doesn't make sense if also asking to write to image itself.
	if IsWritable && IsWritableTmpfs {
		sylog.Warningf("Disabling --writable-tmpfs flag, mutually exclusive with --writable")
		engineConfig.SetWritableTmpfs(false)
	} else {
		engineConfig.SetWritableTmpfs(IsWritableTmpfs)
	}

	// Check key is available for encrypted image, if applicable.
	err = checkEncryptionKey(cobraCmd, engineConfig)
	if err != nil {
		sylog.Fatalf("While checking container encryption: %s", err)
	}

	insideUserNs, _ := namespaces.IsInsideUserNamespace(os.Getpid())

	// Will we use the suid starter? If not we need to force the user namespace.
	useSuid := useSuid(insideUserNs, uid, engineConfig)
	// IgnoreUserns is a hidden control flag
	UserNamespace = UserNamespace && !IgnoreUserns

	// In the setuid workflow, set RLIMIT_STACK to its default value, keeping the
	// original value to restore it before executing the container process.
	if useSuid {
		soft, hard, err := rlimit.Get("RLIMIT_STACK")
		if err != nil {
			sylog.Warningf("can't retrieve stack size limit: %s", err)
		}
		generator.AddProcessRlimits("RLIMIT_STACK", hard, soft)
	}

	// Handle requested binds, fuse mounts.
	if err := setBinds(fakerootPath, engineConfig, generator); err != nil {
		sylog.Fatalf("While setting bind mount configuration: %s", err)
	}
	if err := setFuseMounts(engineConfig); err != nil {
		sylog.Fatalf("While setting FUSE mount configuration: %s", err)
	}

	// Set the home directory that should be effective in the container.
	customHome := cobraCmd.Flag("home").Changed
	if err := setHome(customHome, engineConfig); err != nil {
		sylog.Fatalf("While setting home directory: %s", err)
	}
	// Allow user to disable the home mount via --no-home.
	engineConfig.SetNoHome(NoHome)
	// Allow user to disable binds via --no-mount.
	setNoMountFlags(engineConfig)

	// GPU configuration may add library bind to /.singularity.d/libs.
	if err := SetGPUConfig(engineConfig); err != nil {
		sylog.Fatalf("While setting GPU configuration: %s", err)
	}

	if err := SetCheckpointConfig(engineConfig); err != nil {
		sylog.Fatalf("while setting checkpoint configuration: %s", err)
	}

	// Additional user requested library binds into /.singularity.d/libs.
	engineConfig.AppendLibrariesPath(ContainLibsPath...)

	// Additional directory overrides.
	engineConfig.SetScratchDir(ScratchPath)
	engineConfig.SetWorkdir(WorkdirPath)

	// Container networking configuration.
	engineConfig.SetNetwork(Network)
	engineConfig.SetDNS(DNS)
	engineConfig.SetNetworkArgs(NetworkArgs)

	// If user wants to set a hostname, it requires the UTS namespace.
	if Hostname != "" {
		UtsNamespace = true
		engineConfig.SetHostname(Hostname)
	}

	// Set requested capabilities (effective for root, or if sysadmin has permitted to another user).
	engineConfig.SetAddCaps(AddCaps)
	engineConfig.SetDropCaps(DropCaps)

	// Custom --config file (only effective in non-setuid or as root).
	engineConfig.SetConfigurationFile(configurationFile)

	engineConfig.SetUseBuildConfig(useBuildConfig)

	// When running as root, the user can optionally allow setuid with container.
	err = withPrivilege(uid, AllowSUID, "--allow-setuid", func() error {
		engineConfig.SetAllowSUID(AllowSUID)
		return nil
	})
	if err != nil {
		sylog.Fatalf("Could not configure --allow-setuid: %s", err)
	}

	// When running as root, the user can optionally keep all privs in the container.
	err = withPrivilege(uid, KeepPrivs, "--keep-privs", func() error {
		engineConfig.SetKeepPrivs(KeepPrivs)
		return nil
	})
	if err != nil {
		sylog.Fatalf("Could not configure --keep-privs: %s", err)
	}

	// User can optionally force dropping all privs from root in the container.
	engineConfig.SetNoPrivs(NoPrivs)

	// Set engine --security options (selinux, apparmor, seccomp functionality).
	engineConfig.SetSecurity(Security)

	// User can override shell used when entering container.
	engineConfig.SetShell(ShellPath)
	if ShellPath != "" {
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "SHELL", ShellPath)
	}

	// Are we running with userns and subuid / subgid fakeroot functionality?
	engineConfig.SetFakeroot(IsFakeroot)
	if IsFakeroot {
		UserNamespace = !IgnoreUserns
	}

	// If we are not root, we need to pass in XDG / DBUS environment so we can communicate
	// with systemd for any cgroups (v2) operations.
	if uid != 0 {
		sylog.Debugf("Recording rootless XDG_RUNTIME_DIR / DBUS_SESSION_BUS_ADDRESS")
		engineConfig.SetXdgRuntimeDir(os.Getenv("XDG_RUNTIME_DIR"))
		engineConfig.SetDbusSessionBusAddress(os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
	}

	// Handle cgroups configuration (from limit flags, or provided conf file).
	cgJSON, err := getCgroupsJSON()
	if err != nil {
		sylog.Fatalf("While parsing cgroups configuration: %s", err)
	}
	engineConfig.SetCgroupsJSON(cgJSON)

	// --boot flag requires privilege, so check for this.
	err = withPrivilege(uid, IsBoot, "--boot", func() error { return nil })
	if err != nil {
		sylog.Fatalf("Could not configure --boot: %s", err)
	}

	// --containall or --boot infer --contain.
	if IsContained || IsContainAll || IsBoot {
		engineConfig.SetContain(true)
		// --containall infers PID/IPC isolation and a clean environment.
		if IsContainAll {
			PidNamespace = true
			IpcNamespace = true
			IsCleanEnv = true
		}
	}

	// Setup instance specific configuration if required.
	if instanceName != "" {
		PidNamespace = true
		engineConfig.SetInstance(true)
		engineConfig.SetBootInstance(IsBoot)

		if useSuid && !UserNamespace && hidepidProc() {
			sylog.Fatalf("hidepid option set on /proc mount, require 'hidepid=0' to start instance with setuid workflow")
		}

		_, err := instance.Get(instanceName, instance.AppSubDir)
		if err == nil {
			sylog.Fatalf("instance %s already exists", instanceName)
		}

		if IsBoot {
			UtsNamespace = true
			NetNamespace = true
			if Hostname == "" {
				engineConfig.SetHostname(instanceName)
			}
			if !KeepPrivs {
				engineConfig.SetDropCaps("CAP_SYS_BOOT,CAP_SYS_RAWIO")
			}
			generator.SetProcessArgs([]string{"/sbin/init"})
		}
	}

	// Set the required namespaces in the engine config.
	setNamespaces(uid, gid, engineConfig, generator)
	// Set the container environment.
	if err := setEnvVars(args, engineConfig, generator); err != nil {
		sylog.Fatalf("While setting environment: %s", err)
	}
	// Set the container process work directory.
	setProcessCwd(engineConfig, generator)

	generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "APPNAME", AppName)

	// Get image ready to run, if needed, via FUSE mount / extraction / image driver handling.
	if err := prepareImage(insideUserNs, image, cobraCmd, engineConfig, generator); err != nil {
		sylog.Fatalf("While preparing image: %s", err)
	}

	loadOverlay := false
	if !UserNamespace && starter.IsSuidInstall() {
		loadOverlay = true
	}

	cfg := &config.Common{
		EngineName:   apptainerConfig.Name,
		ContainerID:  instanceName,
		EngineConfig: engineConfig,
	}

	// Allow any plugins with callbacks to modify the assembled Config
	runPluginCallbacks(cfg)

	// Call the starter binary using our prepared config.
	if engineConfig.GetInstance() {
		err = starterInstance(loadOverlay, insideUserNs, instanceName, uid, useSuid, cfg, engineConfig)
	} else {
		err = starterInteractive(loadOverlay, useSuid, cfg, engineConfig)
	}

	// Execution is finished.
	if err != nil {
		sylog.Fatalf("While executing starter: %s", err)
	}
}

// setUmask saves the current umask, to be set for the process run in the container,
// unless the --no-umask option was specified.
// https://github.com/apptainer/singularity/issues/5214
func setUmask(engineConfig *apptainerConfig.EngineConfig) {
	currMask := syscall.Umask(0o022)
	if !NoUmask {
		sylog.Debugf("Saving umask %04o for propagation into container", currMask)
		engineConfig.SetUmask(currMask)
		engineConfig.SetRestoreUmask(true)
	}
}

// setTargetIDs sets engine configuration for any requested target UID and GID (when run as root).
// The effective uid and gid we will run under are returned as uid and gid.
func setTargetIDs(uid uint32, gid uint32, engineConfig *apptainerConfig.EngineConfig) (uint32, uint32, error) {
	// Identify requested uid/gif (if any) from --security options
	uidParam := security.GetParam(Security, "uid")
	gidParam := security.GetParam(Security, "gid")

	targetUID := 0
	targetGID := make([]int, 0)

	// If a target uid was requested, and we are root, handle that.
	err := withPrivilege(uid, uidParam != "", "uid security feature", func() error {
		u, err := strconv.ParseUint(uidParam, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to parse provided UID: %w", err)
		}
		targetUID = int(u)
		uid = uint32(targetUID)

		engineConfig.SetTargetUID(targetUID)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	// If any target gids were requested, and we are root, handle that.
	err = withPrivilege(uid, gidParam != "", "gid security feature", func() error {
		gids := strings.Split(gidParam, ":")
		for _, id := range gids {
			g, err := strconv.ParseUint(id, 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse provided GID: %w", err)
			}
			targetGID = append(targetGID, int(g))
		}
		if len(gids) > 0 {
			gid = uint32(targetGID[0])
		}

		engineConfig.SetTargetGID(targetGID)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	// Return the effective uid, gid the container will run with
	return uid, gid, nil
}

// setImageOrInstance sets the image to start, or instance and it's image to be joined.
func setImageOrInstance(image string, name string, uid uint32, engineConfig *apptainerConfig.EngineConfig, generator *generate.Generator) error {
	if strings.HasPrefix(image, "instance://") {
		if name != "" {
			return fmt.Errorf("Starting an instance from another is not allowed")
		}
		instanceName := instance.ExtractName(image)
		file, err := instance.Get(instanceName, instance.AppSubDir)
		if err != nil {
			return err
		}
		UserNamespace = file.UserNs
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", file.Image)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NAME", filepath.Base(file.Image))
		engineConfig.SetImage(image)
		engineConfig.SetInstanceJoin(true)

		// If we are running non-root, without a user ns, join the instance cgroup now, as we
		// can't manipulate the ppid cgroup in the engine
		// prepareInstanceJoinConfig().
		//
		// TODO - consider where /proc/sys/fs/cgroup is mounted in the engine
		// flow, to move this further down.
		if file.Cgroup && uid != 0 && !UserNamespace {
			pid := os.Getpid()
			sylog.Debugf("Adding process %d to instance cgroup", pid)
			manager, err := cgroups.GetManagerForPid(file.Pid)
			if err != nil {
				return fmt.Errorf("couldn't create cgroup manager: %w", err)
			}
			if err := manager.AddProc(pid); err != nil {
				return fmt.Errorf("couldn't add process to instance cgroup: %w", err)
			}
		}
	} else {
		abspath, err := filepath.Abs(image)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", abspath)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NAME", filepath.Base(abspath))
		if err != nil {
			return fmt.Errorf("Failed to determine image absolute path for %s: %w", image, err)
		}
		engineConfig.SetImage(abspath)
	}
	return nil
}

// checkEncryptionKey verifies key material is available if the image is encrypted.
// Allows us to fail fast if required key material is not available / usable.
func checkEncryptionKey(cobraCmd *cobra.Command, engineConfig *apptainerConfig.EngineConfig) error {
	if !engineConfig.GetInstanceJoin() {
		sylog.Debugf("Checking for encrypted system partition")
		img, err := imgutil.Init(engineConfig.GetImage(), false)
		if err != nil {
			return fmt.Errorf("could not open image %s: %w", engineConfig.GetImage(), err)
		}

		part, err := img.GetRootFsPartition()
		if err != nil {
			return fmt.Errorf("while getting root filesystem in %s: %w", engineConfig.GetImage(), err)
		}

		if part.Type == imgutil.ENCRYPTSQUASHFS {
			sylog.Debugf("Encrypted container filesystem detected")

			keyInfo, err := getEncryptionMaterial(cobraCmd)
			if err != nil {
				return fmt.Errorf("Cannot load key for decryption: %w", err)
			}

			plaintextKey, err := cryptkey.PlaintextKey(keyInfo, engineConfig.GetImage())
			if err != nil {
				sylog.Errorf("Please check you are providing the correct key for decryption")
				return fmt.Errorf("Cannot decrypt %s: %w", engineConfig.GetImage(), err)
			}

			engineConfig.SetEncryptionKey(plaintextKey)
		}
		// don't defer this call as in all cases it won't be
		// called before execing starter, so it would leak the
		// image file descriptor to the container process
		img.File.Close()
	}
	return nil
}

// useSuid checks whether to use the setuid starter binary, and if we need to force the user namespace.
func useSuid(insideUserNs bool, uid uint32, engineConfig *apptainerConfig.EngineConfig) (useSuid bool) {
	// privileged installation by default
	useSuid = true
	if !starter.IsSuidInstall() {
		// not a privileged installation
		useSuid = false

		if !UserNamespace && uid != 0 {
			sylog.Verbosef("Unprivileged installation: using user namespace")
			UserNamespace = true
		}
	}

	// use non privileged starter binary:
	// - if running as root
	// - if already running inside a user namespace
	// - if user namespace is requested
	// - if running as user and 'allow setuid = no' is set in apptainer.conf
	if uid == 0 || insideUserNs || UserNamespace || !engineConfig.File.AllowSetuid {
		useSuid = false

		// fallback to user namespace:
		// - for non root user with setuid installation and 'allow setuid = no'
		// - for root user without effective capability CAP_SYS_ADMIN
		if uid != 0 && starter.IsSuidInstall() && !engineConfig.File.AllowSetuid {
			sylog.Verbosef("'allow setuid' set to 'no' by configuration, fallback to user namespace")
			UserNamespace = true
		} else if uid == 0 && !UserNamespace {
			caps, err := capabilities.GetProcessEffective()
			if err != nil {
				sylog.Fatalf("Could not get process effective capabilities: %s", err)
			}
			if caps&uint64(1<<unix.CAP_SYS_ADMIN) == 0 {
				sylog.Verbosef("Effective capability CAP_SYS_ADMIN is missing, fallback to user namespace")
				UserNamespace = true
			}
		}
	}
	return useSuid
}

// setBinds sets engine configuration for requested bind mounts.
func setBinds(fakerootPath string, engineConfig *apptainerConfig.EngineConfig, generator *generate.Generator) error {
	// First get binds from -B/--bind and env var
	bindPaths := BindPaths
	binds, err := apptainerConfig.ParseBindPath(bindPaths)
	if err != nil {
		return fmt.Errorf("while parsing bind path: %w", err)
	}
	// Now add binds from one or more --mount and env var.
	// Note that these do not get exported for nested containers
	for _, m := range Mounts {
		bps, err := apptainerConfig.ParseMountString(m)
		if err != nil {
			return fmt.Errorf("while parsing mount %q: %w", m, err)
		}
		binds = append(binds, bps...)
	}

	if fakerootPath != "" {
		engineConfig.SetFakerootPath(fakerootPath)
		// Add binds for fakeroot command
		fakebindPaths, err := fakeroot.GetFakeBinds(fakerootPath)
		if err != nil {
			return fmt.Errorf("while getting fakeroot bindpoints: %w", err)
		}
		bindPaths = append(bindPaths, fakebindPaths...)
		fakebinds, err := apptainerConfig.ParseBindPath(fakebindPaths)
		if err != nil {
			return fmt.Errorf("while parsing fakeroot bind paths: %w", err)
		}
		binds = append(binds, fakebinds...)
	}

	engineConfig.SetBindPath(binds)

	for i, bindPath := range bindPaths {
		splits := strings.Split(bindPath, ":")
		if len(splits) > 1 {
			// For nesting, change the source to the destination
			//  because this level is bound at the destination
			if len(splits) > 2 {
				// Replace the source with the destination
				splits[0] = splits[1]
				bindPath = strings.Join(splits, ":")
			} else {
				// leave only the destination
				bindPath = splits[1]
			}
			bindPaths[i] = bindPath
		}
	}
	generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "BIND", strings.Join(bindPaths, ","))
	return nil
}

// setFuseMounts sets engine configuration for requested FUSE mounts.
func setFuseMounts(engineConfig *apptainerConfig.EngineConfig) error {
	if len(FuseMount) > 0 {
		/* If --fusemount is given, imply --pid */
		PidNamespace = true
		if err := engineConfig.SetFuseMount(FuseMount); err != nil {
			return fmt.Errorf("while setting fuse mount: %w", err)
		}
	}
	return nil
}

// Set engine flags to disable mounts, to allow overriding them if they are set true
// in the apptainer.conf.
func setNoMountFlags(c *apptainerConfig.EngineConfig) {
	skipBinds := []string{}
	for _, v := range NoMount {
		switch v {
		case "proc":
			c.SetNoProc(true)
		case "sys":
			c.SetNoSys(true)
		case "dev":
			c.SetNoDev(true)
		case "devpts":
			c.SetNoDevPts(true)
		case "home":
			c.SetNoHome(true)
		case "tmp":
			c.SetNoTmp(true)
		case "hostfs":
			c.SetNoHostfs(true)
		case "cwd":
			c.SetNoCwd(true)
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
	c.SetSkipBinds(skipBinds)
}

// setHome sets the correct home directory configuration for our circumstance.
// If it is not possible to mount a home directory then the mount will be disabled.
func setHome(customHome bool, engineConfig *apptainerConfig.EngineConfig) error {
	engineConfig.SetCustomHome(customHome)
	// If we have fakeroot & the home flag has not been used then we have the standard
	// /root location for the root user $HOME in the container.
	// This doesn't count as a SetCustomHome(true), as we are mounting from the real
	// user's standard $HOME -> /root and we want to respect --contain not mounting
	// the $HOME in this case.
	// See https://github.com/apptainer/singularity/pull/5227
	if !customHome && IsFakeroot {
		HomePath = fmt.Sprintf("%s:/root", HomePath)
	}
	// If we are running apptainer as root, but requesting a target UID in the container,
	// handle set the home directory appropriately.
	targetUID := engineConfig.GetTargetUID()
	if customHome && targetUID != 0 {
		if targetUID > 500 {
			if pwd, err := user.GetPwUID(uint32(targetUID)); err == nil {
				sylog.Debugf("Target UID requested, set home directory to %s", pwd.Dir)
				HomePath = pwd.Dir
				engineConfig.SetCustomHome(true)
			} else {
				sylog.Verbosef("Home directory for UID %d not found, home won't be mounted", targetUID)
				engineConfig.SetNoHome(true)
				HomePath = "/"
			}
		} else {
			sylog.Verbosef("System UID %d requested, home won't be mounted", targetUID)
			engineConfig.SetNoHome(true)
			HomePath = "/"
		}
	}

	// Handle any user request to override the home directory source/dest
	homeSlice := strings.Split(HomePath, ":")
	if len(homeSlice) > 2 || len(homeSlice) == 0 {
		return fmt.Errorf("home argument has incorrect number of elements: %v", len(homeSlice))
	}
	engineConfig.SetHomeSource(homeSlice[0])
	if len(homeSlice) == 1 {
		engineConfig.SetHomeDest(homeSlice[0])
	} else {
		engineConfig.SetHomeDest(homeSlice[1])
	}
	return nil
}

// SetGPUConfig sets up EngineConfig entries for NV / ROCm usage, if requested.
func SetGPUConfig(engineConfig *apptainerConfig.EngineConfig) error {
	if engineConfig.File.AlwaysUseNv && !NoNvidia {
		Nvidia = true
		sylog.Verbosef("'always use nv = yes' found in apptainer.conf")
	}
	if engineConfig.File.AlwaysUseRocm && !NoRocm {
		Rocm = true
		sylog.Verbosef("'always use rocm = yes' found in apptainer.conf")
	}

	if NvCCLI && !Nvidia {
		sylog.Debugf("implying --nv from --nvccli")
		Nvidia = true
	}

	if Nvidia && Rocm {
		sylog.Warningf("--nv and --rocm cannot be used together. Only --nv will be applied.")
	}

	if Nvidia {
		// If nvccli was not enabled by flag or config, drop down to legacy binds immediately
		if !engineConfig.File.UseNvCCLI && !NvCCLI {
			return setNVLegacyConfig(engineConfig)
		}

		// TODO: In privileged fakeroot mode we don't have the correct namespace context to run nvidia-container-cli
		// from  starter, so fall back to legacy NV handling until that workflow is refactored heavily.
		fakeRootPriv := IsFakeroot && engineConfig.File.AllowSetuid && starter.IsSuidInstall()
		if !fakeRootPriv {
			return setNvCCLIConfig(engineConfig)
		}
		return fmt.Errorf("--fakeroot does not support --nvccli in set-uid installations")
	}

	if Rocm {
		return setRocmConfig(engineConfig)
	}
	return nil
}

// setNvCCLIConfig sets up EngineConfig entries for NVIDIA GPU configuration via nvidia-container-cli.
func setNvCCLIConfig(engineConfig *apptainerConfig.EngineConfig) (err error) {
	sylog.Debugf("Using nvidia-container-cli for GPU setup")
	engineConfig.SetNvCCLI(true)

	if os.Getenv("NVIDIA_VISIBLE_DEVICES") == "" {
		if IsContained || IsContainAll {
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
	engineConfig.SetNvCCLIEnv(nvCCLIEnv)

	if !IsWritable && !IsWritableTmpfs {
		sylog.Infof("Setting --writable-tmpfs (required by nvidia-container-cli)")
		IsWritableTmpfs = true
	}

	return nil
}

// setNvLegacyConfig sets up EngineConfig entries for NVIDIA GPU configuration via direct binds of configured bins/libs.
func setNVLegacyConfig(engineConfig *apptainerConfig.EngineConfig) error {
	sylog.Debugf("Using legacy binds for nv GPU setup")
	engineConfig.SetNvLegacy(true)
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
	setGPUBinds(libs, bins, ipcs, "nv", engineConfig)
	return nil
}

// setRocmConfig sets up EngineConfig entries for ROCm GPU configuration via direct binds of configured bins/libs.
func setRocmConfig(engineConfig *apptainerConfig.EngineConfig) error {
	sylog.Debugf("Using rocm GPU setup")
	engineConfig.SetRocm(true)
	gpuConfFile := filepath.Join(buildcfg.APPTAINER_CONFDIR, "rocmliblist.conf")
	libs, bins, err := gpu.RocmPaths(gpuConfFile)
	if err != nil {
		sylog.Warningf("While finding ROCm bind points: %v", err)
	}
	setGPUBinds(libs, bins, []string{}, "nv", engineConfig)
	return nil
}

// setGPUBinds sets EngineConfig entries to bind the provided list of libs, bins, ipc files.
func setGPUBinds(libs, bins, ipcs []string, gpuPlatform string, engineConfig *apptainerConfig.EngineConfig) {
	files := make([]string, len(bins)+len(ipcs))
	if len(files) == 0 {
		sylog.Warningf("Could not find any %s files on this host!", gpuPlatform)
	} else {
		if IsWritable {
			sylog.Warningf("%s files may not be bound with --writable", gpuPlatform)
		}
		for i, binary := range bins {
			usrBinBinary := filepath.Join("/usr/bin", filepath.Base(binary))
			files[i] = strings.Join([]string{binary, usrBinBinary}, ":")
		}
		for i, ipc := range ipcs {
			files[i+len(bins)] = ipc
		}
		engineConfig.SetFilesPath(files)
	}
	if len(libs) == 0 {
		sylog.Warningf("Could not find any %s libraries on this host!", gpuPlatform)
	} else {
		engineConfig.SetLibrariesPath(libs)
	}
}

// setNamespaces sets namespace configuration for the engine.
func setNamespaces(uid uint32, gid uint32, engineConfig *apptainerConfig.EngineConfig, generator *generate.Generator) {
	if !NetNamespace && Network != "" {
		sylog.Infof("Setting --net (required by --network)")
		NetNamespace = true
	}
	if !NetNamespace && len(NetworkArgs) != 0 {
		sylog.Infof("Setting --net (required by --network-args)")
		NetNamespace = true
	}
	if NetNamespace {
		if Network == "" {
			Network = "bridge"
			engineConfig.SetNetwork(Network)
		}
		if IsFakeroot && Network != "none" {
			engineConfig.SetNetwork("fakeroot")

			// unprivileged installation could not use fakeroot
			// network because it requires a setuid installation
			// so we fallback to none
			if !starter.IsSuidInstall() || !engineConfig.File.AllowSetuid {
				sylog.Warningf(
					"fakeroot with unprivileged installation or 'allow setuid = no' " +
						"could not use 'fakeroot' network, fallback to 'none' network",
				)
				engineConfig.SetNetwork("none")
			}
		}
		generator.AddOrReplaceLinuxNamespace("network", "")
	}
	if UtsNamespace {
		generator.AddOrReplaceLinuxNamespace("uts", "")
	}
	if PidNamespace {
		generator.AddOrReplaceLinuxNamespace("pid", "")
		engineConfig.SetNoInit(NoInit)
	}
	if IpcNamespace {
		generator.AddOrReplaceLinuxNamespace("ipc", "")
	}
	if UserNamespace {
		generator.AddOrReplaceLinuxNamespace("user", "")
		if !IsFakeroot {
			generator.AddLinuxUIDMapping(uid, uid, 1)
			generator.AddLinuxGIDMapping(gid, gid, 1)
		}
	}
}

// setEnvVars sets the environment for the container, from the host environment, glads, env-file.
func setEnvVars(args []string, engineConfig *apptainerConfig.EngineConfig, generator *generate.Generator) error {
	if ApptainerEnvFile != "" {
		currentEnv := append(
			os.Environ(),
			"APPTAINER_IMAGE="+engineConfig.GetImage(),
		)

		content, err := os.ReadFile(ApptainerEnvFile)
		if err != nil {
			return fmt.Errorf("Could not read %q environment file: %w", ApptainerEnvFile, err)
		}

		envvars, err := interpreter.EvaluateEnv(content, args, currentEnv)
		if err != nil {
			return fmt.Errorf("While processing %s: %w", ApptainerEnvFile, err)
		}
		// --env variables will take precedence over variables
		// defined by the environment file
		sylog.Debugf("Setting environment variables from file %s", ApptainerEnvFile)

		// Update ApptainerEnv with those from file
		for _, envar := range envvars {
			e := strings.SplitN(envar, "=", 2)
			if len(e) != 2 {
				sylog.Warningf("Ignore environment variable %q: '=' is missing", envar)
				continue
			}
			// Ensure we don't overwrite --env variables with environment file
			if _, ok := ApptainerEnv[e[0]]; ok {
				sylog.Warningf("Ignore environment variable %s from %s: override from --env", e[0], ApptainerEnvFile)
			} else {
				ApptainerEnv[e[0]] = e[1]
			}
		}
	}
	// process --env and --env-file variables for injection
	// into the environment by prefixing them with APPTAINERENV_
	for envName, envValue := range ApptainerEnv {
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
	apptainerEnv := env.SetContainerEnv(generator, environment, IsCleanEnv, engineConfig.GetHomeDest())
	engineConfig.SetApptainerEnv(apptainerEnv)
	return nil
}

// setProcessCwd sets the container process working directory
func setProcessCwd(engineConfig *apptainerConfig.EngineConfig, generator *generate.Generator) {
	if pwd, err := os.Getwd(); err == nil {
		engineConfig.SetCwd(pwd)
		if PwdPath != "" {
			generator.SetProcessCwd(PwdPath)
			if generator.Config.Annotations == nil {
				generator.Config.Annotations = make(map[string]string)
			}
			generator.Config.Annotations["CustomCwd"] = "true"
		} else {
			if engineConfig.GetContain() {
				generator.SetProcessCwd(engineConfig.GetHomeDest())
			} else {
				generator.SetProcessCwd(pwd)
			}
		}
	} else {
		sylog.Warningf("can't determine current working directory: %s", err)
	}
}

// PrepareImage performs any image preparation required before execution.
// This is currently limited to extraction or FUSE mount when using the user namespace,
// and activating any image driver plugins that might handle the image mount.
func prepareImage(insideUserNs bool, image string, cobraCmd *cobra.Command, engineConfig *apptainerConfig.EngineConfig, generator *generate.Generator) error {
	// initialize internal image drivers
	var desiredFeatures imgutil.DriverFeature
	if fs.IsFile(image) {
		desiredFeatures = imgutil.ImageFeature
	}
	driver.InitImageDrivers(true, UserNamespace || insideUserNs, engineConfig.File, desiredFeatures)

	// convert image file to sandbox if either it was requested by
	// `--unsquash` or if we are inside of a user namespace and there's
	// no image driver.
	if fs.IsFile(image) {
		convert := false
		if Unsquash {
			convert = true
		} else if UserNamespace || insideUserNs {
			convert = true
			if engineConfig.File.ImageDriver != "" {
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
				driver := imgutil.GetDriver(engineConfig.File.ImageDriver)
				if driver != nil && driver.Features()&imgutil.ImageFeature != 0 {
					// the image driver indicates support for image so let's
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
			rootfsDir, imageDir, err := convertImage(image, unsquashfsPath, tmpDir)
			if err != nil {
				sylog.Fatalf("while extracting %s: %s", image, err)
			}
			engineConfig.SetImage(imageDir)
			engineConfig.SetDeleteTempDir(rootfsDir)
			generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", imageDir)
			// if '--disable-cache' flag, then remove original SIF after converting to sandbox
			if disableCache {
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
func starterInteractive(loadOverlay bool, useSuid bool, cfg *config.Common, engineConfig *apptainerConfig.EngineConfig) error {
	err := starter.Exec(
		"Apptainer runtime parent",
		cfg,
		starter.UseSuid(useSuid),
		starter.LoadOverlayModule(loadOverlay),
	)
	return err
}

// starterInstance executes the starter binary to run an instance given the supplied engineConfig
func starterInstance(loadOverlay bool, insideUserNs bool, name string, uid uint32, useSuid bool, cfg *config.Common, engineConfig *apptainerConfig.EngineConfig) error {
	pwd, err := user.GetPwUID(uid)
	if err != nil {
		return fmt.Errorf("failed to retrieve user information for UID %d: %w", uid, err)
	}
	procname, err := instance.ProcName(name, pwd.Name)
	if err != nil {
		return err
	}

	stdout, stderr, err := instance.SetLogFile(name, UserNamespace || insideUserNs, int(uid), instance.LogSubDir)
	if err != nil {
		return fmt.Errorf("failed to create instance log files: %w", err)
	}

	start, err := stderr.Seek(0, io.SeekEnd)
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
		return fmt.Errorf("While loading plugins callbacks '%T': %w", callbackType, err)
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
		sylog.Errorf("File %q is an ext3 format continer image.", filename)
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
func SetCheckpointConfig(engineConfig *apptainerConfig.EngineConfig) error {
	if DMTCPLaunch == "" && DMTCPRestart == "" {
		return nil
	}

	return injectDMTCPConfig(engineConfig)
}

func injectDMTCPConfig(engineConfig *apptainerConfig.EngineConfig) error {
	sylog.Debugf("Injecting DMTCP configuration")
	dmtcp.QuickInstallationCheck()

	bins, libs, err := dmtcp.GetPaths()
	if err != nil {
		return err
	}

	var config apptainerConfig.DMTCPConfig
	if DMTCPRestart != "" {
		config = apptainerConfig.DMTCPConfig{
			Enabled:    true,
			Restart:    true,
			Checkpoint: DMTCPRestart,
			Args:       dmtcp.RestartArgs(),
		}
	} else {
		config = apptainerConfig.DMTCPConfig{
			Enabled:    true,
			Restart:    false,
			Checkpoint: DMTCPLaunch,
			Args:       dmtcp.LaunchArgs(),
		}
	}

	m := dmtcp.NewManager()
	e, err := m.Get(config.Checkpoint)
	if err != nil {
		return err
	}

	sylog.Debugf("Injecting checkpoint state bind: %q", config.Checkpoint)
	engineConfig.SetBindPath(append(engineConfig.GetBindPath(), e.BindPath()))
	engineConfig.AppendFilesPath(bins...)
	engineConfig.AppendLibrariesPath(libs...)
	engineConfig.SetDMTCPConfig(config)

	return nil
}
