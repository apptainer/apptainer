// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
)

// This file contains tests that will run under cgroups v1 only.

//nolint:dupl
func TestCgroupsV1(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV1(t)

	// Create process to put into a cgroup
	cmd := exec.Command("/bin/cat", "/dev/zero")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	pid := cmd.Process.Pid
	strPid := strconv.Itoa(pid)
	group := filepath.Join("/apptainer", strPid)

	cgroupsToml := "example/cgroups.toml"
	// Some systems, e.g. ppc64le may not have a 2MB page size, so don't
	// apply a 2MB hugetlb limit if that's the case.
	_, err := os.Stat("/sys/fs/cgroup/hugetlb/hugetlb.2MB.limit_in_bytes")
	if os.IsNotExist(err) {
		t.Log("No hugetlb.2MB.limit_in_bytes - using alternate cgroups test file")
		cgroupsToml = "example/cgroups-no-hugetlb.toml"
	}

	manager, err := NewManagerWithFile(cgroupsToml, pid, group)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
		manager.Destroy()
	}()

	rootPath, err := manager.GetCgroupRootPath()
	if err != nil {
		t.Fatalf("can't determine cgroups root path, is cgroups enabled ?")
	}

	cpuShares := filepath.Join(rootPath, "cpu", group, "cpu.shares")
	ensureIntInFile(t, cpuShares, 1024)

	content := []byte("[cpu]\nshares = 512")
	tmpfile, err := ioutil.TempFile("", "cgroups")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	manager, err = GetManagerForPid(pid)
	if err != nil {
		t.Fatal(err)
	}

	// Update existing cgroup from new config
	if err := manager.UpdateFromFile(tmpfile.Name()); err != nil {
		t.Fatal(err)
	}
	ensureIntInFile(t, cpuShares, 512)
}

//nolint:dupl
func TestPauseResumeV1(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV1(t)

	manager := &Manager{}
	if err := manager.Freeze(); err == nil {
		t.Errorf("unexpected success with PID 0")
	}
	if err := manager.Thaw(); err == nil {
		t.Errorf("unexpected success with PID 0")
	}

	cmd := exec.Command("/bin/cat", "/dev/zero")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	group := filepath.Join("/apptainer", strconv.Itoa(cmd.Process.Pid))
	manager, err := NewManagerWithFile("example/cgroups.toml", cmd.Process.Pid, group)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
		manager.Destroy()
	}()

	manager.Freeze()
	// cgroups v1 freeze is to uninterruptible sleep
	ensureState(t, cmd.Process.Pid, "D")

	manager.Thaw()
	ensureState(t, cmd.Process.Pid, "RS")
}
