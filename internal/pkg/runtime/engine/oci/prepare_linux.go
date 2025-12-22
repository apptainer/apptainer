// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oci

import (
	"fmt"
	"os"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/cgroups"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/starter"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/ociruntime"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/ccoveille/go-safecast"
	"github.com/creack/pty"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// make master/slave as global variable to avoid GC close file descriptor
var (
	master *os.File
	slave  *os.File
)

// PrepareConfig is called during stage1 to validate and prepare
// container configuration. It is responsible for reading capabilities,
// checking what namespaces are required, opening streams for attach and
// exec, etc.
//
// No additional privileges can be gained as any of them are already
// dropped by the time PrepareConfig is called. However, most likely this
// still will be executed as root since `apptainer oci` command set
// requires privileged execution.
//
//nolint:maintidx
func (e *EngineOperations) PrepareConfig(starterConfig *starter.Config) error {
	if e.CommonConfig.EngineName != Name {
		return fmt.Errorf("incorrect engine")
	}

	if e.EngineConfig.OciConfig.Config != &e.EngineConfig.OciConfig.Spec {
		return fmt.Errorf("bad engine configuration provided")
	}

	if starterConfig.GetIsSUID() {
		return fmt.Errorf("suid workflow disabled by administrator")
	}

	if e.EngineConfig.OciConfig.Process == nil {
		return fmt.Errorf("empty OCI process configuration")
	}

	if e.EngineConfig.OciConfig.Linux == nil {
		return fmt.Errorf("empty OCI linux configuration")
	}

	// TODO - investigate whether this is the highest place to pull this value from apptainer.conf
	if !fs.IsOwner(buildcfg.APPTAINER_CONF_FILE, 0) {
		return fmt.Errorf("%s must be owned by root", buildcfg.APPTAINER_CONF_FILE)
	}
	sConf, err := apptainerconf.Parse(buildcfg.APPTAINER_CONF_FILE)
	if err != nil {
		return fmt.Errorf("unable to parse apptainer.conf file: %s", err)
	}
	e.EngineConfig.SystemdCgroups = sConf.SystemdCgroups

	// reset state config that could be passed to engine
	e.EngineConfig.State = ociruntime.State{}

	user := &e.EngineConfig.OciConfig.Process.User
	gids := make([]int, 0, len(user.AdditionalGids)+1)

	uid := int(user.UID)
	gid := user.GID

	gids = append(gids, int(gid))
	for _, g := range user.AdditionalGids {
		gids = append(gids, int(g))
	}

	starterConfig.SetTargetUID(uid)
	starterConfig.SetTargetGID(gids)

	if !e.EngineConfig.Exec {
		starterConfig.SetInstance(true)
	}

	userNS := false
	for _, ns := range e.EngineConfig.OciConfig.Linux.Namespaces {
		if ns.Type == specs.UserNamespace {
			userNS = true
			break
		}
	}

	starterConfig.SetNsFlagsFromSpec(e.EngineConfig.OciConfig.Linux.Namespaces)
	if err := starterConfig.SetNsPathFromSpec(e.EngineConfig.OciConfig.Linux.Namespaces); err != nil {
		return err
	}

	if userNS {
		if len(e.EngineConfig.OciConfig.Linux.UIDMappings) == 0 {
			return fmt.Errorf("user namespace invoked without uid mapping")
		}
		if len(e.EngineConfig.OciConfig.Linux.GIDMappings) == 0 {
			return fmt.Errorf("user namespace invoked without gid mapping")
		}
		if err := starterConfig.AddUIDMappings(e.EngineConfig.OciConfig.Linux.UIDMappings); err != nil {
			return err
		}
		if err := starterConfig.AddGIDMappings(e.EngineConfig.OciConfig.Linux.GIDMappings); err != nil {
			return err
		}
	}

	if e.EngineConfig.OciConfig.Linux.RootfsPropagation != "" {
		starterConfig.SetMountPropagation(e.EngineConfig.OciConfig.Linux.RootfsPropagation)
	} else {
		starterConfig.SetMountPropagation("private")
	}

	starterConfig.SetNoNewPrivs(e.EngineConfig.OciConfig.Process.NoNewPrivileges)

	if e.EngineConfig.OciConfig.Process.Capabilities != nil {
		if err := e.checkCapabilities(); err != nil {
			return err
		}

		// force cap_sys_admin for seccomp and no_new_priv flag
		caps := append(e.EngineConfig.OciConfig.Process.Capabilities.Effective, "CAP_SYS_ADMIN")
		starterConfig.SetCapabilities(capabilities.Effective, caps)

		caps = append(e.EngineConfig.OciConfig.Process.Capabilities.Permitted, "CAP_SYS_ADMIN")
		starterConfig.SetCapabilities(capabilities.Permitted, caps)

		starterConfig.SetCapabilities(capabilities.Inheritable, e.EngineConfig.OciConfig.Process.Capabilities.Inheritable)
		starterConfig.SetCapabilities(capabilities.Bounding, e.EngineConfig.OciConfig.Process.Capabilities.Bounding)
		starterConfig.SetCapabilities(capabilities.Ambient, e.EngineConfig.OciConfig.Process.Capabilities.Ambient)
	}

	e.EngineConfig.MasterPts = -1
	e.EngineConfig.SlavePts = -1
	e.EngineConfig.OutputStreams = [2]int{-1, -1}
	e.EngineConfig.ErrorStreams = [2]int{-1, -1}
	e.EngineConfig.InputStreams = [2]int{-1, -1}

	if e.EngineConfig.GetLogFormat() == "" {
		sylog.Debugf("No log format specified, setting kubernetes log format by default")
		e.EngineConfig.SetLogFormat("kubernetes")
	}

	if !e.EngineConfig.Exec {
		if e.EngineConfig.OciConfig.Process.Terminal {
			var err error

			master, slave, err = pty.Open()
			if err != nil {
				return err
			}
			consoleSize := e.EngineConfig.OciConfig.Process.ConsoleSize
			if consoleSize != nil {
				var size pty.Winsize

				size.Cols, err = safecast.Convert[uint16](consoleSize.Width)
				if err != nil {
					return err
				}
				size.Rows, err = safecast.Convert[uint16](consoleSize.Height)
				if err != nil {
					return err
				}

				if err := pty.Setsize(slave, &size); err != nil {
					return err
				}
			}
			e.EngineConfig.MasterPts = int(master.Fd())
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.MasterPts); err != nil {
				return err
			}
			e.EngineConfig.SlavePts = int(slave.Fd())
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.SlavePts); err != nil {
				return err
			}
		} else {
			r, w, err := os.Pipe()
			if err != nil {
				return err
			}
			e.EngineConfig.OutputStreams = [2]int{int(r.Fd()), int(w.Fd())}
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.OutputStreams[0]); err != nil {
				return err
			}
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.OutputStreams[1]); err != nil {
				return err
			}

			r, w, err = os.Pipe()
			if err != nil {
				return err
			}
			e.EngineConfig.ErrorStreams = [2]int{int(r.Fd()), int(w.Fd())}
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.ErrorStreams[0]); err != nil {
				return err
			}
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.ErrorStreams[1]); err != nil {
				return err
			}

			r, w, err = os.Pipe()
			if err != nil {
				return err
			}
			e.EngineConfig.InputStreams = [2]int{int(w.Fd()), int(r.Fd())}
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.InputStreams[0]); err != nil {
				return err
			}
			if err := starterConfig.KeepFileDescriptor(e.EngineConfig.InputStreams[1]); err != nil {
				return err
			}
		}
	} else {
		starterConfig.SetNamespaceJoinOnly(true)
		cPath := e.EngineConfig.OciConfig.Linux.CgroupsPath
		if cPath == "" {
			return nil
		}
		ppid := os.Getppid()

		sylog.Debugf("Adding process %d to instance cgroup %q", ppid, cPath)
		manager, err := cgroups.GetManagerForGroup(cPath)
		if err != nil {
			return fmt.Errorf("couldn't create cgroup manager: %v", err)
		}
		if err := manager.AddProc(ppid); err != nil {
			return fmt.Errorf("couldn't add process to instance cgroup: %v", err)
		}
	}

	return nil
}

func (e *EngineOperations) checkCapabilities() error {
	for _, capability := range e.EngineConfig.OciConfig.Process.Capabilities.Permitted {
		if _, ok := capabilities.Map[capability]; !ok {
			return fmt.Errorf("unrecognized capabilities %s", capability)
		}
	}
	for _, capability := range e.EngineConfig.OciConfig.Process.Capabilities.Effective {
		if _, ok := capabilities.Map[capability]; !ok {
			return fmt.Errorf("unrecognized capabilities %s", capability)
		}
	}
	for _, capability := range e.EngineConfig.OciConfig.Process.Capabilities.Inheritable {
		if _, ok := capabilities.Map[capability]; !ok {
			return fmt.Errorf("unrecognized capabilities %s", capability)
		}
	}
	for _, capability := range e.EngineConfig.OciConfig.Process.Capabilities.Bounding {
		if _, ok := capabilities.Map[capability]; !ok {
			return fmt.Errorf("unrecognized capabilities %s", capability)
		}
	}
	for _, capability := range e.EngineConfig.OciConfig.Process.Capabilities.Ambient {
		if _, ok := capabilities.Map[capability]; !ok {
			return fmt.Errorf("unrecognized capabilities %s", capability)
		}
	}
	return nil
}
