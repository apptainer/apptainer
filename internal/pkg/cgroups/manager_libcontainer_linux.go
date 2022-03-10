// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
// project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"fmt"
	"path/filepath"
	"strings"

	lccgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	lcmanager "github.com/opencontainers/runc/libcontainer/cgroups/manager"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const unifiedMountPoint = "/sys/fs/cgroup"

// ManagerLibcontainer manages a cgroup 'Group', using the runc/libcontainer packages
type ManagerLC struct {
	group  string
	pid    int
	cgroup lccgroups.Manager
}

func (m *ManagerLC) load() (err error) {
	if m.group != "" {
		return m.loadFromPath()
	}
	return m.loadFromPid()
}

func (m *ManagerLC) loadFromPid() (err error) {
	if m.pid == 0 {
		return fmt.Errorf("cannot load from pid - no process ID specified")
	}

	pidCGFile := fmt.Sprintf("/proc/%d/cgroup", m.pid)
	paths, err := lccgroups.ParseCgroupFile(pidCGFile)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", pidCGFile, err)
	}

	// cgroups v2 path is always given by the unified "" subsystem
	ok := false
	if lccgroups.IsCgroup2UnifiedMode() {
		m.group, ok = paths[""]
		if !ok {
			return fmt.Errorf("could not find cgroups v2 unified path")
		}
		return m.loadFromPath()
	}

	// For cgroups v1 we are relying on fetching the 'devices' subsystem path.
	// The devices subsystem is needed for our OCI engine and its presence is
	// enforced in runc/libcontainer/cgroups/fs initialization without 'skipDevices'.
	// This means we never explicitly put a container into a cgroup without a
	// set 'devices' path.
	m.group, ok = paths["devices"]
	if !ok {
		return fmt.Errorf("could not find cgroups v1 path (using devices subsystem)")
	}
	return m.loadFromPath()
}

func (m *ManagerLC) loadFromPath() (err error) {
	if m.group == "" {
		return fmt.Errorf("cannot load from path - no path specified")
	}

	lcConfig := &configs.Cgroup{
		Path:      m.group,
		Resources: &configs.Resources{},
	}

	m.cgroup, err = lcmanager.New(lcConfig)
	if err != nil {
		return fmt.Errorf("while creating cgroup manager: %w", err)
	}

	return nil
}

// GetCgroupRootPath returns cgroup root path
// TODO - this returns "" on error which needs to be checked for
// carefully. Should return an actual error instead.
func (m *ManagerLC) GetCgroupRootPath() string {
	if m.cgroup == nil {
		return ""
	}

	// v2 - has a single fixed mountpoint for the root cgroup
	if lccgroups.IsCgroup2UnifiedMode() {
		return unifiedMountPoint
	}

	// v1 - Get absolute paths to cgroup by subsystem
	subPaths := m.cgroup.GetPaths()

	// For cgroups v1 we are relying on fetching the 'devices' subsystem path.
	// The devices subsystem is needed for our OCI engine and its presence is
	// enforced in runc/libcontainer/cgroups/fs initialization without 'skipDevices'.
	// This means we never explicitly put a container into a cgroup without a
	// set 'devices' path.
	devicePath, ok := subPaths["devices"]
	if !ok {
		return ""
	}

	// Take the piece before the first occurrence of "devices" as the root.
	// I.E. /sys/fs/cgroup/devices/singularity/196219 -> /sys/fs/cgroup
	pathParts := strings.Split(devicePath, "devices")
	if len(pathParts) != 2 {
		return ""
	}

	return filepath.Clean(pathParts[0])
}

// ApplyFromSpec applies a cgroups configuration from an OCI LinuxResources spec
// struct, creating a new group if necessary, and places the process with
// Manager.Pid into the cgroup. The `Unified` key for native v2 cgroup
// specifications is not yet supported.
func (m *ManagerLC) ApplyFromSpec(resources *specs.LinuxResources) (err error) {
	if m.group == "" {
		return fmt.Errorf("path must be specified when creating a cgroup")
	}
	if m.pid == 0 {
		return fmt.Errorf("pid must be specified when creating a cgroup")
	}

	spec := &specs.Spec{
		Linux: &specs.Linux{
			CgroupsPath: m.group,
			Resources:   resources,
		},
	}

	opts := &specconv.CreateOpts{
		CgroupName:       m.group,
		UseSystemdCgroup: false,
		RootlessCgroups:  false,
		Spec:             spec,
	}

	lcConfig, err := specconv.CreateCgroupConfig(opts, nil)
	if err != nil {
		return fmt.Errorf("could not create cgroup config: %w", err)
	}

	m.cgroup, err = lcmanager.New(lcConfig)
	if err != nil {
		return fmt.Errorf("while creating cgroup manager: %w", err)
	}

	err = m.cgroup.Apply(m.pid)
	if err != nil {
		return fmt.Errorf("while creating cgroup: %w", err)
	}

	err = m.cgroup.Set(lcConfig.Resources)
	if err != nil {
		return fmt.Errorf("while setting cgroup limits: %w", err)
	}

	return nil
}

// ApplyFromFile applies a cgroup configuration from a toml file, creating a new
// group if necessary, and places the process with Manager.Pid into the cgroup.
// The `Unified` key for native v2 cgroup specifications is not yet supported.
func (m *ManagerLC) ApplyFromFile(path string) error {
	spec, err := readSpecFromFile(path)
	if err != nil {
		return err
	}
	return m.ApplyFromSpec(&spec)
}

// UpdateFromSpec updates the existing managed cgroup using configuration from
// an OCI LinuxResources spec struct. The `Unified` key for native v2 cgroup
// specifications is not yet supported.
func (m *ManagerLC) UpdateFromSpec(resources *specs.LinuxResources) (err error) {
	if m.cgroup == nil {
		err = m.load()
		if err != nil {
			return fmt.Errorf("while creating cgroup manager: %w", err)
		}
	}
	if m.group == "" {
		return fmt.Errorf("cgroup path not set on manager, cannot update")
	}

	spec := &specs.Spec{
		Linux: &specs.Linux{
			CgroupsPath: m.group,
			Resources:   resources,
		},
	}

	opts := &specconv.CreateOpts{
		CgroupName:       m.group,
		UseSystemdCgroup: false,
		RootlessCgroups:  false,
		Spec:             spec,
	}

	lcConfig, err := specconv.CreateCgroupConfig(opts, nil)
	if err != nil {
		return fmt.Errorf("could not create cgroup config: %w", err)
	}

	err = m.cgroup.Set(lcConfig.Resources)
	if err != nil {
		return fmt.Errorf("while setting cgroup limits: %w", err)
	}

	return nil
}

// UpdateFromFile updates the existing managed cgroup using configuration
// from a toml file.
func (m *ManagerLC) UpdateFromFile(path string) error {
	spec, err := readSpecFromFile(path)
	if err != nil {
		return err
	}
	return m.UpdateFromSpec(&spec)
}

// Remove deletes the managed cgroup.
func (m *ManagerLC) Remove() (err error) {
	if m.cgroup == nil {
		if err := m.load(); err != nil {
			return err
		}
	}
	return m.cgroup.Destroy()
}

func (m *ManagerLC) AddProc(pid int) (err error) {
	if pid == 0 {
		return fmt.Errorf("cannot add a zero pid to cgroup")
	}
	if m.cgroup == nil {
		if err := m.load(); err != nil {
			return err
		}
	}
	return m.cgroup.Apply(pid)
}

// Pause freezes processes in the managed cgroup.
func (m *ManagerLC) Pause() (err error) {
	if m.cgroup == nil {
		if err := m.load(); err != nil {
			return err
		}
	}
	return m.cgroup.Freeze(configs.Frozen)
}

// Resume unfreezes process in the managed cgroup.
func (m *ManagerLC) Resume() (err error) {
	if m.cgroup == nil {
		if err := m.load(); err != nil {
			return err
		}
	}
	return m.cgroup.Freeze(configs.Thawed)
}
