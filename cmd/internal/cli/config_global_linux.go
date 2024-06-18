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

// -s|--set
var globalConfigSet bool

var globalConfigSetFlag = cmdline.Flag{
	ID:           "globalConfigSetFlag",
	Value:        &globalConfigSet,
	DefaultValue: false,
	Name:         "set",
	ShortHand:    "s",
	Usage:        "set value of the configuration directive (for multi-value directives, it will add it)",
}

// -u|--unset
var globalConfigUnset bool

var globalConfigUnsetFlag = cmdline.Flag{
	ID:           "globalConfigUnsetFlag",
	Value:        &globalConfigUnset,
	DefaultValue: false,
	Name:         "unset",
	ShortHand:    "u",
	Usage:        "unset value of the configuration directive (for multi-value directives, it will remove matching values)",
}

// -g|--get
var globalConfigGet bool

var globalConfigGetFlag = cmdline.Flag{
	ID:           "globalConfigGetFlag",
	Value:        &globalConfigGet,
	DefaultValue: false,
	Name:         "get",
	ShortHand:    "g",
	Usage:        "get value of the configuration directive",
}

// -r|--reset
var globalConfigReset bool

var globalConfigResetFlag = cmdline.Flag{
	ID:           "globalConfigResetFlag",
	Value:        &globalConfigReset,
	DefaultValue: false,
	Name:         "reset",
	ShortHand:    "r",
	Usage:        "reset the configuration directive value to its default value",
}

// -d|--dry-run
var globalConfigDryRun bool

var globalConfigDryRunFlag = cmdline.Flag{
	ID:           "globalConfigDryRunFlag",
	Value:        &globalConfigDryRun,
	DefaultValue: false,
	Name:         "dry-run",
	ShortHand:    "d",
	Usage:        "dump resulting configuration on stdout but doesn't write it to apptainer.conf",
}

// configGlobalCmd apptainer config global
var configGlobalCmd = &cobra.Command{
	Args:                  cobra.RangeArgs(1, 2),
	DisableFlagsInUseLine: true,
	PreRun:                CheckRootOrUnpriv,
	RunE: func(_ *cobra.Command, args []string) error {
		var op apptainer.GlobalConfigOp

		if globalConfigSet {
			op = apptainer.GlobalConfigSet
		} else if globalConfigUnset {
			op = apptainer.GlobalConfigUnset
		} else if globalConfigReset {
			op = apptainer.GlobalConfigReset
		} else if globalConfigGet {
			op = apptainer.GlobalConfigGet
		} else {
			return fmt.Errorf("you must specify an option (eg: --set/--unset)")
		}

		if err := apptainer.GlobalConfig(args, configurationFile, globalConfigDryRun, op); err != nil {
			sylog.Fatalf("%s", err)
		}

		return nil
	},

	Use:     docs.ConfigGlobalUse,
	Short:   docs.ConfigGlobalShort,
	Long:    docs.ConfigGlobalLong,
	Example: docs.ConfigGlobalExample,
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&globalConfigSetFlag, configGlobalCmd)
		cmdManager.RegisterFlagForCmd(&globalConfigUnsetFlag, configGlobalCmd)
		cmdManager.RegisterFlagForCmd(&globalConfigGetFlag, configGlobalCmd)
		cmdManager.RegisterFlagForCmd(&globalConfigResetFlag, configGlobalCmd)
		cmdManager.RegisterFlagForCmd(&globalConfigDryRunFlag, configGlobalCmd)
	})
}
