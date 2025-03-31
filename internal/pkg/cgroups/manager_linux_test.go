// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
)

// This file contains tests that will run under cgroups v1 & v2, and test utility functions.

type (
	CgroupTestFunc func(t *testing.T, systemd bool)
	CgroupTest     struct {
		name     string
		testFunc CgroupTestFunc
	}
)
type CgroupTests []CgroupTest

func TestCgroups(t *testing.T) {
	tests := CgroupTests{
		{
			name:     "GetFromPid",
			testFunc: testGetFromPid,
		},
	}
	runCgroupfsTests(t, tests)
	runSystemdTests(t, tests)
}

func runCgroupfsTests(t *testing.T, tests CgroupTests) {
	t.Run("cgroupfs", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.testFunc(t, false)
			})
		}
	})
}

func runSystemdTests(t *testing.T, tests CgroupTests) {
	t.Run("systemd", func(t *testing.T) {
		if !fs.IsDir("/run/systemd/system") {
			t.Skip("systemd not running as init on this host")
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.testFunc(t, true)
			})
		}
	})
}

func testGetFromPid(t *testing.T, systemd bool) {
	test.EnsurePrivilege(t)
	require.Cgroups(t)

	// We create either a cgroupfs or systemd cgroup initially
	pid, manager, cleanup := testManager(t, systemd)
	defer cleanup()

	// We can only retrieve a cgroupfs managed cgroup from pid
	pidMgr, err := GetManagerForPid(pid)
	if err != nil {
		t.Fatalf("While getting cgroup manager for pid: %v", err)
	}

	relPath, err := manager.GetCgroupRelPath()
	if err != nil {
		t.Fatalf("While getting manager cgroup relative path")
	}

	if pidMgr.group != relPath {
		t.Errorf("Expected %s for cgroup from pid, got %s", manager.group, pidMgr.cgroup)
	}
}

// ensureInt asserts that the content of path is the integer wantInt
func ensureInt(t *testing.T, path string, wantInt int64) {
	file, err := os.Open(path)
	if err != nil {
		t.Errorf("while opening %q: %v", path, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	hasData := scanner.Scan()
	if !hasData {
		t.Errorf("no data found in %q", path)
	}

	val, err := strconv.ParseInt(scanner.Text(), 10, 64)
	if err != nil {
		t.Errorf("could not parse %q: %v", path, err)
	}

	if val != wantInt {
		t.Errorf("found %d in %q, expected %d", val, path, wantInt)
	}
}

// ensureContainsInt asserts that the content of path contains the integer wantInt
func ensureContainsInt(t *testing.T, path string, wantInt int64) {
	file, err := os.Open(path)
	if err != nil {
		t.Errorf("while opening %q: %v", path, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		val, err := strconv.ParseInt(scanner.Text(), 10, 64)
		if err != nil {
			t.Errorf("could not parse %q: %v", path, err)
		}
		if val == wantInt {
			return
		}
	}

	t.Fatalf("%s did not contain expected value %d", path, wantInt)
}

// ensureStateBecomes asserts that a process pid has any of the wanted
// states, or reaches one of these states in a 5 second window.
func ensureStateBecomes(t *testing.T, pid int, wantStates string) {
	const retries = 5
	const delay = time.Second

	file, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		t.Error(err)
	}
	defer file.Close()

	procState := ""

	for r := 0; r <= retries; r++ {
		if r > 0 {
			t.Logf("Process %d has state %q, need %q - retrying %d/%d", pid, procState, wantStates, r, retries)
			time.Sleep(delay)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("Could not seek to start of /proc/%d/status", pid)
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			// State:	R (running)
			if strings.HasPrefix(scanner.Text(), "State:\t") {
				f := strings.Fields(scanner.Text())
				if len(f) < 2 {
					t.Errorf("Could not check process state - not enough fields: %s", scanner.Text())
				}
				procState = f[1]
			}
		}

		if strings.ContainsAny(procState, wantStates) {
			return
		}
	}

	t.Errorf("Process %d did not reach expected state %q", pid, wantStates)
}

// testManager returns a cgroup manager, that has created a cgroup with a `cat /dev/zero` process,
// and example resource config.
func testManager(t *testing.T, systemd bool) (pid int, manager *Manager, cleanup func()) {
	// Create process to put into a cgroup
	t.Log("Creating test process")
	cmd := exec.Command("/bin/cat", "/dev/zero")
	if err := cmd.Start(); err != nil {
		t.Fatalf("While starting test process: %v", err)
	}
	pid = cmd.Process.Pid
	strPid := strconv.Itoa(pid)
	group := filepath.Join("/apptainer", strPid)
	if systemd {
		group = "system.slice:apptainer:" + strPid
	}

	cgroupsToml := "example/cgroups.toml"
	// Some systems, e.g. ppc64le may not have a 2MB page size, so don't
	// apply a 2MB hugetlb limit if that's the case.
	_, err := os.Stat("/sys/fs/cgroup/dev-hugepages.mount/hugetlb.2MB.max")
	if os.IsNotExist(err) {
		t.Log("No hugetlb.2MB.max - using alternate cgroups test file")
		cgroupsToml = "example/cgroups-no-hugetlb.toml"
	}

	manager, err = NewManagerWithFile(cgroupsToml, pid, group, systemd)
	if err != nil {
		t.Fatalf("While creating new cgroup: %v", err)
	}

	cleanup = func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
		manager.Destroy()
	}

	return pid, manager, cleanup
}
