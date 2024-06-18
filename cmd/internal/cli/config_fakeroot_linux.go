// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"fmt"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// -a|--add
var fakerootConfigAdd bool

var fakerootConfigAddFlag = cmdline.Flag{
	ID:           "fakerootConfigAddFlag",
	Value:        &fakerootConfigAdd,
	DefaultValue: false,
	Name:         "add",
	ShortHand:    "a",
	Usage:        "add a fakeroot mapping entry for a user allowing him to use the fakeroot feature",
}

// -r|--remove
var fakerootConfigRemove bool

var fakerootConfigRemoveFlag = cmdline.Flag{
	ID:           "fakerootConfigRemoveFlag",
	Value:        &fakerootConfigRemove,
	DefaultValue: false,
	Name:         "remove",
	ShortHand:    "r",
	Usage:        "remove the user fakeroot mapping entry preventing him to use the fakeroot feature",
}

// -e|--enable
var fakerootConfigEnable bool

var fakerootConfigEnableFlag = cmdline.Flag{
	ID:           "fakerootConfigEnableFlag",
	Value:        &fakerootConfigEnable,
	DefaultValue: false,
	Name:         "enable",
	ShortHand:    "e",
	Usage:        "enable a user fakeroot mapping entry allowing him to use the fakeroot feature (the user mapping must be present)",
}

// -d|--disable
var fakerootConfigDisable bool

var fakerootConfigDisableFlag = cmdline.Flag{
	ID:           "fakerootConfigDisableFlag",
	Value:        &fakerootConfigDisable,
	DefaultValue: false,
	Name:         "disable",
	ShortHand:    "d",
	Usage:        "disable a user fakeroot mapping entry preventing him to use the fakeroot feature (the user mapping must be present)",
}

// configFakerootCmd apptainer config fakeroot
var configFakerootCmd = &cobra.Command{
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	PreRun:                CheckRoot,
	RunE: func(_ *cobra.Command, args []string) error {
		username := args[0]
		var op apptainer.FakerootConfigOp

		if fakerootConfigAdd {
			op = apptainer.FakerootAddUser
		} else if fakerootConfigRemove {
			op = apptainer.FakerootRemoveUser
		} else if fakerootConfigEnable {
			op = apptainer.FakerootEnableUser
		} else if fakerootConfigDisable {
			op = apptainer.FakerootDisableUser
		} else {
			return fmt.Errorf("you must specify an option (eg: --add/--remove)")
		}

		if err := apptainer.FakerootConfig(username, op); err != nil {
			sylog.Fatalf("%s", err)
		}

		return nil
	},

	Use:     docs.ConfigFakerootUse,
	Short:   docs.ConfigFakerootShort,
	Long:    docs.ConfigFakerootLong,
	Example: docs.ConfigFakerootExample,
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&fakerootConfigAddFlag, configFakerootCmd)
		cmdManager.RegisterFlagForCmd(&fakerootConfigRemoveFlag, configFakerootCmd)
		cmdManager.RegisterFlagForCmd(&fakerootConfigEnableFlag, configFakerootCmd)
		cmdManager.RegisterFlagForCmd(&fakerootConfigDisableFlag, configFakerootCmd)
	})
}
