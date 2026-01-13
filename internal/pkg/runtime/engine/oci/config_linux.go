// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oci

import (
	"sync"

	"github.com/apptainer/apptainer/internal/pkg/cgroups"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci"
	"github.com/apptainer/apptainer/pkg/ociruntime"
)

// Name of the engine.
const Name = "oci"

// EngineConfig is the config for the OCI engine.
type EngineConfig struct {
	BundlePath     string           `json:"bundlePath"`
	LogPath        string           `json:"logPath"`
	LogFormat      string           `json:"logFormat"`
	PidFile        string           `json:"pidFile"`
	OciConfig      *oci.Config      `json:"ociConfig"`
	MasterPts      int              `json:"masterPts"`
	SlavePts       int              `json:"slavePts"`
	OutputStreams  [2]int           `json:"outputStreams"`
	ErrorStreams   [2]int           `json:"errorStreams"`
	InputStreams   [2]int           `json:"inputStreams"`
	SyncSocket     string           `json:"syncSocket"`
	EmptyProcess   bool             `json:"emptyProcess"`
	Exec           bool             `json:"exec"`
	SystemdCgroups bool             `json:"systemdCgroups"`
	Cgroups        *cgroups.Manager `json:"-"`
	Devices        []string         `json:"devices"`
	CdiDirs        []string         `json:"cdiDirs"`

	sync.Mutex `json:"-"`
	State      ociruntime.State `json:"state"`
}

// NewConfig returns an oci.EngineConfig.
func NewConfig() *EngineConfig {
	ret := &EngineConfig{
		OciConfig: &oci.Config{},
	}

	return ret
}

// SetBundlePath sets the container bundle path.
func (e *EngineConfig) SetBundlePath(path string) {
	e.BundlePath = path
}

// GetBundlePath returns the container bundle path.
func (e *EngineConfig) GetBundlePath() string {
	return e.BundlePath
}

// SetState sets the container state as defined by OCI state specification.
func (e *EngineConfig) SetState(state *ociruntime.State) {
	e.State = *state
}

// GetState returns the container state as defined by OCI state specification.
func (e *EngineConfig) GetState() *ociruntime.State {
	return &e.State
}

// SetLogPath sets the container log path.
func (e *EngineConfig) SetLogPath(path string) {
	e.LogPath = path
}

// GetLogPath returns the container log path.
func (e *EngineConfig) GetLogPath() string {
	return e.LogPath
}

// SetLogFormat sets the container log format.
func (e *EngineConfig) SetLogFormat(format string) {
	e.LogFormat = format
}

// GetLogFormat returns the container log format.
func (e *EngineConfig) GetLogFormat() string {
	return e.LogFormat
}

// SetPidFile sets the pid file path.
func (e *EngineConfig) SetPidFile(path string) {
	e.PidFile = path
}

// GetPidFile gets the pid file path.
func (e *EngineConfig) GetPidFile() string {
	return e.PidFile
}

// SetSystemdCgroups sets whether to manage cgroups with systemd.
func (e *EngineConfig) SetSystemdCgroups(systemd bool) {
	e.SystemdCgroups = systemd
}

// SetSystemdCgroups gets whether to manage cgroups with systemd.
func (e *EngineConfig) GetSystemdCgroups() bool {
	return e.SystemdCgroups
}

// SetDevices sets the list of fully-qualified CDI device names.
func (e *EngineConfig) SetDevices(devices []string) {
	e.Devices = devices
}

// GetDevices returns the list of fully-qualified CDI device names.
func (e *EngineConfig) GetDevices() []string {
	return e.Devices
}

// SetCdiDirs sets the list of directories in which CDI should look for device definition JSON files.
func (e *EngineConfig) SetCdiDirs(dirs []string) {
	e.CdiDirs = dirs
}

// GetCdiDirs returns the list of directories in which CDI should look for device definition JSON files.
func (e *EngineConfig) GetCdiDirs() []string {
	return e.CdiDirs
}
