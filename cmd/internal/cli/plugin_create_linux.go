// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// PluginCreateCmd creates a plugin skeleton directory
// structure to start developing a new plugin.
//
// apptainer plugin create <directory> <name>
var PluginCreateCmd = &cobra.Command{
	Run: func(_ *cobra.Command, args []string) {
		name := args[1]
		dir := args[0]

		err := apptainer.CreatePlugin(dir, name)
		if err != nil {
			sylog.Fatalf("Failed to create plugin directory %s: %s.", dir, err)
		}
	},
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),

	Use:     docs.PluginCreateUse,
	Short:   docs.PluginCreateShort,
	Long:    docs.PluginCreateLong,
	Example: docs.PluginCreateExample,
}
