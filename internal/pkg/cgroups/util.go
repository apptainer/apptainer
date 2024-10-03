// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"fmt"
	"os"
	"strings"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"golang.org/x/sys/unix"
)

const unifiedMountPoint = "/sys/fs/cgroup"

// pidToPath returns the path of the cgroup containing process ID pid.
// It is assumed that for v1 cgroups the devices controller is in use.
func pidToPath(pid int) (path string, err error) {
	if pid == 0 {
		return "", fmt.Errorf("must provide a valid pid")
	}

	pidCGFile := fmt.Sprintf("/proc/%d/cgroup", pid)
	paths, err := cgroups.ParseCgroupFile(pidCGFile)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", pidCGFile, err)
	}

	// cgroups v2 path is always given by the unified "" subsystem
	ok := false
	if cgroups.IsCgroup2UnifiedMode() {
		path, ok := paths[""]
		if !ok {
			return "", fmt.Errorf("could not find cgroups v2 unified path")
		}
		return path, nil
	}

	// For cgroups v1 we are relying on fetching the 'devices' subsystem path.
	// The devices subsystem is needed for our OCI engine and its presence is
	// enforced in runc/libcontainer/cgroups/fs initialization without 'skipDevices'.
	// This means we never explicitly put a container into a cgroup without a
	// set 'devices' path.
	path, ok = paths["devices"]
	if !ok {
		return "", fmt.Errorf("could not find cgroups v1 path (using devices subsystem)")
	}
	return path, nil
}

// HasDbus checks if DBUS_SESSION_BUS_ADDRESS is set, and sane.
// Logs unset var / non-existent target at DEBUG level.
func HasDbus() (bool, error) {
	dbusEnv := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if dbusEnv == "" {
		return false, fmt.Errorf("DBUS_SESSION_BUS_ADDRESS is not set")
	}

	if !strings.HasPrefix(dbusEnv, "unix:") {
		return false, fmt.Errorf("DBUS_SESSION_BUS_ADDRESS %q is not a 'unix:' socket", dbusEnv)
	}

	return true, nil
}

// HasXDGRuntimeDir checks if XDG_Runtime_Dir is set, and sane.
// Logs unset var / non-existent target at DEBUG level.
func HasXDGRuntimeDir() (bool, error) {
	xdgRuntimeEnv := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeEnv == "" {
		return false, fmt.Errorf("XDG_RUNTIME_DIR is not set")
	}

	fi, err := os.Stat(xdgRuntimeEnv)
	if err != nil {
		return false, fmt.Errorf("XDG_RUNTIME_DIR %q not accessible: %v", xdgRuntimeEnv, err)
	}

	if !fi.IsDir() {
		return false, fmt.Errorf("XDG_RUNTIME_DIR %q is not a directory", xdgRuntimeEnv)
	}

	if err := unix.Access(xdgRuntimeEnv, unix.W_OK); err != nil {
		return false, fmt.Errorf("XDG_RUNTIME_DIR %q is not writable", xdgRuntimeEnv)
	}

	return true, nil
}

// CanUseCgroups checks whether it's possible to use the cgroups manager.
// - Host root can always use cgroups.
// - Rootless needs cgroups v2.
// - Rootless needs systemd manager.
// - Rootless needs DBUS_SESSION_BUS_ADDRESS and XDG_RUNTIME_DIR set properly.
// warn controls whether configuration problems preventing use of cgroups will be logged as warnings, or debug messages.
func CanUseCgroups(systemd bool, warn bool) bool {
	uid := os.Geteuid()
	if uid == 0 {
		return true
	}

	rootlessOK := true

	if !cgroups.IsCgroup2UnifiedMode() {
		rootlessOK = false
		if warn {
			sylog.Warningf("Rootless cgroups require the system to be configured for cgroups v2 in unified mode.")
		} else {
			sylog.Debugf("Rootless cgroups require 'systemd cgroups' to be enabled in apptainer.conf")
		}
	}

	if !systemd {
		rootlessOK = false
		if warn {
			sylog.Warningf("Rootless cgroups require 'systemd cgroups' to be enabled in apptainer.conf")
		} else {
			sylog.Debugf("Rootless cgroups require 'systemd cgroups' to be enabled in apptainer.conf")
		}
	}

	if ok, err := HasXDGRuntimeDir(); !ok {
		rootlessOK = false
		if warn {
			sylog.Warningf("%s", err)
		} else {
			sylog.Debugf("%s", err)
		}
	}

	if ok, err := HasDbus(); !ok {
		rootlessOK = false
		if warn {
			sylog.Warningf("%s", err)
		} else {
			sylog.Debugf("%s", err)
		}
	}

	return rootlessOK
}
