// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/sif/v2/pkg/siftool"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmd := &cobra.Command{
			Use:                   docs.SIFUse,
			Aliases:               []string{docs.SIFAlias},
			Short:                 docs.SIFShort,
			Long:                  docs.SIFLong,
			Example:               docs.SIFExample,
			DisableFlagsInUseLine: true,
		}
		siftool.AddCommands(cmd)

		cmdManager.RegisterCmd(cmd)
	})
}
