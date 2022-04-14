// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package squashfuse

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
)

const driverName = "squashfuse"

type squashfuseDriver struct {
	cmd     *exec.Cmd
	cmdpath string
}

func Init(register bool, unprivileged bool, fileconf *apptainerconf.File) error {
	if fileconf.ImageDriver != "" && fileconf.ImageDriver != driverName {
		sylog.Debugf("skipping installing %v image driver because %v already configured", driverName, fileconf.ImageDriver)
		// allow a configured driver to take precedence
		return nil
	}
	if !unprivileged {
		// no need for this driver if running privileged
		if fileconf.ImageDriver == driverName {
			// must have been incorrectly thought to be unprivileged
			// at an earlier point (e.g. TestLibraryPacker unit-test)
			fileconf.ImageDriver = ""
		}
		return nil
	}
	squashpath, err := bin.FindBin("squashfuse")
	if err != nil {
		if register {
			sylog.Debugf("skipping registering %v driver because: %v", driverName, err)
		} else {
			// this only happens once
			sylog.Infof("no squashfuse found, will not be able to mount SIF")
		}
		return nil
	}
	sylog.Debugf("Setting ImageDriver to %v", driverName)
	fileconf.ImageDriver = driverName
	if !register {
		return nil
	}
	sylog.Debugf("Registering Driver %v", driverName)
	return image.RegisterDriver(driverName, &squashfuseDriver{nil, squashpath})
}

func (d *squashfuseDriver) Features() image.DriverFeature {
	return image.ImageFeature
}

func (d *squashfuseDriver) Mount(params *image.MountParams, _ image.MountFunc) error {
	optsStr := "offset=" + strconv.FormatUint(params.Offset, 10)
	d.cmd = exec.Command(d.cmdpath, "-f", "-o", optsStr, params.Source, params.Target)
	sylog.Debugf("Executing %v", d.cmd.String())
	var stderr bytes.Buffer
	d.cmd.Stderr = &stderr
	if path.Dir(params.Source) == "/proc/self/fd" {
		d.cmd.ExtraFiles = make([]*os.File, 1)
		targetFd, _ := strconv.Atoi(path.Base(params.Source))
		d.cmd.ExtraFiles[0] = os.NewFile(uintptr(targetFd), params.Source)
	}
	d.cmd.SysProcAttr = &syscall.SysProcAttr{
		AmbientCaps: []uintptr{
			uintptr(capabilities.Map["CAP_SYS_ADMIN"].Value),
		},
	}
	var err error
	if err = d.cmd.Start(); err != nil {
		return fmt.Errorf("squashfuse Start failed: %v: %v", err, stderr.String())
	}
	process := d.cmd.Process
	if process == nil {
		return fmt.Errorf("no squashfuse process started")
	}
	maxTime := 2 * time.Second
	totTime := 0 * time.Second
	for totTime < maxTime {
		sleepTime := 25 * time.Millisecond
		time.Sleep(sleepTime)
		totTime += sleepTime
		err = process.Signal(os.Signal(syscall.Signal(0)))
		if err != nil {
			err := d.cmd.Wait()
			return fmt.Errorf("squashfuse failed: %v: %v", err, stderr.String())
		}
		entries, err := proc.GetMountInfoEntry("/proc/self/mountinfo")
		if err != nil {
			d.Stop()
			return fmt.Errorf("squashfuse failure to get mount info: %v", err)
		}
		for _, entry := range entries {
			if entry.Point == params.Target {
				sylog.Debugf("%v mounted in %v", params.Target, totTime)
				return nil
			}
		}
	}
	d.Stop()
	return fmt.Errorf("squashfuse failed to mount %v in %v", params.Target, maxTime)
}

func (d *squashfuseDriver) Start(params *image.DriverParams) error {
	return nil
}

func (d *squashfuseDriver) Stop() error {
	if d.cmd != nil {
		process := d.cmd.Process
		if process != nil {
			sylog.Debugf("Killing squashfuse")
			process.Kill()
		}
	}
	return nil
}
