// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2017-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"fmt"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/sypgp"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/spf13/cobra"
)

var secret bool

// -s|--secret
var keyListSecretFlag = cmdline.Flag{
	ID:           "keyListSecretFlag",
	Value:        &secret,
	DefaultValue: false,
	Name:         "secret",
	ShortHand:    "s",
	Usage:        "list private keys instead of the default which displays public ones",
	EnvKeys:      []string{"SECRET"},
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterFlagForCmd(&keyListSecretFlag, KeyListCmd)
	})
}

// KeyListCmd is `apptainer key list' and lists local store OpenPGP keys
var KeyListCmd = &cobra.Command{
	Args:                  cobra.ExactArgs(0),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if err := doKeyListCmd(secret); err != nil {
			sylog.Fatalf("While listing keys: %s", err)
		}
	},

	Use:     docs.KeyListUse,
	Short:   docs.KeyListShort,
	Long:    docs.KeyListLong,
	Example: docs.KeyListExample,
}

func doKeyListCmd(secret bool) error {
	var opts []sypgp.HandleOpt
	path := keyLocalDir

	if keyGlobalPubKey {
		path = buildcfg.APPTAINER_CONFDIR
		opts = append(opts, sypgp.GlobalHandleOpt())
	}

	keyring := sypgp.NewHandle(path, opts...)
	if !secret {
		fmt.Printf("Public key listing (%s):\n\n", keyring.PublicPath())
		if err := keyring.PrintPubKeyring(); err != nil {
			return fmt.Errorf("could not list public keys: %s", err)
		}
	} else {
		fmt.Printf("Private key listing (%s):\n\n", keyring.SecretPath())
		if err := keyring.PrintPrivKeyring(); err != nil {
			return fmt.Errorf("could not list private keys: %s", err)
		}
	}

	return nil
}
