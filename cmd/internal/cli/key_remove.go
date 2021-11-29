// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/sypgp"
	"github.com/spf13/cobra"
)

// KeyRemoveCmd is `apptainer key remove <fingerprint>' command
var KeyRemoveCmd = &cobra.Command{
	PreRun:                checkGlobal,
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var opts []sypgp.HandleOpt
		path := ""

		if keyGlobalPubKey {
			path = buildcfg.APPTAINER_CONFDIR
			opts = append(opts, sypgp.GlobalHandleOpt())
		}

		keyring := sypgp.NewHandle(path, opts...)
		err := keyring.RemovePubKey(args[0])
		if err != nil {
			sylog.Fatalf("Unable to remove public key: %s", err)
		}
	},

	Use:     docs.KeyRemoveUse,
	Short:   docs.KeyRemoveShort,
	Long:    docs.KeyRemoveLong,
	Example: docs.KeyRemoveExample,
}
