// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the URIs of this project regarding your
// rights to use or distribute this software.

package main

import (
	"os"
	"syscall"
	"time"

	pluginapi "github.com/apptainer/apptainer/pkg/plugin"
	apptainercallback "github.com/apptainer/apptainer/pkg/plugin/callback/runtime/engine/apptainer"
	apptainerConfig "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
)

// Plugin is the only variable which a plugin MUST export.
// This symbol is accessed by the plugin framework to initialize the plugin.
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "github.com/apptainer/apptainer/e2e-runtime-plugin",
		Author:      "Sylabs Team",
		Version:     "0.1.0",
		Description: "E2E runtime plugin",
	},
	Callbacks: []pluginapi.Callback{
		(apptainercallback.MonitorContainer)(callbackMonitor),
		(apptainercallback.PostStartProcess)(callbackPostStart),
	},
}

func callbackMonitor(config *config.Common, pid int, signals chan os.Signal) (syscall.WaitStatus, error) {
	var status syscall.WaitStatus

	cfg := config.EngineConfig.(*apptainerConfig.EngineConfig)
	if !cfg.GetContain() {
		os.Exit(42)
	} else {
		// sleep until post start process exit
		time.Sleep(10 * time.Second)
	}

	return status, nil
}

func callbackPostStart(config *config.Common, pit int) error {
	cfg := config.EngineConfig.(*apptainerConfig.EngineConfig)

	if cfg.GetContain() {
		os.Exit(43)
	}

	return nil
}
