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
	"encoding/json"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// Manager is used to work with cgroups resource restrictions. It is an
// interface satisfied by different implementations for v1 and v2 cgroups.
type Manager interface {
	// GetCgroupRootPath returns the path to the root of the cgroup on the
	// filesystem.
	GetCgroupRootPath() string
	// ApplyFromFile applies a cgroup configuration from a toml file, creating a
	// new group if necessary, and places the process with Manager.Pid into the
	// cgroup.
	ApplyFromFile(path string) error
	// ApplyFromSpec applies a cgroups configuration from an OCI LinuxResources
	// spec struct, creating a new group if necessary, and places the process
	// with Manager.Pid into the cgroup.
	ApplyFromSpec(spec *specs.LinuxResources) error
	// UpdateFromFile updates the existing managed cgroup using configuration
	// from a toml file.
	UpdateFromFile(path string) error
	// UpdateFromSpec updates the existing managed cgroup using configuration
	// from an OCI LinuxResources spec struct.
	UpdateFromSpec(spec *specs.LinuxResources) error
	// AddProc adds the process with specified pid to the managed cgroup
	AddProc(pid int) error
	// Remove deletes the managed cgroup.
	Remove() error
	// Pause freezes processes in the managed cgroup.
	Pause() error
	// Resume unfreezes process in the managed cgroup.
	Resume() error
}

// NewManagerFromFile creates a Manager, applies the configuration at specPath, and adds pid to the cgroup.
// If a group name is supplied, it will be used by the manager.
// If group = "" then "/apptainer/<pid>" is used as a default.
func NewManagerFromFile(specPath string, pid int, group string) (manager Manager, err error) {
	if group == "" {
		group = filepath.Join("/apptainer", strconv.Itoa(pid))
	}
	mgr := ManagerLC{pid: pid, group: group}
	if err := mgr.ApplyFromFile(specPath); err != nil {
		return nil, err
	}
	return &mgr, err
}

// NewManagerFromSpec creates a Manager, applies the configuration in spec, and adds pid to the cgroup.
// If a group name is supplied, it will be used by the manager.
// If group = "" then "/apptainer/<pid>" is used as a default.
func NewManagerFromSpec(spec *specs.LinuxResources, pid int, group string) (manager Manager, err error) {
	if group == "" {
		group = filepath.Join("/apptainer", strconv.Itoa(pid))
	}

	mgr := ManagerLC{pid: pid, group: group}
	if err := mgr.ApplyFromSpec(spec); err != nil {
		return nil, err
	}
	return &mgr, err
}

// GetManager returns a Manager for the provided cgroup name/path.
func GetManager(group string) (manager Manager, err error) {
	mgr := ManagerLC{group: group}
	if err := mgr.load(); err != nil {
		return nil, err
	}
	return &mgr, nil
}

// GetManagerFromPid returns a Manager for the cgroup that pid is a member of.
func GetManagerFromPid(pid int) (manager Manager, err error) {
	mgr := ManagerLC{pid: pid}
	if err := mgr.load(); err != nil {
		return nil, err
	}
	return &mgr, nil
}

// readSpecFromFile loads a TOML file containing a specs.LinuxResources cgroups configuration.
func readSpecFromFile(path string) (spec specs.LinuxResources, err error) {
	conf, err := LoadConfig(path)
	if err != nil {
		return
	}

	// convert TOML structures to OCI JSON structures
	data, err := json.Marshal(conf)
	if err != nil {
		return
	}

	if err = json.Unmarshal(data, &spec); err != nil {
		return
	}

	return
}
