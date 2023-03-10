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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	lccgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	lcmanager "github.com/opencontainers/runc/libcontainer/cgroups/manager"
	lcconfigs "github.com/opencontainers/runc/libcontainer/configs"
	lcspecconv "github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var ErrUnitialized = errors.New("cgroups manager is not initialized")

// Manager provides functions to modify, freeze, thaw, and destroy a cgroup.
// Apptainer's cgroups.Manager is a wrapper around runc/libcontainer/cgroups.
// The manager supports v1 cgroups, and v2 cgroups with a unified hierarchy.
// Resource specifications are handles in specs.LinuxResources format and
// translated to runc/libcontainer/cgroups format internally.
type Manager struct {
	// The name of the cgroup
	group string
	// Are we using systemd?
	systemd bool
	// The underlying runc/libcontainer/cgroups manager
	cgroup lccgroups.Manager
}

// GetCgroupRootPath returns the cgroups mount root path, for the managed cgroup
func (m *Manager) GetCgroupRootPath() (rootPath string, err error) {
	if m.group == "" || m.cgroup == nil {
		return "", ErrUnitialized
	}

	// v2 - has a single fixed mountpoint for the root cgroup
	if lccgroups.IsCgroup2UnifiedMode() {
		return unifiedMountPoint, nil
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
		return "", fmt.Errorf("could not find devices controller path")
	}

	// Take the piece before the first occurrence of "devices" as the root.
	// I.E. /sys/fs/cgroup/devices/apptainer/196219 -> /sys/fs/cgroup
	pathParts := strings.SplitN(devicePath, "devices", 2)
	if len(pathParts) != 2 {
		return "", fmt.Errorf("could not find devices controller path")
	}

	return filepath.Clean(pathParts[0]), nil
}

// GetCgroupRelPath returns the relative path of the cgroup under the mount point
func (m *Manager) GetCgroupRelPath() (relPath string, err error) {
	if m.group == "" || m.cgroup == nil {
		return "", ErrUnitialized
	}

	// v2 - has a single fixed mountpoint for the root cgroup
	if lccgroups.IsCgroup2UnifiedMode() {
		absPath := m.cgroup.Path("")
		return strings.TrimPrefix(absPath, unifiedMountPoint), nil
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
		return "", fmt.Errorf("could not find devices controller path")
	}

	// Take the piece after the first occurrence of "devices" as the relative path.
	// I.E. /sys/fs/cgroup/devices/apptainer/196219 -> /apptainer/196219
	pathParts := strings.SplitN(devicePath, "devices", 2)
	if len(pathParts) != 2 {
		return "", fmt.Errorf("could not find devices controller path")
	}

	return filepath.Clean(pathParts[1]), nil
}

// GetStats wraps the Manager.GetStats from runc
func (m *Manager) GetStats() (*lccgroups.Stats, error) {
	stats, err := m.cgroup.GetStats()
	if err != nil {
		return &lccgroups.Stats{}, fmt.Errorf("could not get stats from cgroups manager: %x", err)
	}
	return stats, nil
}

// UpdateFromSpec updates the existing managed cgroup using configuration from
// an OCI LinuxResources spec struct.
func (m *Manager) UpdateFromSpec(resources *specs.LinuxResources) (err error) {
	if m.group == "" || m.cgroup == nil {
		return ErrUnitialized
	}

	spec := &specs.Spec{
		Linux: &specs.Linux{
			CgroupsPath: m.group,
			Resources:   resources,
		},
	}

	opts := &lcspecconv.CreateOpts{
		CgroupName:       m.group,
		UseSystemdCgroup: false,
		RootlessCgroups:  os.Getuid() != 0,
		Spec:             spec,
	}

	lcConfig, err := lcspecconv.CreateCgroupConfig(opts, nil)
	if err != nil {
		return fmt.Errorf("could not create cgroup config: %w", err)
	}

	// runc/libcontainer/cgroups defaults to a deny-all policy, while
	// apptainer has always allowed access to devices by default. If no device
	// rules are provided in the spec, then skip setting them so the deny-all is
	// not applied when we update the cgroup.
	if len(resources.Devices) == 0 {
		lcConfig.SkipDevices = true
	}

	err = m.cgroup.Set(lcConfig.Resources)
	if err != nil {
		return fmt.Errorf("while setting cgroup limits: %w", err)
	}

	return nil
}

// UpdateFromFile updates the existing managed cgroup using configuration
// from a toml file.
func (m *Manager) UpdateFromFile(path string) error {
	spec, err := LoadResources(path)
	if err != nil {
		return fmt.Errorf("while loading cgroups file %s: %w", path, err)
	}
	return m.UpdateFromSpec(&spec)
}

// AddProc adds the process with specified pid to the managed cgroup
//
// Disable context check as it raises a warning throuch lcmanager.New, which is
// in a dependency we cannot modify to pass a context.
//
// nolint:contextcheck
func (m *Manager) AddProc(pid int) (err error) {
	if m.group == "" || m.cgroup == nil {
		return ErrUnitialized
	}
	if pid == 0 {
		return fmt.Errorf("cannot add a zero pid to cgroup")
	}

	// If we are managing cgroupfs directly we are good to go.
	procMgr := m.cgroup
	// However, the systemd manager won't put another process in the cgroup...
	// so we use an underlying cgroupfs manager for this particular operation.
	if m.systemd {
		relPath, err := m.GetCgroupRelPath()
		if err != nil {
			return err
		}
		lcConfig := &lcconfigs.Cgroup{
			Path:      relPath,
			Resources: &lcconfigs.Resources{},
			Systemd:   false,
		}
		procMgr, err = lcmanager.New(lcConfig)
		if err != nil {
			return fmt.Errorf("while creating cgroupfs manager: %w", err)
		}
	}

	return procMgr.Apply(pid)
}

// Freeze freezes processes in the managed cgroup.
func (m *Manager) Freeze() (err error) {
	if m.group == "" || m.cgroup == nil {
		return ErrUnitialized
	}
	return m.cgroup.Freeze(lcconfigs.Frozen)
}

// Thaw unfreezes process in the managed cgroup.
func (m *Manager) Thaw() (err error) {
	if m.group == "" || m.cgroup == nil {
		return ErrUnitialized
	}
	return m.cgroup.Freeze(lcconfigs.Thawed)
}

// Destroy deletes the managed cgroup.
func (m *Manager) Destroy() (err error) {
	if m.group == "" || m.cgroup == nil {
		return ErrUnitialized
	}
	return m.cgroup.Destroy()
}

// checkRootless identifies if rootless cgroups are required / supported
func checkRootless(group string, systemd bool) (rootless bool, err error) {
	if os.Getuid() == 0 {
		if systemd {
			if !strings.HasPrefix(group, "system.slice:") {
				return false, fmt.Errorf("systemd cgroups require a cgroups path beginning with 'system.slice:'")
			}
		}
		return false, nil
	}

	if !cgroups.IsCgroup2HybridMode() && !cgroups.IsCgroup2UnifiedMode() {
		return false, fmt.Errorf("rootless cgroups requires cgroups v2")
	}
	if !systemd {
		return false, fmt.Errorf("rootless cgroups require 'systemd cgroups' to be enabled in apptainer.conf")
	}
	if os.Getenv("XDG_RUNTIME_DIR") == "" || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return false, fmt.Errorf("rootless cgroups require a D-Bus session - check that XDG_RUNTIME_DIR and DBUS_SESSION_BUS_ADDRESS are set")
	}

	if !strings.HasPrefix(group, "user.slice:") {
		return false, fmt.Errorf("rootless cgroups require a cgroups path beginning with 'user.slice:'")
	}

	return true, nil
}

// newManager creates a new Manager, with the associated resources and cgroup.
// The Manager is ready to manage the cgroup but does not apply limits etc.
//
// Disable context check as it raises a warning throuch lcmanager.New, which is
// in a dependency we cannot modify to pass a context.
//
// nolint:contextcheck
func newManager(resources *specs.LinuxResources, group string, systemd bool) (manager *Manager, err error) {
	if resources == nil {
		return nil, fmt.Errorf("non-nil cgroup LinuxResources definition is required")
	}
	if group == "" {
		return nil, fmt.Errorf("a cgroup name/path is required")
	}

	rootless, err := checkRootless(group, systemd)
	if err != nil {
		return nil, err
	}
	// Rootless manager code invokes systemctl, which it expects to be on PATH.
	// Must set default PATH as starter sets up a very stripped down environment.
	if rootless {
		sylog.Debugf("Using rootless cgroups")
		oldPath := os.Getenv("PATH")
		if err := os.Setenv("PATH", env.DefaultPath); err != nil {
			return nil, fmt.Errorf("could not set default PATH for cgroups manager to locate systemctl: %w", err)
		}
		defer os.Setenv("PATH", oldPath)

		if len(resources.Devices) > 0 {
			sylog.Warningf("Device limits will not be applied with rootless cgroups")
		}
	}

	spec := &specs.Spec{
		Linux: &specs.Linux{
			CgroupsPath: group,
			Resources:   resources,
		},
	}

	opts := &lcspecconv.CreateOpts{
		CgroupName:       group,
		UseSystemdCgroup: systemd,
		RootlessCgroups:  rootless,
		Spec:             spec,
	}

	lcConfig, err := lcspecconv.CreateCgroupConfig(opts, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create cgroup config: %w", err)
	}

	// runc/libcontainer/cgroups defaults to a deny-all policy, while
	// apptainer has always allowed access to devices by default.
	if len(resources.Devices) == 0 {
		resources.Devices = []specs.LinuxDeviceCgroup{
			{
				Allow:  true,
				Access: "rwm",
			},
		}
	}

	cgroup, err := lcmanager.New(lcConfig)
	if err != nil {
		return nil, fmt.Errorf("while creating cgroup manager: %w", err)
	}

	mgr := Manager{
		group:   group,
		systemd: systemd,
		cgroup:  cgroup,
	}
	return &mgr, nil
}

// NewManagerWithSpec creates a Manager, applies the configuration in spec, and adds pid to the cgroup.
// If a group name is supplied, it will be used by the manager.
// If group = "" then "/apptainer/<pid>" is used as a default.
func NewManagerWithSpec(spec *specs.LinuxResources, pid int, group string, systemd bool) (manager *Manager, err error) {
	if pid == 0 {
		return nil, fmt.Errorf("a pid is required to create a new cgroup")
	}
	if group == "" && !systemd {
		group = filepath.Join("/apptainer", strconv.Itoa(pid))
	}
	if group == "" && systemd {
		if os.Getuid() == 0 {
			group = "system.slice:apptainer:" + strconv.Itoa(pid)
		} else {
			group = "user.slice:apptainer:" + strconv.Itoa(pid)
		}
	}

	sylog.Debugf("Creating cgroups manager for %s", group)

	// Create the manager
	mgr, err := newManager(spec, group, systemd)
	if err != nil {
		return nil, err
	}
	// Apply the cgroup to pid (add pid to cgroup)
	if err := mgr.cgroup.Apply(pid); err != nil {
		return nil, err
	}
	if err := mgr.UpdateFromSpec(spec); err != nil {
		return nil, err
	}

	return mgr, nil
}

// NewManagerWithJSON creates a Manager, applies the JSON configuration supplied, and adds pid to the cgroup.
// If a group name is supplied, it will be used by the manager.
// If group = "" then "/apptainer/<pid>" is used as a default.
func NewManagerWithJSON(jsonSpec string, pid int, group string, systemd bool) (manager *Manager, err error) {
	spec, err := UnmarshalJSONResources(jsonSpec)
	if err != nil {
		return nil, fmt.Errorf("while loading cgroups spec: %w", err)
	}
	return NewManagerWithSpec(spec, pid, group, systemd)
}

// NewManagerWithFile creates a Manager, applies the configuration at specPath, and adds pid to the cgroup.
// If a group name is supplied, it will be used by the manager.
// If group = "" then "/apptainer/<pid>" is used as a default.
func NewManagerWithFile(specPath string, pid int, group string, systemd bool) (manager *Manager, err error) {
	spec, err := LoadResources(specPath)
	if err != nil {
		return nil, fmt.Errorf("while loading cgroups spec: %w", err)
	}
	return NewManagerWithSpec(&spec, pid, group, systemd)
}

// GetManager returns a Manager for the provided cgroup name/path.
// It can only return a cgroupfs manager, as we aren't wiring back up to systemd
// through dbus etc.
//
// Disable context check as it raises a warning throuch lcmanager.New, which is
// in a dependency we cannot modify to pass a context.
//
// nolint:contextcheck
func GetManagerForGroup(group string) (manager *Manager, err error) {
	if group == "" {
		return nil, fmt.Errorf("cannot load cgroup - no name/path specified")
	}

	// Create an empty runc/libcontainer/configs resource spec directly.
	// We could call newManager() with an empty LinuxResources spec, but this
	// saves the specconv processing.
	lcConfig := &lcconfigs.Cgroup{
		Path:      group,
		Resources: &lcconfigs.Resources{},
		Systemd:   false,
	}
	cgroup, err := lcmanager.New(lcConfig)
	if err != nil {
		return nil, fmt.Errorf("while creating cgroup manager: %w", err)
	}

	mgr := Manager{
		group:   group,
		systemd: false,
		cgroup:  cgroup,
	}
	return &mgr, nil
}

// GetManagerFromPid returns a Manager for the cgroup that pid is a member of.
// It can only return a cgroupfs manager, as we aren't wiring back up to systemd
// through dbus etc.
func GetManagerForPid(pid int) (manager *Manager, err error) {
	path, err := pidToPath(pid)
	if err != nil {
		return nil, err
	}
	return GetManagerForGroup(path)
}
