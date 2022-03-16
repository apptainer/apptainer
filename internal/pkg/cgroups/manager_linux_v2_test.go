// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// This file contains tests that will run under cgroups v2 only.

func TestCgroupsV2(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV2Unified(t)
	t.Run("GetCgroupRootPath", testGetCgroupRootPathV2)
	t.Run("NewUpdate", testNewUpdateV2)
	t.Run("AddProc", testAddProcV2)
	t.Run("FreezeThaw", testFreezeThawV2)
}

//nolint:dupl
func testGetCgroupRootPathV2(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV2Unified(t)

	// This cgroup won't be created in the fs as we don't add a PID through the manager
	group := filepath.Join("/apptainer", "rootpathtest")
	manager, err := newManager(&specs.LinuxResources{}, group)
	if err != nil {
		t.Fatalf("While creating manager: %v", err)
	}

	rootPath, err := manager.GetCgroupRootPath()
	if err != nil {
		t.Errorf("While getting root path: %v", err)
	}
	// Cgroups v2 has a fixed mount point
	if rootPath != unifiedMountPoint {
		t.Errorf("Expected %s, got %s", unifiedMountPoint, rootPath)
	}
}

//nolint:dupl
func testNewUpdateV2(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV2Unified(t)

	_, manager, cleanup := testManager(t)
	defer cleanup()

	// For cgroups v2 [pids] limit -> pids.max
	// Check for correct 1024 value
	pidsMax := filepath.Join("/sys/fs/cgroup", manager.group, "pids.max")
	ensureInt(t, pidsMax, 1024)

	// Write a new config with [pids] limit = 512
	content := []byte("[pids]\nlimit = 512")
	tmpfile, err := ioutil.TempFile("", "cgroups")
	if err != nil {
		t.Fatalf("While creating update file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("While writing update file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("While closing update file: %v", err)
	}

	// Update existing cgroup from new config
	if err := manager.UpdateFromFile(tmpfile.Name()); err != nil {
		t.Fatalf("While updating cgroup: %v", err)
	}

	// Check pids.max is now 512
	ensureInt(t, pidsMax, 512)
}

func testAddProcV2(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV2Unified(t)

	pid, manager, cleanup := testManager(t)

	cmd := exec.Command("/bin/cat", "/dev/zero")
	if err := cmd.Start(); err != nil {
		t.Fatalf("While starting test process: %v", err)
	}
	newPid := cmd.Process.Pid
	defer func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
		cleanup()
	}()

	if err := manager.AddProc(newPid); err != nil {
		t.Errorf("While adding proc to cgroup: %v", err)
	}

	cgroupProcs := filepath.Join("/sys/fs/cgroup", manager.group, "cgroup.procs")
	ensureContainsInt(t, cgroupProcs, int64(pid))
	ensureContainsInt(t, cgroupProcs, int64(newPid))
}

//nolint:dupl
func testFreezeThawV2(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV2Unified(t)

	manager := &Manager{}
	if err := manager.Freeze(); err == nil {
		t.Errorf("unexpected success freezing PID 0")
	}
	if err := manager.Thaw(); err == nil {
		t.Errorf("unexpected success thawing PID 0")
	}

	pid, manager, cleanup := testManager(t)
	defer cleanup()

	manager.Freeze()
	// cgroups v2 freeze is to interruptible sleep, which could actually occur
	// for our cat /dev/zero while it's running, so check freeze marker as well
	// as the process state here.
	ensureState(t, pid, "S")
	freezePath := path.Join("/sys/fs/cgroup", manager.group, "cgroup.freeze")
	ensureInt(t, freezePath, 1)

	manager.Thaw()
	ensureState(t, pid, "RS")
	ensureInt(t, freezePath, 0)
}
