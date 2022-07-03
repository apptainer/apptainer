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
	"github.com/apptainer/apptainer/pkg/sylog"
)

// re-exec the command effectively under unshare -r
func UnshareRootMapped(args []string) error {
	cmd := osExec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWUSER
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: syscall.Getuid(), Size: 1},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: syscall.Getgid(), Size: 1},
	}
	sylog.Debugf("Re-executing to root-mapped unprivileged user namespace")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Error re-executing in root-mapped unprivileged user namespace: %v", err)
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

// This just adds debug messages around bin.FindBin("fakeroot")
func FindFake() (string, error) {
	sylog.Debugf("looking for the fakeroot command")
	fakerootPath, err := bin.FindBin("fakeroot")
	if err != nil {
		sylog.Debugf("failure finding fakeroot: %v", err)
		return "", err
	}
	sylog.Debugf("fakeroot found at %v", fakerootPath)
	return fakerootPath, nil
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

	// Start by examining the environment fakeroot creates
	cmd := osExec.Command(fakerootPath, "env")
	env := os.Environ()
	for idx := range env {
		if strings.HasPrefix(env[idx], "LD_LIBRARY_PATH=") {
			// Remove any incoming LD_LIBRARY_PATH
			env[idx] = "LD_LIBRARY_PREFIX="
		}
	}
	cmd.Env = env
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
		return binds, fmt.Errorf("No LD_PRELOAD in fakeroot environment")
	}
	if libraryPath == "" {
		return binds, fmt.Errorf("No LD_LIBRARY_PATH in fakeroot environment")
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
		if _, err = os.Stat(src); err == nil {
			binds[1] = src + ":" + point
		}
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
