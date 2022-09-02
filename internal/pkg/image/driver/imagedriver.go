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
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
)

const driverName = "fuseapps"

type fuseappsInstance struct {
	cmd    *exec.Cmd
	params *image.MountParams
}

type fuseappsFeature struct {
	binName   string
	cmdPath   string
	instances []fuseappsInstance
}

type fuseappsDriver struct {
	squashFeature  fuseappsFeature
	ext3Feature    fuseappsFeature
	overlayFeature fuseappsFeature
}

func (f *fuseappsFeature) init(binNames string, purpose string, desired image.DriverFeature) {
	var err error
	for _, binName := range strings.Split(binNames, "|") {
		f.binName = binName
		f.cmdPath, err = bin.FindBin(binName)
		if err == nil {
			break
		}
	}
	if err != nil {
		sylog.Debugf("%v mounting not enabled because: %v", f.binName, err)
		if desired != 0 {
			sylog.Infof("%v not found, will not be able to %v", f.binName, purpose)
		}
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
	var ext3Feature fuseappsFeature
	var overlayFeature fuseappsFeature
	squashFeature.init("squashfuse_ll|squashfuse", "mount SIF", desiredFeatures&image.ImageFeature)
	ext3Feature.init("fuse2fs", "mount EXT3 filesystems", desiredFeatures&image.ImageFeature)
	overlayFeature.init("fuse-overlayfs", "use overlay", desiredFeatures&image.OverlayFeature)

	if squashFeature.cmdPath != "" || ext3Feature.cmdPath != "" || overlayFeature.cmdPath != "" {
		sylog.Debugf("Setting ImageDriver to %v", driverName)
		fileconf.ImageDriver = driverName
		if register {
			return image.RegisterDriver(driverName, &fuseappsDriver{squashFeature, ext3Feature, overlayFeature})
		}
	}
	return nil
}

func (d *fuseappsDriver) Features() image.DriverFeature {
	var features image.DriverFeature
	if d.squashFeature.cmdPath != "" || d.ext3Feature.cmdPath != "" {
		features |= image.ImageFeature
	}
	if d.overlayFeature.cmdPath != "" {
		features |= image.OverlayFeature
	}
	return features
}

//nolint:maintidx
func (d *fuseappsDriver) Mount(params *image.MountParams, mfunc image.MountFunc) error {
	var f *fuseappsFeature
	var cmd *exec.Cmd
	switch params.Filesystem {
	case "overlay":
		f = &d.overlayFeature
		optsStr := strings.Join(params.FSOptions, ",")
		cmd = exec.Command(f.cmdPath, "-f", "-o", optsStr, params.Target)

	case "squashfs":
		f = &d.squashFeature
		optsStr := ""
		// Unfortunately squashfuse_ll does not currently support
		//  setting the uid/gid options, so need to skip those with
		//  that.  It makes setuid-flow fakeroot overlay unwritable
		//  because the default root ownership gets mapped to 65534
		//  inside the container.
		if f.binName == "squashfuse" {
			optsStr = fmt.Sprintf("uid=%v,gid=%v", os.Getuid(), os.Getgid())
		}
		if params.Offset > 0 {
			if optsStr != "" {
				optsStr += ","
			}
			optsStr += "offset=" + strconv.FormatUint(params.Offset, 10)
		}
		srcPath := params.Source
		if path.Dir(params.Source) == "/proc/self/fd" {
			// this will be passed as the first ExtraFile below, always fd 3
			srcPath = "/proc/self/fd/3"
		}
		cmd = exec.Command(f.cmdPath, "-f", "-o", optsStr, srcPath, params.Target)

	case "ext3":
		f = &d.ext3Feature
		srcPath := params.Source
		if path.Dir(params.Source) == "/proc/self/fd" {
			// this will be passed as the first ExtraFile below, always fd 3
			srcPath = "/proc/self/fd/3"
		}
		optsStr := ""
		if os.Getuid() != 0 {
			// Bypass permission checks so all can be read,
			//  especially overlay work dir
			optsStr = "fakeroot"
		}
		if (params.Flags & syscall.MS_RDONLY) != 0 {
			if optsStr != "" {
				optsStr += ","
			}
			optsStr += "ro"
		}

		var cmdArgs []string
		stdbuf, err := bin.FindBin("stdbuf")
		if err == nil {
			// Run fuse2fs through stdbuf to be able to read the
			//  warnings sometimes sent through stdout
			cmdArgs = []string{stdbuf, "-oL"}
		}
		cmdArgs = append(cmdArgs, f.cmdPath, "-f", "-o", optsStr, srcPath, params.Target)
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)

		if params.Offset > 0 {
			// fuse2fs cannot natively offset into a file,
			//  so load a preload wrapper
			cmd.Env = []string{
				"LD_PRELOAD=" + buildcfg.LIBEXECDIR + "/apptainer/lib/offsetpreload.so",
				"OFFSETPRELOAD_FILE=" + srcPath,
				"OFFSETPRELOAD_OFFSET=" + strconv.FormatUint(params.Offset, 10),
			}
			for _, e := range cmd.Env {
				sylog.Debugf("Setting env %s", e)
			}
		}

	case "encryptfs":
		return fmt.Errorf("mounting an encrypted filesystem requires root or a suid installation")

	default:
		return fmt.Errorf("filesystem type %v not recognized by image driver", params.Filesystem)
	}

	if f.cmdPath == "" {
		return fmt.Errorf("%v not found", f.binName)
	}

	sylog.Debugf("Executing %v", cmd.String())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if path.Dir(params.Source) == "/proc/self/fd" {
		cmd.ExtraFiles = make([]*os.File, 1)
		targetFd, _ := strconv.Atoi(path.Base(params.Source))
		cmd.ExtraFiles[0] = os.NewFile(uintptr(targetFd), params.Source)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		AmbientCaps: []uintptr{
			uintptr(capabilities.Map["CAP_SYS_ADMIN"].Value),
		},
	}
	var err error
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("%v Start failed: %v: %v", f.binName, err, stderr.String())
	}
	process := cmd.Process
	if process == nil {
		return fmt.Errorf("no %v process started", f.binName)
	}

	ignoreMsgs := []string{
		// from fuse2fs
		"journal is not supported.",
		"Mounting read-only.",
		// from squashfuse_ll
		"failed to clone device fd",
		"continue without -o clone_fd",
	}
	filterMsg := func() string {
		var errstr string
		for idx, fd := range []bytes.Buffer{stdout, stderr} {
			str := fd.String()
			for _, line := range strings.Split(str, "\n") {
				if len(line) == 0 {
					continue
				}
				skip := false
				for _, ignoreMsg := range ignoreMsgs {
					if strings.Contains(line, ignoreMsg) {
						// skip these unhelpful messages
						skip = true
						break
					}
				}
				if skip {
					sylog.Debugf("%v", line)
				} else if idx == 0 {
					sylog.Infof("%v\n", line)
				} else {
					errstr += line + "\n"
				}
			}
		}
		return errstr
	}

	f.instances = append(f.instances, fuseappsInstance{cmd, params})
	maxTime := 2 * time.Second
	totTime := 0 * time.Second
	for totTime < maxTime {
		sleepTime := 25 * time.Millisecond
		time.Sleep(sleepTime)
		totTime += sleepTime
		var ws syscall.WaitStatus
		wpid, err := syscall.Wait4(process.Pid, &ws, syscall.WNOHANG, nil)
		if err != nil {
			return fmt.Errorf("unable to get wait status on %v: %v: %v", f.binName, err, filterMsg())
		}
		if wpid != 0 {
			return fmt.Errorf("%v exited with status %v: %v", f.binName, ws.ExitStatus(), filterMsg())
		}
		// See if mount has succeeded
		entries, err := proc.GetMountInfoEntry("/proc/self/mountinfo")
		if err != nil {
			f.stop(params.Target, true)
			return fmt.Errorf("%v failure to get mount info: %v", f.binName, err)
		}
		for _, entry := range entries {
			if entry.Point != params.Target {
				continue
			}
			msg := filterMsg()
			if len(msg) > 0 {
				// Haven't seen this happen, but just in case
				sylog.Infof("%v", msg)
			}
			sylog.Debugf("%v mounted in %v", params.Target, totTime)
			if params.Filesystem == "overlay" && os.Getuid() == 0 {
				// Look for unexpectedly readonly overlay
				hasUpper := false
				for _, opt := range params.FSOptions {
					if strings.HasPrefix(opt, "upperdir=") {
						hasUpper = true
					}
				}
				if !hasUpper {
					// No upperdir means readonly expected
					return nil
				}
				// Using unix.Access is not sufficient here
				// so have to attempt to create a file
				binpath := params.Target + "/usr/bin"
				tmpfile, err := ioutil.TempFile(binpath, ".tmp*")
				if err != nil {
					sylog.Debugf("%v not writable: %v", binpath, err)
					sylog.Infof("/usr/bin not writable in container")
					sylog.Infof("Consider using a different overlay upper layer filesystem type")
				} else {
					sylog.Debugf("successfully created %v", tmpfile.Name())
					tmpfile.Close()
					os.Remove(tmpfile.Name())
				}
			}
			return nil
		}
	}
	f.stop(params.Target, true)
	return fmt.Errorf("%v failed to mount %v in %v: %v", f.binName, params.Target, maxTime, stderr.String())
}

func (d *fuseappsDriver) Start(params *image.DriverParams) error {
	return nil
}

// Stop the process associated with the mount target, if there is one.
// If kill is not true, an unmount should already have happened so at
//   first just wait for the process to exit.
func (f *fuseappsFeature) stop(target string, kill bool) error {
	for _, instance := range f.instances {
		if instance.params.Target != target {
			continue
		}
		process := instance.cmd.Process
		var ws syscall.WaitStatus
		sylog.Debugf("Waiting for %v pid %v to exit", f.binName, process.Pid)
		// maxTime is total time to wait including after kill signal,
		//   and kill signal is sent at half the time
		maxTime := 1 * time.Second
		totTime := 0 * time.Second
		killed := false
		for totTime < maxTime {
			wpid, err := syscall.Wait4(process.Pid, &ws, syscall.WNOHANG, nil)
			if err != nil {
				sylog.Debugf("Waiting for %v pid %v failed: %v", f.binName, process.Pid, err)
				if err == syscall.ECHILD {
					// not a terrible problem when stopping
					return nil
				}
				return err
			} else if wpid != 0 {
				sylog.Debugf("%v pid %v exited with status %v within %v", f.binName, wpid, ws.ExitStatus(), totTime)
				return nil
			}
			if kill {
				sylog.Debugf("Killing pid %v", process.Pid)
			} else if !killed && totTime >= maxTime/2 {
				sylog.Debugf("Took more than %v, killing", maxTime/2)
				kill = true
			}
			if kill {
				kill = false
				killed = true
				process.Kill()
				continue
			}
			sleepTime := 10 * time.Millisecond
			time.Sleep(sleepTime)
			totTime += sleepTime
		}
		// This is unexpected, because the kill signal at half
		//  of maxTime should kill quickly
		return fmt.Errorf("took more than %v to stop %v pid %v", maxTime, f.binName, process.Pid)
	}
	return nil
}

func (d *fuseappsDriver) Stop(target string) error {
	var err error
	if err = d.squashFeature.stop(target, false); err != nil {
		return err
	}
	if err = d.ext3Feature.stop(target, false); err != nil {
		return err
	}
	if err = d.overlayFeature.stop(target, false); err != nil {
		return err
	}
	return nil
}
