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

var (
	secretExport bool
	armor        bool
)

// -s|--secret
var keyExportSecretFlag = cmdline.Flag{
	ID:           "keyExportSecretFlag",
	Value:        &secretExport,
	DefaultValue: false,
	Name:         "secret",
	ShortHand:    "s",
	Usage:        "export a secret key",
}

// -a|--armor
var keyExportArmorFlag = cmdline.Flag{
	ID:           "keyExportArmorFlag",
	Value:        &armor,
	DefaultValue: false,
	Name:         "armor",
	ShortHand:    "a",
	Usage:        "ascii armored format",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&keyExportSecretFlag, KeyExportCmd)
		cmdManager.RegisterFlagForCmd(&keyExportArmorFlag, KeyExportCmd)
	})
}

// KeyExportCmd is `apptainer key export` and exports a public or secret
// key from local keyring.
var KeyExportCmd = &cobra.Command{
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run:                   exportRun,

	Use:     docs.KeyExportUse,
	Short:   docs.KeyExportShort,
	Long:    docs.KeyExportLong,
	Example: docs.KeyExportExample,
}

func exportRun(cmd *cobra.Command, args []string) {
	var opts []sypgp.HandleOpt
	path := keyLocalDir

	if keyGlobalPubKey {
		path = buildcfg.APPTAINER_CONFDIR
		opts = append(opts, sypgp.GlobalHandleOpt())
	}

	keyring := sypgp.NewHandle(path, opts...)
	if secretExport {
		err := keyring.ExportPrivateKey(args[0], armor)
		if err != nil {
			sylog.Errorf("key export command failed: %s", err)
			os.Exit(10)
		}
	} else {
		err := keyring.ExportPubKey(args[0], armor)
		if err != nil {
			sylog.Errorf("key export command failed: %s", err)
			os.Exit(10)
		}
	}
}
