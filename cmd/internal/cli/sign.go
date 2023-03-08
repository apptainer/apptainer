// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2017-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"crypto"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/sypgp"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/spf13/cobra"
)

var (
	priKeyPath string
	priKeyIdx  int
	signAll    bool
)

// -g|--group-id
var signSifGroupIDFlag = cmdline.Flag{
	ID:           "signSifGroupIDFlag",
	Value:        &sifGroupID,
	DefaultValue: uint32(0),
	Name:         "group-id",
	ShortHand:    "g",
	Usage:        "sign objects with the specified group ID",
}

// --groupid (deprecated)
var signOldSifGroupIDFlag = cmdline.Flag{
	ID:           "signOldSifGroupIDFlag",
	Value:        &sifGroupID,
	DefaultValue: uint32(0),
	Name:         "groupid",
	Usage:        "sign objects with the specified group ID",
	Deprecated:   "use '--group-id'",
}

// -i| --sif-id
var signSifDescSifIDFlag = cmdline.Flag{
	ID:           "signSifDescSifIDFlag",
	Value:        &sifDescID,
	DefaultValue: uint32(0),
	Name:         "sif-id",
	ShortHand:    "i",
	Usage:        "sign object with the specified ID",
}

// --id (deprecated)
var signSifDescIDFlag = cmdline.Flag{
	ID:           "signSifDescIDFlag",
	Value:        &sifDescID,
	DefaultValue: uint32(0),
	Name:         "id",
	Usage:        "sign object with the specified ID",
	Deprecated:   "use '--sif-id'",
}

// --key
var signPrivateKeyFlag = cmdline.Flag{
	ID:           "privateKeyFlag",
	Value:        &priKeyPath,
	DefaultValue: "",
	Name:         "key",
	Usage:        "path to the private key file",
	EnvKeys:      []string{"SIGN_KEY"},
}

// -k|--keyidx
var signKeyIdxFlag = cmdline.Flag{
	ID:           "signKeyIdxFlag",
	Value:        &priKeyIdx,
	DefaultValue: 0,
	Name:         "keyidx",
	ShortHand:    "k",
	Usage:        "PGP private key to use (index from 'key list --secret')",
}

// -a|--all (deprecated)
var signAllFlag = cmdline.Flag{
	ID:           "signAllFlag",
	Value:        &signAll,
	DefaultValue: false,
	Name:         "all",
	ShortHand:    "a",
	Usage:        "sign all objects",
	Deprecated:   "now the default behavior",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(SignCmd)

		cmdManager.RegisterFlagForCmd(&signSifGroupIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signOldSifGroupIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signSifDescSifIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signSifDescIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signPrivateKeyFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signKeyIdxFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signAllFlag, SignCmd)
	})
}

// SignCmd apptainer sign
var SignCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),

	Run: func(cmd *cobra.Command, args []string) {
		// args[0] contains image path
		doSignCmd(cmd, args[0])
	},

	Use:     docs.SignUse,
	Short:   docs.SignShort,
	Long:    docs.SignLong,
	Example: docs.SignExample,
}

func doSignCmd(cmd *cobra.Command, cpath string) {
	var opts []apptainer.SignOpt

	// Set key material.
	switch {
	case cmd.Flag(signPrivateKeyFlag.Name).Changed:
		sylog.Infof("Signing image with key material from '%v'", priKeyPath)

		s, err := signature.LoadSignerFromPEMFile(priKeyPath, crypto.SHA256, cryptoutils.GetPasswordFromStdIn)
		if err != nil {
			sylog.Fatalf("Failed to load key material: %v", err)
		}
		opts = append(opts, apptainer.OptSignWithSigner(s))

	default:
		sylog.Infof("Signing image with PGP key material")

		// Set entity selector option, and ensure the entity is decrypted.
		var f sypgp.EntitySelector
		if cmd.Flag(signKeyIdxFlag.Name).Changed {
			f = selectEntityAtIndex(priKeyIdx)
		} else {
			f = selectEntityInteractive()
		}
		f = decryptSelectedEntityInteractive(f)
		opts = append(opts, apptainer.OptSignEntitySelector(f))
	}

	// Set group option, if applicable.
	if cmd.Flag(signSifGroupIDFlag.Name).Changed || cmd.Flag(signOldSifGroupIDFlag.Name).Changed {
		opts = append(opts, apptainer.OptSignGroup(sifGroupID))
	}

	// Set object option, if applicable.
	if cmd.Flag(signSifDescSifIDFlag.Name).Changed || cmd.Flag(signSifDescIDFlag.Name).Changed {
		opts = append(opts, apptainer.OptSignObjects(sifDescID))
	}

	// Sign the image.
	if err := apptainer.Sign(cpath, opts...); err != nil {
		sylog.Fatalf("Failed to sign container: %v", err)
	}
	sylog.Infof("Signature created and applied to %v\n", cpath)
}
