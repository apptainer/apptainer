// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package driver

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
)

const driverName = "fuseapps"

type fuseappsFeature struct {
	binName string
	cmd     *exec.Cmd
	cmdPath string
}

type fuseappsDriver struct {
	squashFeature  fuseappsFeature
	overlayFeature fuseappsFeature
}

func (f *fuseappsFeature) init(binName string, purpose string, desired image.DriverFeature) {
	var err error
	f.cmdPath, err = bin.FindBin(binName)
	if err != nil {
		sylog.Debugf("%v mounting not enabled because: %v", binName, err)
		if desired != 0 {
			sylog.Infof("%v not found, will not be able to %v", binName, purpose)
		}
	} else {
		f.binName = binName
	}
}

func InitImageDrivers(register bool, unprivileged bool, fileconf *apptainerconf.File, desiredFeatures image.DriverFeature) error {
	if fileconf.ImageDriver != "" && fileconf.ImageDriver != driverName {
		sylog.Debugf("skipping installing %v image driver because %v already configured", driverName, fileconf.ImageDriver)
		// allow a configured driver to take precedence
		return nil
	}
	if !unprivileged {
		// no need for these features if running privileged
		if fileconf.ImageDriver == driverName {
			// must have been incorrectly thought to be unprivileged
			// at an earlier point (e.g. TestLibraryPacker unit-test)
			fileconf.ImageDriver = ""
		}
		return nil
	}

	var squashFeature fuseappsFeature
	var overlayFeature fuseappsFeature
	squashFeature.init("squashfuse", "mount SIF", desiredFeatures&image.ImageFeature)
	overlayFeature.init("fuse-overlayfs", "use overlay", desiredFeatures&image.OverlayFeature)

	if squashFeature.cmdPath != "" || overlayFeature.cmdPath != "" {
		sylog.Debugf("Setting ImageDriver to %v", driverName)
		fileconf.ImageDriver = driverName
		if register {
			return image.RegisterDriver(driverName, &fuseappsDriver{squashFeature, overlayFeature})
		}
	}
	return nil
}

func (d *fuseappsDriver) Features() image.DriverFeature {
	var features image.DriverFeature
	if d.squashFeature.cmdPath != "" {
		features |= image.ImageFeature
	}
	if d.overlayFeature.cmdPath != "" {
		features |= image.OverlayFeature
	}
	return features
}

func (d *fuseappsDriver) Mount(params *image.MountParams, mfunc image.MountFunc) error {
	var f *fuseappsFeature
	switch params.Filesystem {
	case "overlay":
		f = &d.overlayFeature
		optsStr := strings.Join(params.FSOptions, ",")
		f.cmd = exec.Command(f.cmdPath, "-f", "-o", optsStr, params.Target)

	case "squashfs":
		f = &d.squashFeature
		optsStr := "offset=" + strconv.FormatUint(params.Offset, 10)
		srcPath := params.Source
		if path.Dir(params.Source) == "/proc/self/fd" {
			// this will be passed as the first ExtraFile below, always fd 3
			srcPath = "/proc/self/fd/3"
		}
		f.cmd = exec.Command(f.cmdPath, "-f", "-o", optsStr, srcPath, params.Target)

	case "ext3":
		return fmt.Errorf("mounting an EXT3 filesystem requires root or a suid installation")

	case "encryptfs":
		return fmt.Errorf("mounting an encrypted filesystem requires root or a suid installation")

	default:
		return fmt.Errorf("filesystem type %v not recognized by image driver", params.Filesystem)
	}

	sylog.Debugf("Executing %v", f.cmd.String())
	var stderr bytes.Buffer
	f.cmd.Stderr = &stderr
	if path.Dir(params.Source) == "/proc/self/fd" {
		f.cmd.ExtraFiles = make([]*os.File, 1)
		targetFd, _ := strconv.Atoi(path.Base(params.Source))
		f.cmd.ExtraFiles[0] = os.NewFile(uintptr(targetFd), params.Source)
	}
	f.cmd.SysProcAttr = &syscall.SysProcAttr{
		AmbientCaps: []uintptr{
			uintptr(capabilities.Map["CAP_SYS_ADMIN"].Value),
		},
	}
	var err error
	if err = f.cmd.Start(); err != nil {
		return fmt.Errorf("%v Start failed: %v: %v", f.binName, err, stderr.String())
	}
	process := f.cmd.Process
	if process == nil {
		return fmt.Errorf("no %v process started", f.binName)
	}
	maxTime := 2 * time.Second
	totTime := 0 * time.Second
	for totTime < maxTime {
		sleepTime := 25 * time.Millisecond
		time.Sleep(sleepTime)
		totTime += sleepTime
		err = process.Signal(os.Signal(syscall.Signal(0)))
		if err != nil {
			err := f.cmd.Wait()
			return fmt.Errorf("%v failed: %v: %v", f.binName, err, stderr.String())
		}
		entries, err := proc.GetMountInfoEntry("/proc/self/mountinfo")
		if err != nil {
			f.stop()
			return fmt.Errorf("%v failure to get mount info: %v", f.binName, err)
		}
		for _, entry := range entries {
			if entry.Point == params.Target {
				sylog.Debugf("%v mounted in %v", params.Target, totTime)
				return nil
			}
		}
	}
	f.stop()
	return fmt.Errorf("%v failed to mount %v in %v", f.binName, params.Target, maxTime)
}

func (d *fuseappsDriver) Start(params *image.DriverParams) error {
	return nil
}

func (f *fuseappsFeature) stop() {
	if f.cmd != nil {
		process := f.cmd.Process
		if process != nil {
			sylog.Debugf("Killing %v", f.binName)
			process.Kill()
		}
	}
}

func (d *fuseappsDriver) Stop() error {
	d.squashFeature.stop()
	d.overlayFeature.stop()
	return nil
}
