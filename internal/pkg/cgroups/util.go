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

	lccgroups "github.com/opencontainers/runc/libcontainer/cgroups"
)

const unifiedMountPoint = "/sys/fs/cgroup"

// pidToPath returns the path of the cgroup containing process ID pid.
// It is assumed that for v1 cgroups the devices controller is in use.
func pidToPath(pid int) (path string, err error) {
	if pid == 0 {
		return "", fmt.Errorf("must provide a valid pid")
	}

	pidCGFile := fmt.Sprintf("/proc/%d/cgroup", pid)
	paths, err := lccgroups.ParseCgroupFile(pidCGFile)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", pidCGFile, err)
	}

	// cgroups v2 path is always given by the unified "" subsystem
	ok := false
	if lccgroups.IsCgroup2UnifiedMode() {
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
