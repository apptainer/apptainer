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
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/util/signal"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&instanceStopUserFlag, instanceStopCmd)
		cmdManager.RegisterFlagForCmd(&instanceStopAllFlag, instanceStopCmd)
		cmdManager.RegisterFlagForCmd(&instanceStopForceFlag, instanceStopCmd)
		cmdManager.RegisterFlagForCmd(&instanceStopSignalFlag, instanceStopCmd)
		cmdManager.RegisterFlagForCmd(&instanceStopTimeoutFlag, instanceStopCmd)
	})
}

// -u|--user
var instanceStopUser string

var instanceStopUserFlag = cmdline.Flag{
	ID:           "instanceStopUserFlag",
	Value:        &instanceStopUser,
	DefaultValue: "",
	Name:         "user",
	ShortHand:    "u",
	Usage:        "if running as root, stop instances belonging to user",
	Tag:          "<username>",
	EnvKeys:      []string{"USER"},
}

// -a|--all
var instanceStopAll bool

var instanceStopAllFlag = cmdline.Flag{
	ID:           "instanceStopAllFlag",
	Value:        &instanceStopAll,
	DefaultValue: false,
	Name:         "all",
	ShortHand:    "a",
	Usage:        "stop all user's instances",
	EnvKeys:      []string{"ALL"},
}

// -f|--force
var instanceStopForce bool

var instanceStopForceFlag = cmdline.Flag{
	ID:           "instanceStopForceFlag",
	Value:        &instanceStopForce,
	DefaultValue: false,
	Name:         "force",
	ShortHand:    "F",
	Usage:        "force kill instance",
	EnvKeys:      []string{"FORCE"},
}

// -s|--signal
var instanceStopSignal string

var instanceStopSignalFlag = cmdline.Flag{
	ID:           "instanceStopSignalFlag",
	Value:        &instanceStopSignal,
	DefaultValue: "",
	Name:         "signal",
	ShortHand:    "s",
	Usage:        "signal sent to the instance",
	Tag:          "<signal>",
	EnvKeys:      []string{"SIGNAL"},
}

// -t|--timeout
var instanceStopTimeout int

var instanceStopTimeoutFlag = cmdline.Flag{
	ID:           "instanceStopTimeoutFlag",
	Value:        &instanceStopTimeout,
	DefaultValue: 10,
	Name:         "timeout",
	ShortHand:    "t",
	Usage:        "force kill non stopped instances after X seconds",
}

// apptainer instance stop
var instanceStopCmd = &cobra.Command{
	Args:                  cobra.RangeArgs(0, 1),
	DisableFlagsInUseLine: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 && !instanceStopAll {
			return errors.New("invalid command")
		}

		uid := os.Getuid()
		if instanceStopUser != "" && uid != 0 {
			sylog.Fatalf("Only root user can stop user's instances")
		}

		sig := syscall.SIGINT
		if instanceStopSignal != "" {
			var err error
			sig, err = signal.Convert(instanceStopSignal)
			if err != nil {
				sylog.Fatalf("Could not convert stop signal: %s", err)
			}
		}
		if instanceStopForce {
			sig = syscall.SIGKILL
		}

		name := "*"
		if len(args) > 0 {
			name = args[0]
		}

		timeout := time.Duration(instanceStopTimeout) * time.Second
		return apptainer.StopInstance(name, instanceStopUser, sig, timeout)
	},

	Use:     docs.InstanceStopUse,
	Short:   docs.InstanceStopShort,
	Long:    docs.InstanceStopLong,
	Example: docs.InstanceStopExample,
}
