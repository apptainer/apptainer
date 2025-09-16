// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
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
	tests := CgroupTests{
		{
			name:     "GetCGroupRootPath",
			testFunc: testGetCgroupRootPathV2,
		},
		{
			name:     "GetCGroupRelPath",
			testFunc: testGetCgroupRelPathV2,
		},
		{
			name:     "NewUpdate",
			testFunc: testNewUpdateV2,
		},
		{
			name:     "UpdateUnified",
			testFunc: testUpdateUnifiedV2,
		},
		{
			name:     "AddProc",
			testFunc: testAddProcV2,
		},
		{
			name:     "FreezeThaw",
			testFunc: testFreezeThawV2,
		},
	}
	runCgroupfsTests(t, tests)
	runSystemdTests(t, tests)
}

//nolint:dupl
func testGetCgroupRootPathV2(t *testing.T, systemd bool) {
	// This cgroup won't be created in the fs as we don't add a PID through the manager
	group := filepath.Join("/apptainer/rootpathtest")
	if systemd {
		group = "system.slice:apptainer:rootpathtest"
	}

	manager, err := newManager(&specs.LinuxResources{}, group, systemd)
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

func testGetCgroupRelPathV2(t *testing.T, systemd bool) {
	// This cgroup won't be created in the fs as we don't add a PID through the manager
	group := filepath.Join("/apptainer/rootpathtest")
	wantPath := group
	if systemd {
		group = "system.slice:apptainer:rootpathtest"
		wantPath = "/system.slice/apptainer-rootpathtest.scope"
	}

	manager, err := newManager(&specs.LinuxResources{}, group, systemd)
	if err != nil {
		t.Fatalf("While creating manager: %v", err)
	}

	relPath, err := manager.GetCgroupRelPath()
	if err != nil {
		t.Errorf("While getting root path: %v", err)
	}

	if relPath != wantPath {
		t.Errorf("Expected %s, got %s", wantPath, relPath)
	}
}

//nolint:dupl
func testNewUpdateV2(t *testing.T, systemd bool) {
	_, manager, cleanup := testManager(t, systemd)
	defer cleanup()

	// For cgroups v2 [pids] limit -> pids.max
	// Check for correct 1024 value
	pidsMax := filepath.Join(manager.cgroup.Path(""), "pids.max")
	ensureInt(t, pidsMax, 1024)

	// Write a new config with [pids] limit = 512
	content := []byte("[pids]\nlimit = 512")
	tmpfile := filepath.Join(t.TempDir(), "cgroups")
	if err := os.WriteFile(tmpfile, content, 0o644); err != nil {
		t.Fatalf("While writing update file: %v", err)
	}

	// Update existing cgroup from new config
	if err := manager.UpdateFromFile(tmpfile); err != nil {
		t.Fatalf("While updating cgroup: %v", err)
	}

	// Check pids.max is now 512
	ensureInt(t, pidsMax, 512)
}

//nolint:dupl
func testUpdateUnifiedV2(t *testing.T, systemd bool) {
	// Apply a 1024 pids.max limit using the v1 style config that sets [pids] limit
	_, manager, cleanup := testManager(t, systemd)
	defer cleanup()
	pidsMax := filepath.Join(manager.cgroup.Path(""), "pids.max")
	ensureInt(t, pidsMax, 1024)

	// Update existing cgroup from unified style config setting [Unified] pids.max directly
	if err := manager.UpdateFromFile("example/cgroups-unified.toml"); err != nil {
		t.Fatalf("While updating cgroup: %v", err)
	}

	// Check pids.max is now 512
	ensureInt(t, pidsMax, 512)
}

//nolint:dupl
func testAddProcV2(t *testing.T, systemd bool) {
	pid, manager, cleanup := testManager(t, systemd)

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

	cgroupProcs := filepath.Join(manager.cgroup.Path(""), "cgroup.procs")
	ensureContainsInt(t, cgroupProcs, int64(pid))
	ensureContainsInt(t, cgroupProcs, int64(newPid))
}

//nolint:dupl
func testFreezeThawV2(t *testing.T, systemd bool) {
	manager := &Manager{}
	if err := manager.Freeze(); err == nil {
		t.Errorf("unexpected success freezing PID 0")
	}
	if err := manager.Thaw(); err == nil {
		t.Errorf("unexpected success thawing PID 0")
	}

	pid, manager, cleanup := testManager(t, systemd)
	defer cleanup()

	manager.Freeze()
	// cgroups v2 freeze is to interruptible sleep, which could actually occur
	// for our cat /dev/zero while it's running, so check freeze marker as well
	// as the process state here.
	ensureStateBecomes(t, pid, "S")
	freezePath := path.Join(manager.cgroup.Path(""), "cgroup.freeze")
	ensureInt(t, freezePath, 1)

	manager.Thaw()
	ensureStateBecomes(t, pid, "RS")
	ensureInt(t, freezePath, 0)
}
