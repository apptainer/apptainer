// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/apptainer/rpc/server"
	apptainerConfig "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

// EngineOperations is an Apptainer runtime engine that implements engine.Operations.
// Basically, this is the core of `apptainer run/exec/shell/instance` commands.
type EngineOperations struct {
	CommonConfig *config.Common                `json:"-"`
	EngineConfig *apptainerConfig.EngineConfig `json:"engineConfig"`
}

// InitConfig stores the parsed config.Common inside the engine.
// If privStageOne is true, re-parse the configuration file
func (e *EngineOperations) InitConfig(cfg *config.Common, privStageOne bool) {
	e.CommonConfig = cfg
	if privStageOne {
		// override the contents of File for security reasons
		var err error
		e.EngineConfig.File, err = apptainerconf.Parse(buildcfg.APPTAINER_CONF_FILE)
		if err != nil {
			sylog.Fatalf("unable to parse apptainer.conf file: %s", err)
		}
		if e.EngineConfig.GetUseBuildConfig() {
			// Note that this is different from what is seen by
			//  the unprivileged cli code, because that gets based
			//  on a default configuration instead of the system
			//  configuration.  We can't do that here because it
			//  could bypass restrictions the system administrator
			//  has defined.
			sylog.Debugf("Applying build configuration on system configuration")
			apptainerconf.ApplyBuildConfig(e.EngineConfig.File)
		}
		apptainerconf.SetCurrentConfig(e.EngineConfig.File)
		apptainerconf.SetBinaryPath(buildcfg.LIBEXECDIR, false)
	} else {
		// use the configuration passed in
		apptainerconf.SetCurrentConfig(e.EngineConfig.File)
	}
}

// Config returns a pointer to an apptainerConfig.EngineConfig
// literal as a config.EngineConfig interface. This pointer
// gets stored in the engine.Engine.Common field.
//
// Since this method simply returns a zero value of the concrete
// EngineConfig, it does not matter whether or not there are any elevated
// privileges during this call.
func (e *EngineOperations) Config() config.EngineConfig {
	return e.EngineConfig
}

func init() {
	engine.RegisterOperations(
		apptainerConfig.Name,
		&EngineOperations{
			EngineConfig: apptainerConfig.NewConfig(),
		},
	)

	engine.RegisterRPCMethods(
		apptainerConfig.Name,
		new(server.Methods),
	)
}
