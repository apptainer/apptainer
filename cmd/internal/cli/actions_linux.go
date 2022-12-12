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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
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
	rootfsDir, err = ioutil.TempDir(tmpDir, "rootfs-")
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

// checkHidepid checks if hidepid is set on /proc mount point, when this
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

// Set engine flags to disable mounts, to allow overriding them if they are set true
// in the apptainer.conf
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
		default:
			if filepath.IsAbs(v) {
				skipBinds = append(skipBinds, v)
				continue
			}
			sylog.Warningf("Ignoring unknown mount type '%s'", v)
		}
	}
	c.SetSkipBinds(skipBinds)
}

// TODO: Let's stick this in another file so that that CLI is just CLI
//nolint:maintidx
func execStarter(cobraCmd *cobra.Command, image string, args []string, name string) {
	var err error

	targetUID := 0
	targetGID := make([]int, 0)

	procname := ""

	uid := uint32(os.Getuid())
	gid := uint32(os.Getgid())
	insideUserNs, _ := namespaces.IsInsideUserNamespace(os.Getpid())

	// Are we running from a privileged account?
	isPrivileged := uid == 0

	var fakerootPath string
	if IsFakeroot {
		if isPrivileged && namespaces.IsUnprivileged() {
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
		} else if !isPrivileged && (!fakeroot.IsUIDMapped(uid) || IgnoreSubuid) {
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

	withPrivilege := func(priv bool, cond bool, desc string, fn func()) {
		if !cond {
			return
		}

		if !priv {
			sylog.Fatalf("%s requires root privileges", desc)
		}

		fn()
	}

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

	generator.SetProcessArgs(args)

	currMask := syscall.Umask(0o022)
	if !NoUmask {
		// Save the current umask, to be set for the process run in the container
		// https://github.com/apptainer/singularity/issues/5214
		sylog.Debugf("Saving umask %04o for propagation into container", currMask)
		engineConfig.SetUmask(currMask)
		engineConfig.SetRestoreUmask(true)
	}

	if NoEval {
		engineConfig.SetNoEval(true)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NO_EVAL", "1")
	}

	useSuid := true
	if buildcfg.APPTAINER_SUID_INSTALL == 0 {
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
		if uid != 0 && buildcfg.APPTAINER_SUID_INSTALL == 1 && !engineConfig.File.AllowSetuid {
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

	// IgnoreUserns is a hidden control flag
	UserNamespace = UserNamespace && !IgnoreUserns

	uidParam := security.GetParam(Security, "uid")
	gidParam := security.GetParam(Security, "gid")

	pseudoPriv := isPrivileged
	if !useSuid {
		// always allow when not using suid starter
		pseudoPriv = true
	}

	// handle target UID/GID for root or non-suid user
	withPrivilege(pseudoPriv, uidParam != "", "uid security feature with suid mode", func() {
		u, err := strconv.ParseUint(uidParam, 10, 32)
		if err != nil {
			sylog.Fatalf("failed to parse provided UID")
		}
		targetUID = int(u)
		uid = uint32(targetUID)

		engineConfig.SetTargetUID(targetUID)
	})

	withPrivilege(pseudoPriv, gidParam != "", "gid security feature with suid mode", func() {
		gids := strings.Split(gidParam, ":")
		for _, id := range gids {
			g, err := strconv.ParseUint(id, 10, 32)
			if err != nil {
				sylog.Fatalf("failed to parse provided GID")
			}
			targetGID = append(targetGID, int(g))
		}
		if len(gids) > 0 {
			gid = uint32(targetGID[0])
		}

		engineConfig.SetTargetGID(targetGID)
	})

	if strings.HasPrefix(image, "instance://") {
		if name != "" {
			sylog.Fatalf("Starting an instance from another is not allowed")
		}
		instanceName := instance.ExtractName(image)
		file, err := instance.Get(instanceName, instance.AppSubDir)
		if err != nil {
			sylog.Fatalf("%s", err)
		}
		UserNamespace = file.UserNs
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", file.Image)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NAME", filepath.Base(file.Image))
		engineConfig.SetImage(image)
		engineConfig.SetInstanceJoin(true)
	} else {
		abspath, err := filepath.Abs(image)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "CONTAINER", abspath)
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "NAME", filepath.Base(abspath))
		if err != nil {
			sylog.Fatalf("Failed to determine image absolute path for %s: %s", image, err)
		}
		engineConfig.SetImage(abspath)
	}

	// early check for key material before we start engine so we can fail fast if missing
	// we do not need this check when joining a running instance, just for starting a container
	if !engineConfig.GetInstanceJoin() {
		sylog.Debugf("Checking for encrypted system partition")
		img, err := imgutil.Init(engineConfig.GetImage(), false)
		if err != nil {
			sylog.Fatalf("could not open image %s: %s", engineConfig.GetImage(), err)
		}

		part, err := img.GetRootFsPartition()
		if err != nil {
			sylog.Fatalf("while getting root filesystem in %s: %s", engineConfig.GetImage(), err)
		}

		// ensure we have decryption material
		if part.Type == imgutil.ENCRYPTSQUASHFS {
			sylog.Debugf("Encrypted container filesystem detected")

			keyInfo, err := getEncryptionMaterial(cobraCmd)
			if err != nil {
				sylog.Fatalf("Cannot load key for decryption: %v", err)
			}

			plaintextKey, err := cryptkey.PlaintextKey(keyInfo, engineConfig.GetImage())
			if err != nil {
				sylog.Errorf("Cannot decrypt %s: %v", engineConfig.GetImage(), err)
				sylog.Fatalf("Please check you are providing the correct key for decryption")
			}

			engineConfig.SetEncryptionKey(plaintextKey)
		}

		// don't defer this call as in all cases it won't be
		// called before execing starter, so it would leak the
		// image file descriptor to the container process
		img.File.Close()
	}

	// First get binds from -B/--bind and env var
	bindPaths := BindPaths
	binds, err := apptainerConfig.ParseBindPath(bindPaths)
	if err != nil {
		sylog.Fatalf("while parsing bind path: %s", err)
	}

	// Now add binds from one or more --mount and env var.
	// Note that these do not get exported for nested containers
	for _, m := range Mounts {
		bps, err := apptainerConfig.ParseMountString(m)
		if err != nil {
			sylog.Fatalf("while parsing mount %q: %s", m, err)
		}
		binds = append(binds, bps...)
	}

	if fakerootPath != "" {
		engineConfig.SetFakerootPath(fakerootPath)
		// Add binds for fakeroot command
		fakebindPaths, err := fakeroot.GetFakeBinds(fakerootPath)
		if err != nil {
			sylog.Fatalf("while getting fakeroot bindpoints: %v", err)
		}
		bindPaths = append(bindPaths, fakebindPaths...)
		fakebinds, err := apptainerConfig.ParseBindPath(fakebindPaths)
		if err != nil {
			sylog.Fatalf("while parsing fakeroot bind paths: %s", err)
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

	if len(FuseMount) > 0 {
		/* If --fusemount is given, imply --pid */
		PidNamespace = true
		if err := engineConfig.SetFuseMount(FuseMount); err != nil {
			sylog.Fatalf("while setting fuse mount: %s", err)
		}
	}
	engineConfig.SetNetwork(Network)
	engineConfig.SetDNS(DNS)
	engineConfig.SetNetworkArgs(NetworkArgs)
	engineConfig.SetOverlayImage(OverlayPath)
	engineConfig.SetWritableImage(IsWritable)
	engineConfig.SetNoHome(NoHome)
	setNoMountFlags(engineConfig)

	if err := SetGPUConfig(engineConfig); err != nil {
		// We must fatal on error, as we are checking for correct ownership of nvidia-container-cli,
		// which is important to maintain security.
		sylog.Fatalf("while setting GPU configuration: %s", err)
	}

	if err := SetCheckpointConfig(engineConfig); err != nil {
		sylog.Fatalf("while setting checkpoint configuration: %s", err)
	}

	engineConfig.SetAddCaps(AddCaps)
	engineConfig.SetDropCaps(DropCaps)
	engineConfig.SetConfigurationFile(configurationFile)
	engineConfig.SetUseBuildConfig(useBuildConfig)

	withPrivilege(isPrivileged, AllowSUID, "--allow-setuid", func() {
		engineConfig.SetAllowSUID(AllowSUID)
	})

	withPrivilege(isPrivileged, KeepPrivs, "--keep-privs", func() {
		engineConfig.SetKeepPrivs(KeepPrivs)
	})

	engineConfig.SetNoPrivs(NoPrivs)
	engineConfig.SetSecurity(Security)
	engineConfig.SetShell(ShellPath)
	engineConfig.AppendLibrariesPath(ContainLibsPath...)
	engineConfig.SetFakeroot(IsFakeroot)

	if ShellPath != "" {
		generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "SHELL", ShellPath)
	}

	if name != "" && uid != 0 && CgroupsTOMLFile != "" {
		sylog.Fatalf("Instances do not currently support rootless cgroups")
	}

	if uid != 0 {
		sylog.Debugf("Recording rootless XDG_RUNTIME_DIR / DBUS_SESSION_BUS_ADDRESS")
		engineConfig.SetXdgRuntimeDir(os.Getenv("XDG_RUNTIME_DIR"))
		engineConfig.SetDbusSessionBusAddress(os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
	}

	cgJSON, err := getCgroupsJSON()
	if err != nil {
		sylog.Fatalf("While parsing cgroups configuration: %s", err)
	}
	engineConfig.SetCgroupsJSON(cgJSON)

	if IsWritable && IsWritableTmpfs {
		sylog.Warningf("Disabling --writable-tmpfs flag, mutually exclusive with --writable")
		engineConfig.SetWritableTmpfs(false)
	} else {
		engineConfig.SetWritableTmpfs(IsWritableTmpfs)
	}

	homeFlag := cobraCmd.Flag("home")
	engineConfig.SetCustomHome(homeFlag.Changed)

	// If we have fakeroot & the home flag has not been used then we have the standard
	// /root location for the root user $HOME in the container.
	// This doesn't count as a SetCustomHome(true), as we are mounting from the real
	// user's standard $HOME -> /root and we want to respect --contain not mounting
	// the $HOME in this case.
	// See https://github.com/apptainer/singularity/pull/5227
	if !homeFlag.Changed && IsFakeroot {
		HomePath = fmt.Sprintf("%s:/root", HomePath)
	}

	// set home directory for the targeted UID if it exists on host system
	if !homeFlag.Changed && targetUID != 0 {
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

	if Hostname != "" {
		UtsNamespace = true
		engineConfig.SetHostname(Hostname)
	}

	withPrivilege(isPrivileged, IsBoot, "--boot", func() {})

	if IsContained || IsContainAll || IsBoot {
		engineConfig.SetContain(true)

		if IsContainAll {
			PidNamespace = true
			IpcNamespace = true
			IsCleanEnv = true
		}
	}

	engineConfig.SetScratchDir(ScratchPath)
	engineConfig.SetWorkdir(WorkdirPath)

	homeSlice := strings.Split(HomePath, ":")

	if len(homeSlice) > 2 || len(homeSlice) == 0 {
		sylog.Fatalf("home argument has incorrect number of elements: %v", len(homeSlice))
	}

	engineConfig.SetHomeSource(homeSlice[0])
	if len(homeSlice) == 1 {
		engineConfig.SetHomeDest(homeSlice[0])
	} else {
		engineConfig.SetHomeDest(homeSlice[1])
	}

	if IsFakeroot {
		UserNamespace = !IgnoreUserns
	}

	/* if name submitted, run as instance */
	if name != "" {
		PidNamespace = true
		engineConfig.SetInstance(true)
		engineConfig.SetBootInstance(IsBoot)

		if useSuid && !UserNamespace && hidepidProc() {
			sylog.Fatalf("hidepid option set on /proc mount, require 'hidepid=0' to start instance with setuid workflow")
		}

		_, err := instance.Get(name, instance.AppSubDir)
		if err == nil {
			sylog.Fatalf("instance %s already exists", name)
		}

		if IsBoot {
			UtsNamespace = true
			NetNamespace = true
			if Hostname == "" {
				engineConfig.SetHostname(name)
			}
			if !KeepPrivs {
				engineConfig.SetDropCaps("CAP_SYS_BOOT,CAP_SYS_RAWIO")
			}
			generator.SetProcessArgs([]string{"/sbin/init"})
		}
		pwd, err := user.GetPwUID(uint32(os.Getuid()))
		if err != nil {
			sylog.Fatalf("failed to retrieve user information for UID %d: %s", os.Getuid(), err)
		}
		procname, err = instance.ProcName(name, pwd.Name)
		if err != nil {
			sylog.Fatalf("%s", err)
		}
	} else {
		generator.SetProcessArgs(args)
		procname = "Apptainer runtime parent"
	}

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
			if buildcfg.APPTAINER_SUID_INSTALL == 0 || !engineConfig.File.AllowSetuid {
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
			generator.AddLinuxUIDMapping(uint32(os.Getuid()), uid, 1)
			generator.AddLinuxGIDMapping(uint32(os.Getgid()), gid, 1)
		}
	}

	if ApptainerEnvFile != "" {
		currentEnv := append(
			os.Environ(),
			"APPTAINER_IMAGE="+engineConfig.GetImage(),
		)

		content, err := ioutil.ReadFile(ApptainerEnvFile)
		if err != nil {
			sylog.Fatalf("Could not read %q environment file: %s", ApptainerEnvFile, err)
		}

		envvars, err := interpreter.EvaluateEnv(content, args, currentEnv)
		if err != nil {
			sylog.Fatalf("While processing %s: %s", ApptainerEnvFile, err)
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

	// starter will force the loading of kernel overlay module
	loadOverlay := false
	if !UserNamespace && buildcfg.APPTAINER_SUID_INSTALL == 1 {
		loadOverlay = true
	}

	generator.SetProcessEnvWithPrefixes(env.ApptainerPrefixes, "APPNAME", AppName)

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
					sylog.Errorf("unable to remove tmp image: %s: %v", image, err)
				}
			}
		}
	}

	// setuid workflow set RLIMIT_STACK to its default value,
	// get the original value to restore it before executing
	// container process
	if useSuid {
		soft, hard, err := rlimit.Get("RLIMIT_STACK")
		if err != nil {
			sylog.Warningf("can't retrieve stack size limit: %s", err)
		}
		generator.AddProcessRlimits("RLIMIT_STACK", hard, soft)
	}

	cfg := &config.Common{
		EngineName:   apptainerConfig.Name,
		ContainerID:  name,
		EngineConfig: engineConfig,
	}

	callbackType := (clicallback.ApptainerEngineConfig)(nil)
	callbacks, err := plugin.LoadCallbacks(callbackType)
	if err != nil {
		sylog.Fatalf("While loading plugins callbacks '%T': %s", callbackType, err)
	}
	for _, c := range callbacks {
		c.(clicallback.ApptainerEngineConfig)(cfg)
	}

	if engineConfig.GetInstance() {
		stdout, stderr, err := instance.SetLogFile(name, UserNamespace || insideUserNs, int(uid), instance.LogSubDir)
		if err != nil {
			sylog.Fatalf("failed to create instance log files: %s", err)
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
			sylog.Fatalf("failed to start instance: %s", cmdErr)
		} else {
			sylog.Verbosef("you will find instance output here: %s", stdout.Name())
			sylog.Verbosef("you will find instance error here: %s", stderr.Name())
			sylog.Infof("instance started successfully")
		}
	} else {
		err := starter.Exec(
			procname,
			cfg,
			starter.UseSuid(useSuid),
			starter.LoadOverlayModule(loadOverlay),
		)
		sylog.Fatalf("%s", err)
	}
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
		fakeRootPriv := IsFakeroot && engineConfig.File.AllowSetuid && buildcfg.APPTAINER_SUID_INSTALL == 1
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

// setNvCCLIConfig sets up EngineConfig entries for NVIDIA GPU configuration via nvidia-container-cli
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
	setGPUBinds(engineConfig, libs, bins, ipcs, "nv")
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
	setGPUBinds(engineConfig, libs, bins, []string{}, "nv")
	return nil
}

// setGPUBinds sets EngineConfig entries to bind the provided list of libs, bins, ipc files.
func setGPUBinds(engineConfig *apptainerConfig.EngineConfig, libs, bins, ipcs []string, gpuPlatform string) {
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
