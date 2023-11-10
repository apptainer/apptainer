// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/sypgp"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// KeyRemoveCmd is `apptainer key remove <fingerprint>' command
var KeyRemoveCmd = &cobra.Command{
	PreRun:                checkGlobal,
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var opts []sypgp.HandleOpt
		path := keyLocalDir

		if keyGlobalPubKey {
			path = buildcfg.APPTAINER_CONFDIR
			opts = append(opts, sypgp.GlobalHandleOpt())
		}

		keyring := sypgp.NewHandle(path, opts...)

		if (keyRemovePrivate && keyRemovePublic) || keyRemoveBoth {
			pubErr := keyring.RemovePubKey(args[0])
			priErr := keyring.RemovePrivKey(args[0])
			if pubErr != nil && priErr != nil {
				sylog.Fatalf("Unable to remove neither public key: %s, nor private key: %s", pubErr, priErr)
			}
		} else if keyRemovePrivate {
			err := keyring.RemovePrivKey(args[0])
			if err != nil {
				sylog.Fatalf("Unable to remove private key: %s", err)
			}
		} else {
			err := keyring.RemovePubKey(args[0])
			if err != nil {
				sylog.Fatalf("Unable to remove public key: %s", err)
			}
		}
	},

	Use:     docs.KeyRemoveUse,
	Short:   docs.KeyRemoveShort,
	Long:    docs.KeyRemoveLong,
	Example: docs.KeyRemoveExample,
}
