// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"fmt"

	"github.com/apptainer/apptainer/internal/pkg/plugin"
)

// InspectPlugin inspects the named plugin.
func InspectPlugin(name string) error {
	manifest, err := plugin.Inspect(name)
	if err != nil {
		return err
	}

	fmt.Printf("Name: %s\n"+
		"Description: %s\n"+
		"Author: %s\n"+
		"Version: %s\n",
		manifest.Name,
		manifest.Description,
		manifest.Author,
		manifest.Version)

	return nil
}
