// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2017-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"context"
	"os"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/internal/pkg/sypgp"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/container-key-client/client"
	"github.com/spf13/cobra"
)

// KeySearchCmd is 'apptainer key search' and look for public keys from a key server
var KeySearchCmd = &cobra.Command{
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		co, err := getKeyserverClientOpts(keyServerURI, endpoint.KeyserverSearchOp)
		if err != nil {
			sylog.Fatalf("Keyserver client failed: %s", err)
		}

		if err := doKeySearchCmd(cmd.Context(), args[0], co...); err != nil {
			sylog.Errorf("search failed: %s", err)
			os.Exit(2)
		}
	},

	Use:     docs.KeySearchUse,
	Short:   docs.KeySearchShort,
	Long:    docs.KeySearchLong,
	Example: docs.KeySearchExample,
}

func doKeySearchCmd(ctx context.Context, search string, co ...client.Option) error {
	// get keyring with matching search string
	return sypgp.SearchPubkey(ctx, search, keySearchLongList, co...)
}
