// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"errors"
	"fmt"
	"os"

	"github.com/apptainer/apptainer/internal/pkg/plugin"
)

var errPluginNotFound = errors.New("plugin not found")

// UninstallPlugin removes the named plugin from the system.
func UninstallPlugin(name string) error {
	err := plugin.Uninstall(name)
	if errors.Is(err, os.ErrNotExist) {
		return errPluginNotFound
	}
	if err != nil {
		return fmt.Errorf("could not uninstall plugin: %w", err)
	}
	return nil
}
