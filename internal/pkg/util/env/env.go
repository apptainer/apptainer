// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package env

import (
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultPath defines default value for PATH environment variable.
	DefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	// ApptainerPrefix Apptainer environment variable recognized prefixes for Apptainer CLI
	ApptainerPrefix = "APPTAINER_"

	// ApptainerEnvPrefix Apptainer environment variables recognized prefixes for passthru to container
	ApptainerEnvPrefix = "APPTAINERENV_"
)

// ApptainerPrefixes the following prefixes are for settings looked at by Apptainer command
var ApptainerPrefixes = []string{ApptainerPrefix, "SINGULARITY_"}

// ApptainerEnvPrefixes defines the environment variable prefixes for passthru
// to container
var ApptainerEnvPrefixes = []string{ApptainerEnvPrefix, "SINGULARITYENV_"}

// SetFromList sets environment variables from environ argument list.
func SetFromList(environ []string) error {
	for _, env := range environ {
		splitted := strings.SplitN(env, "=", 2)
		if len(splitted) != 2 {
			return fmt.Errorf("can't process environment variable %s", env)
		}
		if err := os.Setenv(splitted[0], splitted[1]); err != nil {
			return err
		}
	}
	return nil
}
