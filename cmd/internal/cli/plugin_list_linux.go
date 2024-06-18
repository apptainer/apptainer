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
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// PluginListCmd lists the plugins installed in the system.
var PluginListCmd = &cobra.Command{
	Run: func(_ *cobra.Command, _ []string) {
		err := apptainer.ListPlugins()
		if err != nil {
			sylog.Fatalf("Failed to get a list of installed plugins: %s.", err)
		}
	},
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(0),

	Use:     docs.PluginListUse,
	Short:   docs.PluginListShort,
	Long:    docs.PluginListLong,
	Example: docs.PluginListExample,
}
