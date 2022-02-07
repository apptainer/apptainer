// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package dmtcp

import (
	"path/filepath"

	apptainerConfig "github.com/apptainer/apptainer/pkg/runtime/engine/apptainer/config"
)

func InjectArgs(config apptainerConfig.DMTCPConfig, argv []string) []string {
	// On restart we run a restart script and do not care about user defined arguments
	// as the container process is executing with state generated from the args at launch time.
	if config.Restart {
		return config.Args
	}

	// On launch, we inject dmtcp_launch to execute the container script with specified arguments.
	return append(config.Args, argv...)
}

func LaunchArgs() []string {
	return []string{
		"dmtcp_launch",
		"--coord-port",
		"0",
		"--coord-logfile",
		filepath.Join(containerStatepath, logFile),
		"--port-file",
		filepath.Join(containerStatepath, portFile),
		"--ckptdir",
		containerStatepath,
		"--no-gzip",
		"--ckpt-open-files",
	}
}

func RestartArgs() []string {
	return []string{
		filepath.Join(containerStatepath, "dmtcp_restart_script.sh"),
	}
}

func CheckpointArgs(coordinatorPort string) []string {
	return []string{
		"dmtcp_command",
		"--coord-port",
		coordinatorPort,
		"-c",
	}
}
