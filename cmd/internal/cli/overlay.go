// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies

package cli

import (
	"errors"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/spf13/cobra"
)

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(OverlayCmd)
		cmdManager.RegisterSubCmd(OverlayCmd, OverlayCreateCmd)

		cmdManager.RegisterFlagForCmd(&overlaySizeFlag, OverlayCreateCmd)
		cmdManager.RegisterFlagForCmd(&overlayCreateDirFlag, OverlayCreateCmd)
		cmdManager.RegisterFlagForCmd(&overlayFakerootFlag, OverlayCreateCmd)
		cmdManager.RegisterFlagForCmd(&overlaySparseFlag, OverlayCreateCmd)
	})
}

// OverlayCmd is the 'overlay' command that allows to manage writable overlay.
var OverlayCmd = &cobra.Command{
	RunE: func(_ *cobra.Command, _ []string) error {
		return errors.New("invalid command")
	},
	DisableFlagsInUseLine: true,

	Use:     docs.OverlayUse,
	Short:   docs.OverlayShort,
	Long:    docs.OverlayLong,
	Example: docs.OverlayExample,
}
