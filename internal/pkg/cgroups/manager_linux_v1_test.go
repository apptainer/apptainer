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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// This file contains tests that will run under cgroups v1 only.

func TestCgroupsV1(t *testing.T) {
	test.EnsurePrivilege(t)
	require.CgroupsV1(t)
	tests := CgroupTests{
		{
			name:     "GetCGroupRootPath",
			testFunc: testGetCgroupRootPathV1,
		},
		{
			name:     "GetCGroupRelPath",
			testFunc: testGetCgroupRelPathV1,
		},
		{
			name:     "NewUpdate",
			testFunc: testNewUpdateV1,
		},
		{
			name:     "UpdateUnified",
			testFunc: testUpdateUnifiedV1,
		},
		{
			name:     "AddProc",
			testFunc: testAddProcV1,
		},
		{
			name:     "FreezeThaw",
			testFunc: testFreezeThawV1,
		},
	}
	runCgroupfsTests(t, tests)
	runSystemdTests(t, tests)
}

//nolint:dupl
func testGetCgroupRootPathV1(t *testing.T, systemd bool) {
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

	// With v1 the mount could be somewhere odd... but we can test indirectly.
	// The root path + '/devices' + the rel path should give us the absolute path
	// for the cgroup with the devices controller.
	// The rel path is tested explicitly, so we know it works.
	relPath, err := manager.GetCgroupRelPath()
	if err != nil {
		t.Errorf("While getting rel path: %v", err)
	}

	absDevicePath := path.Join(rootPath, "devices", relPath)
	if absDevicePath != manager.cgroup.Path("devices") {
		t.Errorf("Expected %s, got %s", unifiedMountPoint, rootPath)
	}
}

//nolint:dupl
func testGetCgroupRelPathV1(t *testing.T, systemd bool) {
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
func testNewUpdateV1(t *testing.T, systemd bool) {
	_, manager, cleanup := testManager(t, systemd)
	defer cleanup()

	// Check for correct 1024 value
	pidsMax := filepath.Join(manager.cgroup.Path("pids"), "pids.max")
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
func testUpdateUnifiedV1(t *testing.T, systemd bool) {
	_, manager, cleanup := testManager(t, systemd)
	defer cleanup()

	// Try to update existing cgroup from unified style config setting [Unified] pids.max directly
	if err := manager.UpdateFromFile("example/cgroups-unified.toml"); err == nil {
		t.Fatalf("Unexpected success applying unified config on cgroups v1")
	}
}

func testAddProcV1(t *testing.T, systemd bool) {
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

	cgroupProcs := filepath.Join(manager.cgroup.Path("pids"), "cgroup.procs")
	ensureContainsInt(t, cgroupProcs, int64(pid))
	ensureContainsInt(t, cgroupProcs, int64(newPid))
}

func testFreezeThawV1(t *testing.T, systemd bool) {
	manager := &Manager{}
	if err := manager.Freeze(); err == nil {
		t.Errorf("unexpected success with PID 0")
	}
	if err := manager.Thaw(); err == nil {
		t.Errorf("unexpected success with PID 0")
	}

	pid, manager, cleanup := testManager(t, systemd)
	defer cleanup()

	manager.Freeze()
	// cgroups v1 freeze is to uninterruptible sleep
	ensureStateBecomes(t, pid, "D")

	manager.Thaw()
	ensureStateBecomes(t, pid, "RS")
}
