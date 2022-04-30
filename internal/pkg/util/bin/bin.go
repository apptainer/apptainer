// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// Package bin provides access to external binaries
package bin

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/pkg/errors"
)

// FindBin returns the path to the named binary, or an error if it is not found.
func FindBin(name string) (path string, err error) {
	switch name {
	// Basic system executables that we assume are always on PATH
	case "true", "mkfs.ext3", "cp", "rm", "dd":
		return findOnPath(name)
	// Bootstrap related executables that we assume are on PATH
	case "mount", "mknod", "debootstrap", "pacstrap", "dnf", "yum", "rpm", "curl", "uname", "zypper", "SUSEConnect", "rpmkeys", "squashfuse", "fuse-overlayfs":
		return findOnPath(name)
	// Configurable executables that are found at build time, can be overridden
	// in apptainer.conf. If config value is "" will look on PATH.
	case "unsquashfs", "mksquashfs", "go", "cryptsetup", "ldconfig", "nvidia-container-cli":
		return findFromConfigOrPath(name)
	// distro provided setUID executables that are used in the fakeroot flow to setup subuid/subgid mappings
	case "newuidmap", "newgidmap":
		return findOnPath(name)
	}
	return "", fmt.Errorf("unknown executable name %q", name)
}

// findOnPath performs a search on the configurated binary path for the
// named executable, returning its full path.
func findOnPath(name string) (path string, err error) {
	cfg := apptainerconf.GetCurrentConfig()
	if cfg == nil {
		if strings.HasSuffix(os.Args[0], ".test") {
			// read config if doing unit tests
			cfg, err = apptainerconf.Parse(buildcfg.APPTAINER_CONF_FILE)
			if err != nil {
				return "", errors.Wrap(err, "unable to parse apptainer configuration file")
			}
			apptainerconf.SetCurrentConfig(cfg)
		} else {
			sylog.Fatalf("configuration not pre-loaded in findOnPath")
		}
	}
	newPath := cfg.BinaryPath
	if strings.Contains(newPath, "$PATH:") {
		if strings.HasSuffix(os.Args[0], ".test") {
			apptainerconf.SetBinaryPath(true)
		} else {
			sylog.Fatalf("SetBinaryPath has not been run before findOnPath")
		}
	}
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	os.Setenv("PATH", newPath)

	path, err = exec.LookPath(name)
	if err != nil {
		sylog.Debugf("Found %q at %q", name, path)
	}
	return path, err
}

// findFromConfigOrPath retrieves the path to an executable from apptainer.conf,
// or searches PATH if not set there.
func findFromConfigOrPath(name string) (path string, err error) {
	cfg := apptainerconf.GetCurrentConfig()
	if cfg == nil {
		if strings.HasSuffix(os.Args[0], ".test") {
			// read config if doing unit tests
			cfg, err = apptainerconf.Parse(buildcfg.APPTAINER_CONF_FILE)
			if err != nil {
				return "", errors.Wrap(err, "unable to parse apptainer configuration file")
			}
			apptainerconf.SetCurrentConfig(cfg)
		} else {
			sylog.Fatalf("configuration not pre-loaded in findFromConfigOrPath")
		}
	}

	switch name {
	case "go":
		path = cfg.GoPath
	case "mksquashfs":
		path = cfg.MksquashfsPath
	case "unsquashfs":
		path = cfg.UnsquashfsPath
	case "cryptsetup":
		path = cfg.CryptsetupPath
	case "ldconfig":
		path = cfg.LdconfigPath
	case "nvidia-container-cli":
		path = cfg.NvidiaContainerCliPath
	default:
		return "", fmt.Errorf("unknown executable name %q", name)
	}

	if path == "" {
		return findOnPath(name)
	}

	sylog.Debugf("Using %q at %q (from apptainer.conf)", name, path)

	// Use lookPath with the absolute path to confirm it is accessible & executable
	return exec.LookPath(path)
}
