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
	"strings"

	"github.com/apptainer/apptainer/pkg/util/capabilities"
)

// CapAvailConfig instructs CapabilityAvail on what capability to list/describe
type CapAvailConfig struct {
	Caps string
	Desc bool
}

// CapabilityAvail lists the capabilities based on the CapAvailConfig
func CapabilityAvail(c CapAvailConfig) error {
	caps, ign := capabilities.Split(c.Caps)
	if len(ign) > 0 {
		return fmt.Errorf("unknown capabilities found in: %s", strings.Join(ign, ","))
	}

	if len(caps) > 0 {
		for _, capability := range caps {
			fmt.Printf("%-22s %s\n\n", capability+":", capabilities.Map[capability].Description)
		}
		return nil
	}

	for k := range capabilities.Map {
		fmt.Printf("%-22s %s\n\n", k+":", capabilities.Map[k].Description)
	}
	return nil
}
