// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/pkg/image"
	pluginapi "github.com/apptainer/apptainer/pkg/plugin"
	apptainercallback "github.com/apptainer/apptainer/pkg/plugin/callback/runtime/engine/apptainer"
)

// Plugin allows usage of overlay with user namespace on Ubuntu flavors.
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "example.com/ubuntu-userns-overlay-plugin",
		Author:      "Apptainer Team",
		Version:     "0.1.0",
		Description: "Overlay ubuntu driver with user namespace",
	},
	Callbacks: []pluginapi.Callback{
		(apptainercallback.RegisterImageDriver)(ubuntuOvlRegister),
	},
	Install: setConfiguration,
}

const driverName = "ubuntu-userns-overlay"

type ubuntuOvlDriver struct {
	unprivileged bool
}

func ubuntuOvlRegister(unprivileged bool) error {
	return image.RegisterDriver(driverName, &ubuntuOvlDriver{unprivileged})
}

func (d *ubuntuOvlDriver) Features() image.DriverFeature {
	// if we are running unprivileged we are handling the overlay mount
	if d.unprivileged {
		return image.OverlayFeature
	}
	// privileged run are handled as usual by the apptainer runtime
	return 0
}

func (d *ubuntuOvlDriver) Mount(params *image.MountParams, fn image.MountFunc) error {
	return fn(
		params.Source,
		params.Target,
		params.Filesystem,
		params.Flags,
		strings.Join(params.FSOptions, ","),
	)
}

func (d *ubuntuOvlDriver) Start(params *image.DriverParams, containerPid int) error {
	return nil
}

func (d *ubuntuOvlDriver) Stop(target string) error {
	return nil
}

func (d *ubuntuOvlDriver) Stopped(int, syscall.WaitStatus) error {
	return nil
}

// setConfiguration sets "image driver" and "enable overlay" configuration directives
// during apptainer plugin install step.
func setConfiguration(_ string) error {
	cmd := exec.Command("/proc/self/exe", "config", "global", "--set", "image driver", driverName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not set 'image driver = %s' in apptainer.conf", driverName)
	}
	cmd = exec.Command("/proc/self/exe", "config", "global", "--set", "enable overlay", "driver")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not set 'enable overlay = driver' in apptainer.conf")
	}
	return nil
}
