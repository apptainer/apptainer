// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build !seccomp

package seccomp

import (
	"fmt"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Enabled returns whether seccomp is enabled.
func Enabled() bool {
	return false
}

// LoadSeccompConfig loads seccomp configuration filter for the current process.
func LoadSeccompConfig(_ *specs.LinuxSeccomp, _ bool, _ int16) error {
	return fmt.Errorf("can't load seccomp filter: not enabled at compilation time")
}

// LoadProfileFromFile loads seccomp rules from json file and fill in provided OCI configuration.
func LoadProfileFromFile(_ string, generator *generate.Generator) error {
	if generator.Config.Linux == nil {
		generator.Config.Linux = &specs.Linux{}
	}
	if generator.Config.Linux.Seccomp == nil {
		generator.Config.Linux.Seccomp = &specs.LinuxSeccomp{}
	}
	return nil
}
