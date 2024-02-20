// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Package bin provides access to external binaries
package bin

import (
	"fmt"
)

// FindBin returns the path to the named binary, or an error if it is not found.
// We don't list any default because we want a deliberate decision about whether
// to use the SuidBinaryPath which is more restrictive when in the suid flow.
func FindBin(name string) (path string, err error) {
	switch name {
	// Basic system executables that we assume are always on PATH
	// We will search for these only in default PATH when in the suid flow
	case "cp",
		"dd",
		"mkfs.ext3",
		"mknod",
		"mount",
		"nsenter",
		"rm",
		"stdbuf",
		"true",
		"truncate",
		"uname":
		return findOnPath(name, true)
	// Executables that might be run privileged from the suid flow
	// We must not search the user's PATH when in the suid flow with these
	case "cryptsetup":
		return findOnPath(name, true)
	// ldconfig is special on Ubuntu: "ldconfig" is a wrapper around
	// "ldconfig.real" and the latter is the one we want, since the wrapper
	// interacts may drop capabilities. So try "ldconfig.real" first.
	case "ldconfig":
		path, err = findOnPath("ldconfig.real", false)
		if err == nil {
			return path, err
		}
		return findOnPath("ldconfig", false)
	// All other executables
	// We will always search the user's PATH first for these
	case "curl",
		"debootstrap",
		"dnf",
		"fakeroot",
		"fakeroot-sysv",
		"fuse-overlayfs",
		"fuse2fs",
		"go",
		"mksquashfs",
		"newgidmap",
		"newuidmap",
		"nvidia-container-cli",
		"pacstrap",
		"rpm",
		"rpmkeys",
		"squashfuse",
		"squashfuse_ll",
		"SUSEConnect",
		"unsquashfs",
		"yum",
		"zypper",
		"gocryptfs":
		return findOnPath(name, false)
	}
	return "", fmt.Errorf("unknown executable name %q", name)
}
