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
	"fmt"
	"os"
	"strings"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/oci"
	"github.com/apptainer/apptainer/internal/pkg/util/starter"
	"github.com/apptainer/apptainer/pkg/ociruntime"
)

// OciExec executes a command in a container
func OciExec(containerID string, cmdArgs []string) error { //nolint:staticcheck
	commonConfig, err := getCommonConfig(containerID)
	if err != nil {
		return fmt.Errorf("%s doesn't exist", containerID)
	}

	engineConfig := commonConfig.EngineConfig.(*oci.EngineConfig)

	switch engineConfig.GetState().Status {
	case ociruntime.Running, ociruntime.Paused:
	default:
		args := strings.Join(cmdArgs, " ")
		return fmt.Errorf("cannot execute command %q, container '%s' is not running", args, containerID)
	}

	engineConfig.Exec = true
	engineConfig.OciConfig.SetProcessArgs(cmdArgs)

	os.Clearenv()

	procName := fmt.Sprintf("Apptainer OCI %s", containerID)
	return starter.Exec(procName, commonConfig)
}
