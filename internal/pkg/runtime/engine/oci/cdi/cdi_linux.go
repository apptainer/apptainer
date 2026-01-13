// Copyright (c) Contributors to the Apptainer project, established as
//
//	Apptainer a Series of LF Projects LLC.
//	For website terms of use, trademark policy, privacy policy and other
//	project policies see https://lfprojects.org/policies
//
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.
package cdi

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
	cdilib "tags.cncf.io/container-device-interface/pkg/cdi"
	"tags.cncf.io/container-device-interface/pkg/parser"
)

func AddCdiDevices(spec *specs.Spec, cdiDevices []string, cdiDirs []string, cdiOptions ...cdilib.Option) error {
	// if custom CDI directories are specified, configure them
	if len(cdiDirs) > 0 {
		if err := cdilib.Configure(cdilib.WithSpecDirs(cdiDirs...)); err != nil {
			return fmt.Errorf("error configuring CDI cache with custom directories: %w", err)
		}
	} else if len(cdiOptions) > 0 {
		// if the cdiOptions are not empty, cdi will configure and refresh the cache accordingly
		if err := cdilib.Configure(cdiOptions...); err != nil {
			return fmt.Errorf("error configuring CDI cache: %w", err)
		}
	}

	for _, cdiDevice := range cdiDevices {
		// not a valid cdi device name
		if !parser.IsQualifiedName(cdiDevice) {
			return fmt.Errorf("cdiDevice %s is not valid", cdiDevice)
		}
	}

	if _, err := cdilib.InjectDevices(spec, cdiDevices...); err != nil {
		return fmt.Errorf("while setting up CDI devices: %w", err)
	}

	return nil
}
