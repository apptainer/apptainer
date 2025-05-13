// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package fakeroot

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	fakerootutil "github.com/apptainer/apptainer/internal/pkg/fakeroot"
	"github.com/apptainer/apptainer/internal/pkg/plugin"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/starter"
	fakerootConfig "github.com/apptainer/apptainer/internal/pkg/runtime/engine/fakeroot/config"
	"github.com/apptainer/apptainer/internal/pkg/security/seccomp"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	fakerootcallback "github.com/apptainer/apptainer/pkg/plugin/callback/runtime/fakeroot"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// EngineOperations is an Apptainer fakeroot runtime engine that implements engine.Operations.
type EngineOperations struct {
	CommonConfig *config.Common               `json:"-"`
	EngineConfig *fakerootConfig.EngineConfig `json:"engineConfig"`
}

// InitConfig stores the parsed config.Common inside the engine.
//
// Since this method simply stores config.Common, it does not matter
// whether or not there are any elevated privileges during this call.
func (e *EngineOperations) InitConfig(cfg *config.Common, _ bool) {
	e.CommonConfig = cfg
}

// Config returns a pointer to a fakerootConfig.EngineConfig
// literal as a config.EngineConfig interface. This pointer
// gets stored in the engine.Engine.Common field.
//
// Since this method simply returns a zero value of the concrete
// EngineConfig, it does not matter whether or not there are any elevated
// privileges during this call.
func (e *EngineOperations) Config() config.EngineConfig {
	return e.EngineConfig
}

// PrepareConfig is called during stage1 to validate and prepare
// container configuration. It is responsible for apptainer
// configuration file parsing, reading capabilities, configuring
// UID/GID mappings, etc.
//
// No additional privileges can be gained as any of them are already
// dropped by the time PrepareConfig is called.
func (e *EngineOperations) PrepareConfig(starterConfig *starter.Config) error {
	g := generate.New(nil)

	configurationFile := buildcfg.APPTAINER_CONF_FILE

	// check for ownership of apptainer.conf
	if starterConfig.GetIsSUID() && !fs.IsOwner(configurationFile, 0) {
		return fmt.Errorf("%s must be owned by root", configurationFile)
	}

	fileConfig, err := apptainerconf.Parse(configurationFile)
	if err != nil {
		return fmt.Errorf("unable to parse apptainer.conf file: %s", err)
	}
	apptainerconf.SetCurrentConfig(fileConfig)
	apptainerconf.SetBinaryPath(buildcfg.LIBEXECDIR, !starterConfig.GetIsSUID())

	if starterConfig.GetIsSUID() {
		if !fileConfig.AllowSetuid {
			return fmt.Errorf("fakeroot requires to set 'allow setuid = yes' in %s", configurationFile)
		}
	} else {
		sylog.Verbosef("Fakeroot requested with unprivileged workflow, fallback to newuidmap/newgidmap")
		sylog.Debugf("Search for newuidmap binary")
		if err := starterConfig.SetNewUIDMapPath(); err != nil {
			return err
		}
		sylog.Debugf("Search for newgidmap binary")
		if err := starterConfig.SetNewGIDMapPath(); err != nil {
			return err
		}
	}

	g.AddOrReplaceLinuxNamespace(specs.UserNamespace, "")
	g.AddOrReplaceLinuxNamespace(specs.MountNamespace, "")
	g.AddOrReplaceLinuxNamespace(specs.PIDNamespace, "")

	uid := uint32(os.Getuid())
	gid := uint32(os.Getgid())

	getIDRange := fakerootutil.GetIDRange

	callbackType := (fakerootcallback.UserMapping)(nil)
	callbacks, err := plugin.LoadCallbacks(callbackType)
	if err != nil {
		return fmt.Errorf("while loading plugins callbacks '%T': %s", callbackType, err)
	}
	if len(callbacks) > 1 {
		return fmt.Errorf("multiple plugins have registered hook callback for fakeroot")
	} else if len(callbacks) == 1 {
		getIDRange = callbacks[0].(fakerootcallback.UserMapping)
	}

	g.AddLinuxUIDMapping(uid, 0, 1)
	idRange, err := getIDRange(fakerootutil.SubUIDFile, uid)
	if err != nil {
		return fmt.Errorf("could not use fakeroot: %s", err)
	}
	g.AddLinuxUIDMapping(idRange.HostID, idRange.ContainerID, idRange.Size)
	starterConfig.AddUIDMappings(g.Config.Linux.UIDMappings)

	g.AddLinuxGIDMapping(gid, 0, 1)
	idRange, err = getIDRange(fakerootutil.SubGIDFile, uid)
	if err != nil {
		return fmt.Errorf("could not use fakeroot: %s", err)
	}
	g.AddLinuxGIDMapping(idRange.HostID, idRange.ContainerID, idRange.Size)
	starterConfig.AddGIDMappings(g.Config.Linux.GIDMappings)

	starterConfig.SetHybridWorkflow(true)
	starterConfig.SetAllowSetgroups(true)

	starterConfig.SetTargetUID(0)
	starterConfig.SetTargetGID([]int{0})

	if g.Config.Linux != nil {
		starterConfig.SetNsFlagsFromSpec(g.Config.Linux.Namespaces)
	}

	g.SetupPrivileged(true)

	starterConfig.SetCapabilities(capabilities.Permitted, g.Config.Process.Capabilities.Permitted)
	starterConfig.SetCapabilities(capabilities.Effective, g.Config.Process.Capabilities.Effective)
	starterConfig.SetCapabilities(capabilities.Inheritable, g.Config.Process.Capabilities.Inheritable)
	starterConfig.SetCapabilities(capabilities.Bounding, g.Config.Process.Capabilities.Bounding)
	starterConfig.SetCapabilities(capabilities.Ambient, g.Config.Process.Capabilities.Ambient)

	return nil
}

// CreateContainer does nothing for the fakeroot engine.
func (e *EngineOperations) CreateContainer(context.Context, int, net.Conn) error {
	return nil
}

// fakerootSeccompProfile returns a seccomp filter allowing to
// set the return value to 0 for mknod and mknodat syscalls. It
// allows build bootstrap like yum to work with fakeroot.
func fakerootSeccompProfile() *specs.LinuxSeccomp {
	// sets filters allowing to create file/pipe/socket
	// and turns mknod/mknodat as no-op syscalls for block
	// devices and character devices having a device number
	// greater than 0
	syscalls := []specs.LinuxSyscall{
		{
			Names:  []string{"mknod"},
			Action: specs.ActErrno,
			Args: []specs.LinuxSeccompArg{
				{
					Index:    1,
					Value:    syscall.S_IFBLK,
					ValueTwo: syscall.S_IFBLK,
					Op:       specs.OpMaskedEqual,
				},
			},
		},
		{
			Names:  []string{"mknod"},
			Action: specs.ActErrno,
			Args: []specs.LinuxSeccompArg{
				{
					Index:    1,
					Value:    syscall.S_IFCHR,
					ValueTwo: syscall.S_IFCHR,
					Op:       specs.OpMaskedEqual,
				},
				{
					Index:    2,
					Value:    0,
					ValueTwo: 0,
					Op:       specs.OpNotEqual,
				},
			},
		},
		{
			Names:  []string{"mknodat"},
			Action: specs.ActErrno,
			Args: []specs.LinuxSeccompArg{
				{
					Index:    2,
					Value:    syscall.S_IFBLK,
					ValueTwo: syscall.S_IFBLK,
					Op:       specs.OpMaskedEqual,
				},
			},
		},
		{
			Names:  []string{"mknodat"},
			Action: specs.ActErrno,
			Args: []specs.LinuxSeccompArg{
				{
					Index:    2,
					Value:    syscall.S_IFCHR,
					ValueTwo: syscall.S_IFCHR,
					Op:       specs.OpMaskedEqual,
				},
				{
					Index:    3,
					Value:    0,
					ValueTwo: 0,
					Op:       specs.OpNotEqual,
				},
			},
		},
	}

	lseccomp := &specs.LinuxSeccomp{
		DefaultAction: specs.ActAllow,
		Syscalls:      syscalls,
	}

	// assume 32-bit mode compatibility with 64-bit architectures
	switch runtime.GOARCH {
	case "amd64":
		lseccomp.Architectures = []specs.Arch{specs.ArchX86_64, specs.ArchX86, specs.ArchX32}
	case "arm64":
		lseccomp.Architectures = []specs.Arch{specs.ArchAARCH64, specs.ArchARM}
	case "mips64":
		lseccomp.Architectures = []specs.Arch{specs.ArchMIPS64, specs.ArchMIPS, specs.ArchMIPS64N32}
	case "mips64le":
		lseccomp.Architectures = []specs.Arch{specs.ArchMIPSEL64, specs.ArchMIPSEL, specs.ArchMIPSEL64N32}
	case "ppc64":
		lseccomp.Architectures = []specs.Arch{specs.ArchPPC64, specs.ArchPPC}
	case "s390x":
		lseccomp.Architectures = []specs.Arch{specs.ArchS390X, specs.ArchS390}
	case "riscv64":
		lseccomp.Architectures = []specs.Arch{specs.ArchRISCV64}
	}

	return lseccomp
}

// StartProcess is called during stage2 after RPC server finished
// environment preparation. This is the container process itself.
// It will execute command in the fakeroot context.
//
// This will be executed as a fake root user in a new user
// namespace (PrepareConfig will set both).
func (e *EngineOperations) StartProcess(_ int) error {
	const (
		mountInfo    = "/proc/self/mountinfo"
		binfmtMisc   = "/proc/sys/fs/binfmt_misc"
		selinuxMount = "/sys/fs/selinux"
	)

	if e.EngineConfig == nil {
		return fmt.Errorf("bad fakeroot engine configuration provided")
	}

	args := e.EngineConfig.Args
	if len(args) == 0 {
		return fmt.Errorf("no command to execute provided")
	}
	env := e.EngineConfig.Envs

	// simple command execution
	if !e.EngineConfig.BuildEnv {
		return syscall.Exec(args[0], args, env)
	}

	// prepare fakeroot build environment
	if e.EngineConfig.Home == "" {
		return fmt.Errorf("a user home directory is required to bind it on top of /root directory")
	}

	// simple trick to bind user home directory on top of /root
	err := syscall.Mount(e.EngineConfig.Home, "/root", "", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		return fmt.Errorf("failed to mount %s to /root: %s", e.EngineConfig.Home, err)
	}
	tmpdir, err := os.MkdirTemp("", "bind-mount-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %s", err)
	}
	err = syscall.Mount("proc", tmpdir, "proc", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, "")
	if err != nil {
		return fmt.Errorf("failed to mount proc filesystem: %s", err)
	}
	err = syscall.Mount(binfmtMisc, filepath.Join(tmpdir, "sys", "fs", "binfmt_misc"), "", syscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("failed to mount binfmt_misc filesystem: %s", err)
	}
	err = syscall.Mount(tmpdir, "/proc", "", syscall.MS_MOVE, "")
	if err != nil {
		return fmt.Errorf("failed to mount proc filesystem: %s", err)
	}
	err = os.Remove(tmpdir)
	if err != nil {
		return fmt.Errorf("failed to remove temporary directory: %s", err)
	}

	// fix potential issue with SELinux (https://github.com/apptainer/singularity/issues/4038)
	mounts, err := proc.GetMountPointMap(mountInfo)
	if err != nil {
		return fmt.Errorf("while parsing %s: %s", mountInfo, err)
	}
	for _, m := range mounts["/sys"] {
		// In Linux <5.9 this is required so that in the chroot, selinux is seen as ro, i.e.
		// disabled, and errors getting security labels do not occur.
		// In 5.9 the remount will now fail, but it is not needed due to changes in label handling.
		if m == selinuxMount {
			flags := uintptr(syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_RDONLY)
			err = syscall.Mount("", selinuxMount, "", flags, "")
			if err != nil {
				sylog.Debugf("while remount %s read-only: %s", selinuxMount, err)
				sylog.Debugf("note %s remount failure is expected on kernel 5.9+", selinuxMount)
			}
			break
		}
	}

	if seccomp.Enabled() {
		if err := seccomp.LoadSeccompConfig(fakerootSeccompProfile(), false, 0); err != nil {
			sylog.Warningf("Could not apply seccomp filter due to error (%s). Some bootstrap may not work correctly", err)
		}
	} else {
		sylog.Warningf("Not compiled with seccomp, fakeroot may not work correctly, " +
			"if you get permission denied error during creation of pseudo devices, " +
			"you should install seccomp library and recompile Apptainer")
	}
	return syscall.Exec(args[0], args, env)
}

// MonitorContainer is called from master once the container has
// been spawned. It will block until the container exists.
//
// Additional privileges may be gained when running hybrid flow.
//
// Particularly here no additional privileges are gained as monitor does
// not need them for wait4 and kill syscalls.
func (e *EngineOperations) MonitorContainer(pid int, signals chan os.Signal) (syscall.WaitStatus, error) {
	var status syscall.WaitStatus
	waitStatus := make(chan syscall.WaitStatus, 1)
	waitError := make(chan error, 1)

	go func() {
		sylog.Debugf("Waiting for container process %d", pid)
		_, err := syscall.Wait4(pid, &status, 0, nil)
		sylog.Debugf("Wait for process %d complete with status %v, error %v", pid, status, err)
		waitStatus <- status
		waitError <- err
	}()

	for {
		select {
		case s := <-signals:
			// Signal received
			switch s {
			case syscall.SIGCHLD:
				// Our go routine waiting for the container pid will handle container exit.
				break
			case syscall.SIGURG:
				// Ignore SIGURG, which is used for non-cooperative goroutine
				// preemption starting with Go 1.14. For more information, see
				// https://github.com/golang/go/issues/24543.
				break
			default:
				if err := syscall.Kill(pid, s.(syscall.Signal)); err != nil {
					return status, fmt.Errorf("interrupted by signal %s", s.String())
				}
			}
		case ws := <-waitStatus:
			// Container process exited
			we := <-waitError
			if we != nil {
				return ws, fmt.Errorf("error while waiting for child: %w", we)
			}
			return ws, we
		}
	}
}

// CleanupContainer does nothing for the fakeroot engine.
func (e *EngineOperations) CleanupContainer(context.Context, error, syscall.WaitStatus) error {
	// close the connection between apptainer and apptheus
	if e.CommonConfig.ApptheusSocket != nil {
		if err := e.CommonConfig.ApptheusSocket.Close(); err != nil {
			sylog.Warningf("failed to close the aptainer connection with apptheus: %v", err)
		}
	}
	return nil
}

// PostStartProcess does nothing for the fakeroot engine.
func (e *EngineOperations) PostStartProcess(_ context.Context, _ int) error {
	return nil
}

func init() {
	engine.RegisterOperations(
		fakerootConfig.Name,
		&EngineOperations{
			EngineConfig: &fakerootConfig.EngineConfig{},
		},
	)
}
