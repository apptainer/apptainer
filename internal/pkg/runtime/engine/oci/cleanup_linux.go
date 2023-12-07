// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oci

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/instance"
	"github.com/apptainer/apptainer/pkg/ociruntime"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// CleanupContainer is called from master after the MonitorContainer returns.
// It is responsible for ensuring that the container has been properly torn down.
//
// Additional privileges may be gained when running
// in suid flow. However, when a user namespace is requested and it is not
// a hybrid workflow (e.g. fakeroot), then there is no privileged saved uid
// and thus no additional privileges can be gained.
//
// Specifically in oci engine, no additional privileges are gained here. However,
// most likely this still will be executed as root since `apptainer oci`
// command set requires privileged execution.
func (e *EngineOperations) CleanupContainer(ctx context.Context, fatal error, status syscall.WaitStatus) error {
	// close the connection between apptainer and apptheus
	if e.CommonConfig.ApptheusSocket != nil {
		if err := e.CommonConfig.ApptheusSocket.Close(); err != nil {
			sylog.Warningf("failed to close the aptainer connection with apptheus: %v", err)
		}
	}

	if e.EngineConfig.Cgroups != nil {
		if err := e.EngineConfig.Cgroups.Destroy(); err != nil {
			sylog.Warningf("failed to remove cgroup configuration: %v", err)
		}
	}

	pidFile := e.EngineConfig.GetPidFile()
	if pidFile != "" {
		os.Remove(pidFile)
	}

	// if container wasn't created, delete instance files
	if e.EngineConfig.State.Status == ociruntime.Creating {
		name := e.CommonConfig.ContainerID
		file, err := instance.Get(name, instance.OciSubDir)
		if err != nil {
			sylog.Warningf("no instance files found for %s: %s", name, err)
			return nil
		}
		if err := file.Delete(); err != nil {
			sylog.Warningf("failed to delete instance files: %s", err)
		}
		return nil
	}

	exitCode := 0
	desc := ""

	if fatal != nil {
		exitCode = 255
		desc = fatal.Error()
	} else if status.Signaled() {
		s := status.Signal()
		exitCode = int(s) + 128
		desc = fmt.Sprintf("interrupted by signal %s", s.String())
	} else {
		exitCode = status.ExitStatus()
		desc = fmt.Sprintf("exited with code %d", status.ExitStatus())
	}

	e.EngineConfig.State.ExitCode = &exitCode
	e.EngineConfig.State.ExitDesc = desc

	if err := e.updateState(ociruntime.Stopped); err != nil {
		return err
	}

	if e.EngineConfig.State.AttachSocket != "" {
		os.Remove(e.EngineConfig.State.AttachSocket)
	}
	if e.EngineConfig.State.ControlSocket != "" {
		os.Remove(e.EngineConfig.State.ControlSocket)
	}

	return nil
}
