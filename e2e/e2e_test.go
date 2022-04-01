// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build e2e_test
// +build e2e_test

package e2e

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"

	// This import will execute a CGO section with the help of a C constructor
	// section "init". It will create a dedicated mount namespace for the e2e tests
	// and will restore identity to the original user but will retain privileges for
	// Privileged method enabling the execution of a function with root privileges
	// when required
	_ "github.com/apptainer/apptainer/e2e/internal/e2e/init"

	"golang.org/x/sys/unix"
)

func TestE2E(t *testing.T) {
	targetCoverageFilePath := os.Getenv("APPTAINER_E2E_COVERAGE")
	if targetCoverageFilePath != "" {
		logFile, err := os.OpenFile(targetCoverageFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
		if err != nil {
			log.Fatalf("failed to create log file: %s", err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
		log.Println("List of commands called by E2E")
	} else {
		log.SetOutput(ioutil.Discard)
	}

	RunE2ETests(t)
}

func TestMain(m *testing.M) {
	if os.Getenv("E2E_NO_REAPER") != "" {
		ret := m.Run()
		os.Exit(ret)
	}

	// start reaper process
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(1), 0, 0, 0); err != nil {
		log.Fatalf("failed to create reaper process: %s", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh)

	executable, err := os.Executable()
	if err != nil {
		log.Fatalf("unable to determine current executable path: %s", err)
	}

	os.Setenv("E2E_NO_REAPER", "1")

	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	// create a mount namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("e2e test re-execution failed: %s", err)
	}
	cmdPid := cmd.Process.Pid

	for s := range sigCh {
		switch s {
		case syscall.SIGCHLD:
			// reap all childs
			for {
				var status syscall.WaitStatus

				childPid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
				if childPid <= 0 || err != nil {
					break
				}
				if childPid == cmdPid {
					killAllChilds()
					os.Exit(status.ExitStatus())
				}
			}
		default:
			// forward signals to e2e test command
			syscall.Kill(cmdPid, s.(syscall.Signal))
		case syscall.SIGURG:
			// ignore goroutine preemption
			break
		}
	}
}

// kill all direct childs
func killAllChilds() {
	currentPid := os.Getpid()

	matches, err := filepath.Glob("/proc/*/stat")
	if err != nil {
		log.Fatal(err)
	}
	for _, match := range matches {
		statData := ""
		switch match {
		case "/proc/net/stat", "/proc/self/stat", "/proc/thread-self/stat":
		default:
			d, err := ioutil.ReadFile(match)
			if err != nil {
				continue
			}
			statData = string(bytes.TrimSpace(d))
		}
		if statData == "" {
			continue
		}
		pid := 0
		ppid := 0
		if n, err := fmt.Sscanf(statData, "%d %s %c %d", &pid, new(string), new(byte), &ppid); err != nil {
			continue
		} else if n != 4 || ppid != currentPid {
			continue
		}
		// best effort to wait child
		_ = syscall.Kill(pid, syscall.SIGKILL)
		_, _ = syscall.Wait4(pid, nil, 0, nil)
	}
}
