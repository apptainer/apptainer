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
		cmdManager.RegisterFlagForCmd(&instanceStartPidFileFlag, instanceStartCmd, instanceRunCmd)
		cmdManager.RegisterFlagForCmd(&actionDMTCPLaunchFlag, instanceStartCmd, instanceRunCmd)
		cmdManager.RegisterFlagForCmd(&actionDMTCPRestartFlag, instanceStartCmd, instanceRunCmd)
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

// execute either the instance start or run command
func instanceAction(cmd *cobra.Command, args []string) {
	image := args[0]
	name := args[1]
	cmdName := cmd.Name()
	script := "start"
	killCont := ""

	if cmdName == "run" {
		script = "run"
		killCont = "/bin/sh -c kill -CONT 1; "
	}
	a := append([]string{killCont + "/.singularity.d/actions/" + script}, args[2:]...)
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
}

// apptainer instance start
var instanceStartCmd = &cobra.Command{
	Args:                  cobra.MinimumNArgs(2),
	PreRun:                actionPreRun,
	DisableFlagsInUseLine: true,
	Run:                   instanceAction,
	Use:                   docs.InstanceStartUse,
	Short:                 docs.InstanceStartShort,
	Long:                  docs.InstanceStartLong,
	Example:               docs.InstanceStartExample,
}

// apptainer instance run
var instanceRunCmd = &cobra.Command{
	Args:                  cobra.MinimumNArgs(2),
	PreRun:                actionPreRun,
	DisableFlagsInUseLine: true,
	Run:                   instanceAction,
	Use:                   docs.InstanceRunUse,
	Short:                 docs.InstanceRunShort,
	Long:                  docs.InstanceRunLong,
	Example:               docs.InstanceRunExample,
}
