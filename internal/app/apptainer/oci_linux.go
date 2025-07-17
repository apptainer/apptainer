// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/apptainer/apptainer/internal/pkg/instance"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/oci"
	"github.com/apptainer/apptainer/pkg/ociruntime"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// OciArgs contains CLI arguments
type OciArgs struct {
	BundlePath     string
	LogPath        string
	LogFormat      string
	SyncSocketPath string
	PidFile        string
	FromFile       string
	KillSignal     string
	KillTimeout    uint32
	EmptyProcess   bool
	ForceKill      bool
}

func getCommonConfig(containerID string) (*config.Common, error) {
	commonConfig := config.Common{
		EngineConfig: &oci.EngineConfig{},
	}

	file, err := instance.Get(containerID, instance.OciSubDir)
	if err != nil {
		return nil, fmt.Errorf("no container found with name %s", containerID)
	}

	if err := json.Unmarshal(file.Config, &commonConfig); err != nil {
		return nil, fmt.Errorf("failed to read %s container configuration: %s", containerID, err)
	}

	return &commonConfig, nil
}

func getEngineConfig(containerID string) (*oci.EngineConfig, error) {
	commonConfig, err := getCommonConfig(containerID)
	if err != nil {
		return nil, err
	}
	return commonConfig.EngineConfig.(*oci.EngineConfig), nil
}

func getState(containerID string) (*ociruntime.State, error) {
	engineConfig, err := getEngineConfig(containerID)
	if err != nil {
		return nil, err
	}
	return &engineConfig.State, nil
}

func exitContainer(ctx context.Context, containerID string, deleteRes bool) {
	state, err := getState(containerID)
	if err != nil {
		if !deleteRes {
			sylog.Errorf("%s", err)
			os.Exit(1)
		}
		return
	}

	if state.ExitCode != nil {
		defer os.Exit(*state.ExitCode)
	}

	if deleteRes {
		if err := OciDelete(ctx, containerID); err != nil {
			sylog.Errorf("%s", err)
		}
	}
}
