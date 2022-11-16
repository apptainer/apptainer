// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&instanceStartPidFileFlag, instanceStartCmd)
		cmdManager.RegisterFlagForCmd(&actionDMTCPLaunchFlag, instanceStartCmd)
		cmdManager.RegisterFlagForCmd(&actionDMTCPRestartFlag, instanceStartCmd)
	})
}

// --pid-file
var instanceStartPidFile string

var instanceStartPidFileFlag = cmdline.Flag{
	ID:           "instanceStartPidFileFlag",
	Value:        &instanceStartPidFile,
	DefaultValue: "",
	Name:         "pid-file",
	Usage:        "write instance PID to the file with the given name",
	EnvKeys:      []string{"PID_FILE"},
}

// apptainer instance start
var instanceStartCmd = &cobra.Command{
	Args:                  cobra.MinimumNArgs(2),
	PreRun:                actionPreRun,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		image := args[0]
		name := args[1]

		a := append([]string{"/.singularity.d/actions/start"}, args[2:]...)
		setVM(cmd)
		if vm {
			execVM(cmd, image, a)
			return
		}
		if err := launchContainer(cmd, image, a, name); err != nil {
			sylog.Fatalf("%s", err)
		}

		if instanceStartPidFile != "" {
			err := apptainer.WriteInstancePidFile(name, instanceStartPidFile)
			if err != nil {
				sylog.Warningf("Failed to write pid file: %v", err)
			}
		}
	},

	Use:     docs.InstanceStartUse,
	Short:   docs.InstanceStartShort,
	Long:    docs.InstanceStartLong,
	Example: docs.InstanceStartExample,
}
