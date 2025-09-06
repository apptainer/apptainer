// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// This file is for "fake fakeroot", that is, root-mapped unprivileged
//   user namespaces (unshare -r) and the fakeroot command

package fakeroot

import (
	"bufio"
	"fmt"
	"os"
	osExec "os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apptainer/apptainer/internal/pkg/util/bin"
	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// exec the command effectively under unshare -r or unshare -rm
func UnshareRootMapped(args []string, includeMountNamespace bool) error {
	cmd := osExec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWUSER
	if includeMountNamespace {
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNS
	}
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: syscall.Getuid(), Size: 1},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: syscall.Getgid(), Size: 1},
	}
	sylog.Debugf("Executing %s in root-mapped unprivileged user namespace", args[0])
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error re-executing in root-mapped unprivileged user namespace: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*osExec.ExitError); ok {
			// exit with the non-zero exit code
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		sylog.Fatalf("error waiting for root-mapped unprivileged command: %v", err)
	}
	return nil
}

// Make use of UnshareRootMapped to test to see if a user namespace
// can be allocated
func UserNamespaceAvailable() bool {
	err := UnshareRootMapped([]string{"/bin/true"}, false)
	if err != nil {
		sylog.Debugf("UnshareRootMapped failed: %v", err)
		return false
	}
	return true
}

// Look for fakeroot-sysv first and then fakeroot, since fakeroot-sysv
// is much faster than fakeroot-tcp.
func FindFake() (string, error) {
	var err error
	for _, cmd := range []string{"fakeroot-sysv", "fakeroot"} {
		sylog.Debugf("looking for the %v command", cmd)
		var fakerootPath string
		fakerootPath, err = bin.FindBin(cmd)
		if err == nil {
			sylog.Debugf("%v found at %v", cmd, fakerootPath)
			return fakerootPath, nil
		}
		sylog.Debugf("failure finding %v: %v", cmd, err)
	}
	return "", err
}

// Get the args needed to execute the fakeroot mapped into the container
func GetFakeArgs() []string {
	return []string{
		"/.singularity.d/libs/fakeroot",
		"-f",
		"/.singularity.d/libs/faked",
		"-l",
		"/.singularity.d/libs/libfakeroot.so",
	}
}

// Get the binds needed to map the fakeroot command into the container
// The incoming parameter is the path to fakeroot
func GetFakeBinds(fakerootPath string) ([]string, error) {
	args := GetFakeArgs()
	binds := []string{
		args[0],
		args[2],
		args[4],
	}

	if fakerootPath == args[0] {
		// The binding has already been done, this is for nesting
		return binds, nil
	}

	// Start by examining the environment fakeroot creates
	cmd := osExec.Command(fakerootPath, "env")
	environ := os.Environ()
	hasPath := false
	for idx := range environ {
		if strings.HasPrefix(environ[idx], "LD_LIBRARY_PATH=") {
			// Remove any incoming LD_LIBRARY_PATH
			environ[idx] = "LD_LIBRARY_PREFIX="
		} else if strings.HasPrefix(environ[idx], "PATH=") &&
			environ[idx] != "PATH=" {
			hasPath = true
		}
	}
	if !hasPath {
		environ = append(environ, "PATH="+env.DefaultPath)
	}
	cmd.Env = environ
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return binds, fmt.Errorf("error make fakeroot stdout pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		return binds, fmt.Errorf("error starting fakeroot: %v", err)
	}
	preload := ""
	libraryPath := ""
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "LD_PRELOAD=") {
			preload = line[len("LD_PRELOAD="):]
		} else if strings.HasPrefix(line, "LD_LIBRARY_PATH=") {
			libraryPath = line[len("LD_LIBRARY_PATH="):]
		}
	}
	_ = cmd.Wait()
	if preload == "" {
		return binds, fmt.Errorf("no LD_PRELOAD in fakeroot environment")
	}
	if libraryPath == "" {
		return binds, fmt.Errorf("no LD_LIBRARY_PATH in fakeroot environment")
	}
	preloadEntries := strings.Split(preload, ":")
	for _, entry := range preloadEntries {
		if strings.HasPrefix(entry, "libfakeroot") {
			preload = entry
			break
		}
	}

	src := fakerootPath
	point := binds[0]
	binds[0] = src + ":" + point

	dir := filepath.Dir(src)
	src = filepath.Join(dir, "faked")
	point = binds[1]
	splits := strings.Split(preload, ".")
	splits = strings.Split(splits[0], "-")
	if len(splits) > 1 {
		// add the faked that corresponds to the preload library
		src += "-" + splits[1]
	}
	if _, err = os.Stat(src); err == nil {
		binds[1] = src + ":" + point
	}
	point = binds[2]
	splits = strings.Split(libraryPath, ":")
	for _, dir := range splits {
		// Find the preload library in libraryPath
		src = filepath.Join(dir, preload)
		if _, err = os.Stat(src); err == nil {
			binds[2] = src + ":" + point
			break
		}
	}
	return binds, nil
}
