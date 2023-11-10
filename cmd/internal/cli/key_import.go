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
	"os"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/sypgp"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

// KeyImportCmd is `apptainer key (or keys) import` and imports a local key into the apptainer keyring.
var KeyImportCmd = &cobra.Command{
	PreRun:                checkGlobal,
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run:                   importRun,

	Use:     docs.KeyImportUse,
	Short:   docs.KeyImportShort,
	Long:    docs.KeyImportLong,
	Example: docs.KeyImportExample,
}

var (
	keyImportWithNewPassword     bool
	keyImportWithNewPasswordFlag = cmdline.Flag{
		ID:           "keyImportWithNewPasswordFlag",
		Value:        &keyImportWithNewPassword,
		DefaultValue: false,
		Name:         "new-password",
		Usage:        `set a new password to the private key`,
	}
)

func importRun(cmd *cobra.Command, args []string) {
	var opts []sypgp.HandleOpt
	path := keyLocalDir

	if keyGlobalPubKey {
		path = buildcfg.APPTAINER_CONFDIR
		opts = append(opts, sypgp.GlobalHandleOpt())
	}

	keyring := sypgp.NewHandle(path, opts...)
	if err := keyring.ImportKey(args[0], keyImportWithNewPassword); err != nil {
		sylog.Errorf("key import command failed: %s", err)
		os.Exit(2)
	}
}
