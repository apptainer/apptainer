// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"errors"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(PluginCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginListCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginInstallCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginUninstallCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginEnableCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginDisableCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginCompileCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginInspectCmd)
		cmdManager.RegisterSubCmd(PluginCmd, PluginCreateCmd)
	})
}

// PluginCmd is the root command for all plugin related functionality
// which is exposed via the CLI.
//
// apptainer plugin [...]
var PluginCmd = &cobra.Command{
	RunE: func(_ *cobra.Command, _ []string) error {
		return errors.New("invalid command")
	},
	DisableFlagsInUseLine: true,

	Use:           docs.PluginUse,
	Short:         docs.PluginShort,
	Long:          docs.PluginLong,
	Example:       docs.PluginExample,
	Aliases:       []string{"plugins"},
	SilenceErrors: true,
}
