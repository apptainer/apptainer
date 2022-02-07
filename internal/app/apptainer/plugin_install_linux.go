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
	"github.com/apptainer/apptainer/internal/pkg/plugin"
)

// InstallPlugin takes a plugin located at path and installs it into
// the apptainer plugin installation directory.
//
// Installing a plugin will also automatically enable it.
func InstallPlugin(pluginPath string) error {
	return plugin.Install(pluginPath)
}
