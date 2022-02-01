// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/spf13/cobra"
)

// configCmd is the config command
var configCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Use:                   docs.ConfigUse,
	Short:                 docs.ConfigShort,
	Long:                  docs.ConfigLong,
	Example:               docs.ConfigExample,
	SilenceErrors:         true,
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(configCmd)

		cmdManager.RegisterSubCmd(configCmd, configFakerootCmd)
		cmdManager.RegisterSubCmd(configCmd, configGlobalCmd)
	})
}
