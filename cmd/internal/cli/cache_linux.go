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
		cmdManager.RegisterCmd(CacheCmd)
		cmdManager.RegisterSubCmd(CacheCmd, cacheCleanCmd)
		cmdManager.RegisterSubCmd(CacheCmd, CacheListCmd)
	})
}

// CacheCmd : aka, `apptainer cache`
var CacheCmd = &cobra.Command{
	RunE: func(_ *cobra.Command, _ []string) error {
		return errors.New("invalid command")
	},
	DisableFlagsInUseLine: true,

	Use:           docs.CacheUse,
	Short:         docs.CacheShort,
	Long:          docs.CacheLong,
	Example:       docs.CacheExample,
	SilenceErrors: true,
}
