// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package driver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/internal/pkg/util/fs/squashfs"
	"github.com/apptainer/apptainer/pkg/image"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
	"github.com/apptainer/apptainer/pkg/util/capabilities"
	"github.com/apptainer/apptainer/pkg/util/fs/proc"
	"golang.org/x/sys/unix"
)

const DriverName = "fuseapps"

type fuseappsFDescript struct {
	pipe io.ReadCloser
	buf  bytes.Buffer
	err  <-chan error
}

type fuseappsInstance struct {
	cmd    *exec.Cmd
	params *image.MountParams
	stdout fuseappsFDescript
	stderr fuseappsFDescript
}

type fuseappsFeature struct {
	binName   string
	cmdPath   string
	instances []*fuseappsInstance
}

type fuseappsDriver struct {
	squashFeature  fuseappsFeature
	ext3Feature    fuseappsFeature
	overlayFeature fuseappsFeature
	gocryptFeature fuseappsFeature
	features       image.DriverFeature
	cmdPrefix      []string
	squashSetUID   bool
	unprivileged   bool
	stopped        atomic.Bool
	mountErrCh     chan error
	instanceCh     chan *fuseappsInstance
}

func (f *fuseappsFeature) init(binNames string, purpose string, desired image.DriverFeature) bool {
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
		return false
	}
	return true
}

func InitImageDrivers(register, unprivileged bool, fileconf *apptainerconf.File, desiredFeatures image.DriverFeature) error {
	if fileconf.ImageDriver != "" && fileconf.ImageDriver != DriverName {
		sylog.Debugf("Skipping installing %v image driver because %v already configured", DriverName, fileconf.ImageDriver)
		// allow a configured driver to take precedence
		return nil
	}

	if !unprivileged && os.Getuid() == 0 {
		// Skip all the FUSE drivers when running as root
		sylog.Debugf("Skipping installing %v image driver because running as root", DriverName)
		return nil
	}

	var squashFeature fuseappsFeature
	var ext3Feature fuseappsFeature
	var overlayFeature fuseappsFeature
	var gocryptFeature fuseappsFeature
	var features image.DriverFeature
	// Always initialize the SquashFeature because it is needed by
	// the GocryptFeature which can be used even in privileged mode.
	// However, only indicate that it is available when it is needed
	// for other reasons, because when it is marked as available it
	// takes precedence over the kernel squashfs.
	if unprivileged || !squashfs.SetuidMountAllowed(fileconf) {
		if squashFeature.init("squashfuse_ll|squashfuse", "mount SIF or other squashfs files", desiredFeatures&image.SquashFeature) {
			features |= image.SquashFeature
		}
	} else {
		squashFeature.init("squashfuse_ll|squashfuse", "use gocryptfs", desiredFeatures&image.SquashFeature)
	}
	if unprivileged || !fileconf.AllowSetuidMountExtfs {
		if ext3Feature.init("fuse2fs", "mount EXT3 filesystems", desiredFeatures&image.Ext3Feature) {
			features |= image.Ext3Feature
		}
	}
	// Always initialize the OverlayFeature because the kernel overlay
	// doesn't like using FUSE for lower or upper layers.
	if overlayFeature.init("fuse-overlayfs", "use FUSE overlay", desiredFeatures&image.OverlayFeature) {
		features |= image.OverlayFeature
	}
	// gocryptfs is always available
	if gocryptFeature.init("gocryptfs", "use gocryptfs", desiredFeatures&image.GocryptFeature) {
		features |= image.GocryptFeature
	}

	// squashfuse generally supports the -o uid and -o gid options, except
	// on Debian 18.04, but it doesn't show in the help output so we just
	// assume that it does support it.  squashfuse_ll doesn't generally
	// support them, but when it does they are in the help output so we
	// scan for that.
	// See https://github.com/apptainer/apptainer/issues/736
	squashSetUID := true
	if squashFeature.binName == "squashfuse_ll" {
		squashSetUID = false
		cmd := exec.Command(squashFeature.cmdPath)
		output, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("error making squashfuse_ll output pipe: %v", err)
		}
		cmd.Stderr = cmd.Stdout
		err = cmd.Start()
		if err != nil {
			return fmt.Errorf("error starting squashfuse_ll: %v", err)
		}
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "-o uid=") {
				squashSetUID = true
				sylog.Debugf("squashfuse_ll supports -o uid")
				break
			}
		}
		_ = cmd.Wait()
	}

	if squashFeature.cmdPath != "" || ext3Feature.cmdPath != "" || overlayFeature.cmdPath != "" || gocryptFeature.cmdPath != "" {
		sylog.Debugf("Setting ImageDriver to %v", DriverName)
		fileconf.ImageDriver = DriverName
		if register {
			driver := &fuseappsDriver{
				squashFeature:  squashFeature,
				ext3Feature:    ext3Feature,
				overlayFeature: overlayFeature,
				gocryptFeature: gocryptFeature,
				features:       features,
				cmdPrefix:      []string{},
				squashSetUID:   squashSetUID,
				unprivileged:   unprivileged,
				mountErrCh:     make(chan error, 1),
				instanceCh:     make(chan *fuseappsInstance),
			}
			return image.RegisterDriver(DriverName, driver)
		}
	}
	return nil
}

func (d *fuseappsDriver) Features() image.DriverFeature {
	return d.features
}

//nolint:maintidx
func (d *fuseappsDriver) Mount(params *image.MountParams, _ image.MountFunc) error {
	extraFiles := 0
	sourceFd := -1
	if path.Dir(params.Source) == "/proc/self/fd" {
		sourceFd, _ = strconv.Atoi(path.Base(params.Source))
		// this becomes the first ExtraFiles, always fd 3
		params.Source = "/proc/self/fd/3"
		extraFiles++
	}
	waitForMount := true
	targetFd := -1
	if !d.unprivileged {
		if !strings.HasPrefix(params.Target, "/dev/fd/") {
			return fmt.Errorf("program error: in privileged mode the image driver mount target must start with \"/dev/fd/\"")
		}
		// drop privileges
		params.DontElevatePrivs = true
		// get the target file descriptor
		targetFd, _ = strconv.Atoi(path.Base(params.Target))
		// this becomes another of the ExtraFiles
		params.Target = fmt.Sprintf("/dev/fd/%d", 3+extraFiles)
		extraFiles++
		// don't wait for the mountpoint, it's already mounted
		waitForMount = false
	}

	var f *fuseappsFeature
	var cmd *exec.Cmd
	cmdArgs := d.cmdPrefix
	// This avoids sometimes seeing "Permission denied" when FUSE
	// is fooled into thinking two different user ids are involved.
	optsStr := "allow_other"
	if (params.Flags & syscall.MS_RDONLY) != 0 {
		optsStr += ",ro"
	}
	switch params.Filesystem {
	case "overlay":
		f = &d.overlayFeature
		if len(params.FSOptions) > 0 {
			optsStr += "," + strings.Join(params.FSOptions, ",")
		}
		// Ignore xino=on option with fuse-overlayfs
		optsStr = strings.ReplaceAll(optsStr, ",xino=on", "")
		// noacl is needed to avoid failures when the upper layer
		// filesystem type (for example tmpfs) does not support it,
		// when the fuse-overlayfs version is 1.8 or greater.
		optsStr += ",noacl"
		cmdArgs = append(cmdArgs, f.cmdPath, "-f", "-o", optsStr, params.Target)
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)

	case "squashfs":
		f = &d.squashFeature
		if d.squashSetUID {
			optsStr += fmt.Sprintf(",uid=%v,gid=%v", os.Getuid(), os.Getgid())
		}
		if params.Offset > 0 {
			optsStr += ",offset=" + strconv.FormatUint(params.Offset, 10)
		}
		cmdArgs = append(cmdArgs, f.cmdPath, "-f")
		if optsStr != "" {
			cmdArgs = append(cmdArgs, "-o", optsStr)
		}
		cmdArgs = append(cmdArgs, params.Source, params.Target)
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	case "gocryptfs":
		f = &d.gocryptFeature
		cmdArgs = append(cmdArgs, f.cmdPath, "-fg", params.Source, params.Target)
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n", string(params.Key)))
	case "ext3":
		f = &d.ext3Feature
		if os.Getuid() != 0 {
			// Bypass permission checks so all can be read,
			//  especially overlay work dir
			optsStr += ",fakeroot"
		}
		stdbuf, err := bin.FindBin("stdbuf")
		if err == nil {
			// Run fuse2fs through stdbuf to be able to read the
			//  warnings sometimes sent through stdout
			cmdArgs = append(cmdArgs, stdbuf, "-oL")
		}
		cmdArgs = append(cmdArgs, f.cmdPath, "-f", "-o", optsStr, params.Source, params.Target)
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)

		if params.Offset > 0 {
			// fuse2fs cannot natively offset into a file,
			//  so load a preload wrapper
			cmd.Env = []string{
				"LD_PRELOAD=" + buildcfg.LIBEXECDIR + "/apptainer/lib/offsetpreload.so",
				"OFFSETPRELOAD_FILE=" + params.Source,
				"OFFSETPRELOAD_OFFSET=" + strconv.FormatUint(params.Offset, 10),
			}
			for _, e := range cmd.Env {
				sylog.Debugf("Setting env %s", e)
			}
		}

	case "encryptfs":
		return fmt.Errorf("reading a root-encrypted SIF requires root or a suid installation")

	default:
		return fmt.Errorf("filesystem type %v not recognized by image driver", params.Filesystem)
	}

	if f.cmdPath == "" {
		return fmt.Errorf("image driver command for %v type not available", params.Filesystem)
	}

	sylog.Debugf("Executing %v", cmd.String())

	// Use our own go routines for reading from the command output
	// pipes instead of those hidden inside of os/exec in order to
	// better control synchronizing when the child process exits;
	// the os/exec Wait() function does not give enough control.
	var err error
	var stdoutPipe io.ReadCloser
	var stderrPipe io.ReadCloser
	stdoutPipe, err = cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error getting command stdout pipe: %v", err)
	}
	stderrPipe, err = cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error getting command stderr pipe: %v", err)
	}
	stdoutErr := make(chan error, 1)
	stderrErr := make(chan error, 1)
	instance := &fuseappsInstance{
		cmd:    cmd,
		params: params,
		stdout: fuseappsFDescript{
			stdoutPipe,
			bytes.Buffer{},
			stdoutErr,
		},
		stderr: fuseappsFDescript{
			stderrPipe,
			bytes.Buffer{},
			stderrErr,
		},
	}
	f.instances = append(f.instances, instance)
	go func() {
		_, err := io.Copy(&instance.stdout.buf, stdoutPipe)
		stdoutErr <- err
	}()
	go func() {
		_, err := io.Copy(&instance.stderr.buf, stderrPipe)
		stderrErr <- err
	}()

	if extraFiles > 0 {
		cmd.ExtraFiles = make([]*os.File, extraFiles)
		idx := 0
		if sourceFd >= 0 {
			cmd.ExtraFiles[idx] = os.NewFile(uintptr(sourceFd), params.Source)
			idx++
		}
		if targetFd >= 0 {
			cmd.ExtraFiles[idx] = os.NewFile(uintptr(targetFd), params.Target)
		}
	}

	// When using gocryptfs for build step or when running in setuid mode,
	// we should not run with the elevated CAP_SYS_ADMIN privilege.
	if !params.DontElevatePrivs {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			AmbientCaps: []uintptr{
				uintptr(capabilities.Map["CAP_SYS_ADMIN"].Value),
				// Needed for nsenter
				//  https://stackoverflow.com/a/69724124/10457761
				uintptr(capabilities.Map["CAP_SYS_PTRACE"].Value),
				// Required for fuse-overlayfs
				//  see https://github.com/containers/fuse-overlayfs/issues/414#issuecomment-1956140097
				uintptr(capabilities.Map["CAP_DAC_OVERRIDE"].Value),
			},
		}
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("%v Start failed: %v: %v", f.binName, err, instance.filterMsg())
	}
	process := cmd.Process
	if process == nil {
		return fmt.Errorf("no %v process started", f.binName)
	}

	d.instanceCh <- instance

	if !waitForMount {
		return nil
	}

	maxTime := 10 * time.Second
	infoTime := 2 * time.Second
	totTime := 0 * time.Second
	for totTime < maxTime {
		sleepTime := 25 * time.Millisecond
		time.Sleep(sleepTime)
		totTime += sleepTime

		// There is no need to check to see if the command exited
		// because the SIGCHLD signal handler will take care of it.

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
			msg := instance.filterMsg()
			if len(msg) > 0 {
				// Haven't seen this happen, but just in case
				sylog.Infof("%v", msg)
			}
			if totTime > infoTime {
				sylog.Infof("%v mount took an unexpectedly long time: %v", f.binName, totTime)
			} else {
				sylog.Debugf("%v mounted in %v", params.Target, totTime)
			}
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
				tmpfile, err := os.CreateTemp(binpath, ".tmp*")
				if err != nil {
					sylog.Debugf("%v not writable: %v", binpath, err)
					sylog.Infof("/usr/bin not writable in container")
					sylog.Infof("Consider using a different overlay upper layer filesystem type")
				} else {
					sylog.Debugf("Successfully created %v", tmpfile.Name())
					tmpfile.Close()
					os.Remove(tmpfile.Name())
				}
			}
			return nil
		}
	}

	_ = f.stop(params.Target, true)
	return fmt.Errorf("%v failed to mount %v in %v", f.binName, params.Target, maxTime)
}

func (d *fuseappsDriver) Start(_ *image.DriverParams, containerPid int, hybrid bool) error {
	// start process monitor
	d.monitor()

	if hybrid {
		// Running in hybrid setuid-fakeroot mode
		// Need any subcommand to first enter the container's
		//  user namespace
		nsenter, err := bin.FindBin("nsenter")
		if err != nil {
			return fmt.Errorf("failed to find nsenter: %v", err)
		}
		d.cmdPrefix = []string{
			nsenter,
			fmt.Sprintf("--user=/proc/%d/ns/user", containerPid),
			"-F",
		}
	}

	return nil
}

// Stop the process associated with the mount target, if there is one.
// If kill is not true, an unmount should already have happened so at
// first just wait for the process to exit.
func (f *fuseappsFeature) stop(target string, kill bool) error {
	for _, instance := range f.instances {
		if instance.params.Target != target || instance.cmd == nil {
			continue
		}

		process := instance.cmd.Process

		sylog.Debugf("Waiting for %v pid %v to exit", f.binName, process.Pid)

		waitCh := make(chan struct{})

		go func() {
			siginfo := new(unix.Siginfo)
			for {
				err := unix.Waitid(unix.P_PID, process.Pid, siginfo, unix.WNOWAIT|unix.WEXITED, nil)
				if err != syscall.EINTR {
					waitCh <- struct{}{}
					return
				}
			}
		}()

		if kill {
			sylog.Debugf("Terminating pid %v", process.Pid)
			_ = instance.cmd.Process.Signal(syscall.SIGTERM)
		}

		// waitTimeout is time to wait until terminate signal and
		//   again to wait until force kill signal
		waitTimeout := 1 * time.Second
		timer := time.NewTimer(waitTimeout)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				if !kill {
					// set it to true and reset timer for an additional
					// second timeout before force kill
					kill = true
					timer.Reset(waitTimeout)
					sylog.Debugf("Terminating pid %v after wait timeout", process.Pid)
					sylog.Infof("Terminating %v after timeout", f.binName)
					sylog.Infof("Timeouts can be caused by a running background process")
					_ = process.Signal(syscall.SIGTERM)
				} else {
					sylog.Debugf("Killing pid %v after wait timeout", process.Pid)
					_ = process.Kill()
				}
			case <-waitCh:
				close(waitCh)
				return f.waitInstance(instance)
			}
		}
	}

	return nil
}

func (f *fuseappsFeature) waitInstance(instance *fuseappsInstance) error {
	cmd := instance.cmd
	instance.cmd = nil

	// wait for the go funcs reading from the process to exit
	if err := <-instance.stdout.err; err != nil {
		sylog.Debugf("Error from %v stdout: %v", f.binName, err)
	}
	if err := <-instance.stderr.err; err != nil {
		sylog.Debugf("Error from %v stderr: %v", f.binName, err)
	}

	err := cmd.Wait()
	if err == nil {
		return nil
	}

	var status syscall.WaitStatus

	if ee, ok := err.(*exec.ExitError); ok {
		status, ok = ee.Sys().(syscall.WaitStatus)
		if !ok {
			return err
		}
	} else {
		return err
	}

	pid := cmd.Process.Pid
	sylog.Debugf("%v pid %v has exited with status %v", f.binName, pid, status.ExitStatus())

	errmsg := instance.filterMsg()
	if errmsg != "" {
		return fmt.Errorf("%v exited: %v", f.binName, errmsg)
	}

	return nil
}

func (d *fuseappsDriver) allFeatures() []fuseappsFeature {
	return []fuseappsFeature{d.squashFeature, d.ext3Feature, d.overlayFeature, d.gocryptFeature}
}

func (d *fuseappsDriver) Stop(target string) error {
	if !d.stopped.Swap(true) {
		close(d.mountErrCh)
		close(d.instanceCh)
	}

	if target == "" {
		return nil
	}

	for _, feature := range d.allFeatures() {
		if err := feature.stop(target, false); err != nil {
			return err
		}
	}

	return nil
}

func (d *fuseappsDriver) MountErr() error {
	return <-d.mountErrCh
}

// Check if any of the child processes belonging to this image driver feature
// has stopped, and return the status and error if it has.
func (d *fuseappsDriver) checkStopped() {
	for instance := range d.instanceCh {
		go func(instance *fuseappsInstance) {
			pid := instance.cmd.Process.Pid
			for {
				siginfo := new(unix.Siginfo)
				err := unix.Waitid(unix.P_PID, pid, siginfo, unix.WNOWAIT|unix.WEXITED, nil)
				if err != syscall.EINTR {
					if d.stopped.Load() {
						return
					}
					for _, feature := range d.allFeatures() {
						for _, featureInstance := range feature.instances {
							if featureInstance != instance {
								continue
							}
							err := feature.waitInstance(instance)
							if err != nil {
								d.mountErrCh <- fmt.Errorf("image driver %s instance exited with error: %s", feature.binName, err)
							}
						}
					}
					return
				}
			}
		}(instance)
	}
}

func (d *fuseappsDriver) monitor() {
	go d.checkStopped()
}

func (i *fuseappsInstance) filterMsg() string {
	ignoreMsgs := []string{
		// from fuse2fs
		"journal is not supported.",
		"fuse2fs does not support using the journal.",
		"There may be file system corruption",
		"the file system is not gracefully unmounted.",
		"Mounting read-only.",
		// from squashfuse_ll
		"failed to clone device fd",
		"continue without -o clone_fd",
		// from fuse-overlayfs sometimes
		"unknown argument ignored: lazytime",
		// from fuse-overlayfs due to a bug
		// (see https://github.com/containers/fuse-overlayfs/issues/397)
		"/proc seems to be mounted as readonly",
		// from gocryptfs
		"Reading Password from stdin",
		"Decrypting master key",
		"Filesystem mounted and ready.",
		// from any of the programs when an older fuse3 lib is used
		// than what the programs were compiled with
		"fuse: warning: library too old",
		// from any of the programs
		"The option \"-allow_other\" is set",
	}

	errmsg := ""
	for idx, buf := range []bytes.Buffer{i.stdout.buf, i.stderr.buf} {
		str := buf.String()
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
				errmsg += line + "\n"
			}
		}
	}
	if strings.Contains(errmsg, "fusermount") {
		sylog.Infof("A fusermount error may indicate that the kernel is too old")
		if i.params.Filesystem == "squashfs" {
			sylog.Infof("The --unsquash option may work around it")
		}
	}
	return errmsg
}
