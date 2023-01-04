// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/internal/pkg/confgen"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(ConfGenCmd)
	})
}

// ConfGenCmd generates an apptainer.conf file, optionally taking an
// old singularity.conf or apptainer.conf for initial settings.
var ConfGenCmd = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		return confgen.Gen(args)
	},
	DisableFlagsInUseLine: true,

	Hidden:  true,
	Args:    cobra.MaximumNArgs(2),
	Use:     "confgen [oldconffile] newconffile",
	Short:   "Create an apptainer.conf, optionally initializing settings from an old one",
	Example: "$ apptainer confgen oldapptainer.conf newapptainer.conf",
}
