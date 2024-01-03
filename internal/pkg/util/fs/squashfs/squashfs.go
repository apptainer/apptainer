// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package squashfs

import (
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/syecl"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/namespaces"
)

func getConfig() (*apptainerconf.File, error) {
	// if the caller has set the current config use it
	// otherwise parse the default configuration file
	cfg := apptainerconf.GetCurrentConfig()
	if cfg == nil {
		sylog.Fatalf("configuration not pre-loaded in squashfs getConfig")
	}
	return cfg, nil
}

// GetPath figures out where the mksquashfs binary is
// and return an error is not available or not usable.
func GetPath() (string, error) {
	return bin.FindBin("mksquashfs")
}

func GetProcs() (uint, error) {
	c, err := getConfig()
	if err != nil {
		return 0, err
	}
	// proc is either "" or the string value in the conf file
	proc := c.MksquashfsProcs

	return proc, err
}

func GetMem() (string, error) {
	c, err := getConfig()
	if err != nil {
		return "", err
	}
	// mem is either "" or the string value in the conf file
	mem := c.MksquashfsMem

	return mem, err
}

var (
	setuidMountKnown   bool
	setuidMountAllowed bool
)

// SetuidMountAllowed calculates whether or not it is allowed to
// mount a squashfs filesystem using the kernel driver in setuid mode.
func SetuidMountAllowed(cfg *apptainerconf.File) bool {
	if setuidMountKnown {
		return setuidMountAllowed
	}
	setuidMountKnown = true
	str := cfg.AllowSetuidMountSquashfs
	if !namespaces.IsUnprivileged() {
		setuidMountAllowed = true
		sylog.Debugf("Kernel squashfs mount allowed because running as root")
	} else if str == "yes" {
		setuidMountAllowed = true
		sylog.Debugf("Kernel squashfs mount allowed by configuration")
	} else if str == "iflimited" {
		if len(cfg.LimitContainerOwners) > 0 ||
			len(cfg.LimitContainerGroups) > 0 ||
			len(cfg.LimitContainerPaths) > 0 {
			setuidMountAllowed = true
			sylog.Debugf("Kernel squashfs mount allowed because of limit container")
		} else {
			eclcfg, err := syecl.LoadConfig(buildcfg.ECL_FILE)
			if err != nil {
				sylog.Debugf("Kernel squashfs mount not allowed because error loading %s: %v", buildcfg.ECL_FILE, err)
			} else if eclcfg.Activated {
				setuidMountAllowed = true
				sylog.Debugf("Kernel squashfs mount allowed because of activated ECL")
			} else {
				sylog.Debugf("Kernel squashfs mount not allowed because ECL not activated")
			}
		}
	} else {
		sylog.Debugf("Kernel squashfs mount not allowed by configuration")
	}
	return setuidMountAllowed
}
