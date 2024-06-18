// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oci

import (
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine"
	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/apptainer/rpc/server"
	ociServer "github.com/apptainer/apptainer/internal/pkg/runtime/engine/oci/rpc/server"
	"github.com/apptainer/apptainer/pkg/runtime/engine/config"
)

// EngineOperations is an Apptainer OCI runtime engine that implements engine.Operations.
// Basically, this is the core of `apptainer oci` commands.
type EngineOperations struct {
	CommonConfig *config.Common `json:"-"`
	EngineConfig *EngineConfig  `json:"engineConfig"`
}

// InitConfig stores the parsed config.Common inside the engine.
//
// Since this method simply stores config.Common, it does not matter
// whether or not there are any elevated privileges during this call.
func (e *EngineOperations) InitConfig(cfg *config.Common, _ bool) {
	e.CommonConfig = cfg
}

// Config returns a pointer to EngineConfig literal as a config.EngineConfig
// interface. This pointer gets stored in the Engine.Common field.
//
// Since this method simply returns a zero value of the concrete
// EngineConfig, it does not matter whether or not there are any elevated
// privileges during this call.
func (e *EngineOperations) Config() config.EngineConfig {
	return e.EngineConfig
}

func init() {
	engine.RegisterOperations(
		Name,
		&EngineOperations{
			EngineConfig: &EngineConfig{},
		},
	)

	ocimethods := new(ociServer.Methods)
	ocimethods.Methods = new(server.Methods)
	engine.RegisterRPCMethods(
		Name,
		ocimethods,
	)
}
