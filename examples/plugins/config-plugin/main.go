// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"log"

	"github.com/apptainer/apptainer/internal/pkg/cgroups"
	pluginapi "github.com/apptainer/apptainer/pkg/plugin"
	clicallback "github.com/apptainer/apptainer/pkg/plugin/callback/cli"
	apptainer "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
	"github.com/apptainer/apptainer/pkg/sylog"
)

// Plugin is the only variable which a plugin MUST export.
// This symbol is accessed by the plugin framework to initialize the plugin
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "example.com/config-plugin",
		Author:      "Apptainer Team",
		Version:     "0.1.0",
		Description: "This is a short example config plugin for Apptainer",
	},
	Callbacks: []pluginapi.Callback{
		(clicallback.ApptainerEngineConfig)(callbackCgroups),
	},
}

func callbackCgroups(common *config.Common) {
	c, ok := common.EngineConfig.(*apptainer.EngineConfig)
	if !ok {
		log.Printf("Unexpected engine config")
		return
	}
	cfg := cgroups.Config{
		Devices: nil,
		Memory: &cgroups.LinuxMemory{
			Limit: &[]int64{1024 * 1}[0],
		},
	}

	data, err := cfg.MarshalJSON()
	if err != nil {
		sylog.Errorf("While Marshaling cgroups config to JSON: %s", err)
		return
	}
	sylog.Infof("Overriding cgroups config")
	c.SetCgroupsJSON(data)
}
