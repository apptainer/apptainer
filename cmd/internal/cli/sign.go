// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2017-2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"fmt"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/apptainer/pkg/sypgp"
	"github.com/apptainer/sif/v2/pkg/integrity"
	"github.com/spf13/cobra"
)

var (
	privKey int // -k encryption key (index from 'key list --secret') specification
	signAll bool

	PKCS8PrivateKey string // -p path to PKCS8 private key specification
	x509Cert        string // -c path to X509 certificate specification
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

// -k|--keyidx
var signKeyIdxFlag = cmdline.Flag{
	ID:           "signKeyIdxFlag",
	Value:        &privKey,
	DefaultValue: 0,
	Name:         "keyidx",
	ShortHand:    "k",
	Usage:        "private key to use (index from 'key list --secret')",
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

// -p|--pkcs8Key
var signPKCS8KeyFlag = cmdline.Flag{
	ID:           "signPKCS8PrivateKeyFlag",
	Value:        &PKCS8PrivateKey,
	DefaultValue: "~/.apptainer/keys/pkcs8.key",
	Name:         "pkcs8key",
	ShortHand:    "p",
	Usage:        "path to PKCS8 private key to use",
}

// -c |--x509cert
var signX509CertFlag = cmdline.Flag{
	ID:           "signX509CertFlag",
	Value:        &x509Cert,
	DefaultValue: "~/.apptainer/keys/cert.pem",
	Name:         "x509Cert",
	ShortHand:    "c",
	Usage:        "path to X509 certificate to use",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(SignCmd)

		cmdManager.RegisterFlagForCmd(&signSifGroupIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signOldSifGroupIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signSifDescSifIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signSifDescIDFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signKeyIdxFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signAllFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signPKCS8KeyFlag, SignCmd)
		cmdManager.RegisterFlagForCmd(&signX509CertFlag, SignCmd)
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

	// Set group option, if applicable.
	if cmd.Flag(signSifGroupIDFlag.Name).Changed || cmd.Flag(signOldSifGroupIDFlag.Name).Changed {
		opts = append(opts, apptainer.OptSignGroup(sifGroupID))
	}

	// Set object option, if applicable.
	if cmd.Flag(signSifDescSifIDFlag.Name).Changed || cmd.Flag(signSifDescIDFlag.Name).Changed {
		opts = append(opts, apptainer.OptSignObjects(sifDescID))
	}

	// Set Signing method
	switch {
	case cmd.Flag(signPKCS8KeyFlag.Name).Changed: // Sign using X509
		signer, err := integrity.GetX509Signer(PKCS8PrivateKey, x509Cert)
		if err != nil {
			sylog.Fatalf("Failed to get X509 signer: %s", err)
		}

		opts = append(opts, apptainer.OptSignX509(signer))

	default: // Sign using PGP
		// Set entity selector option, and ensure the entity is decrypted.
		var f sypgp.EntitySelector
		if cmd.Flag(signKeyIdxFlag.Name).Changed {
			f = selectEntityAtIndex(privKey)
		} else {
			f = selectEntityInteractive()
		}
		f = decryptSelectedEntityInteractive(f)
		opts = append(opts, apptainer.OptSignEntitySelector(f))
	}

	// Sign the image.
	fmt.Printf("Signing image: %s\n", cpath)
	if err := apptainer.Sign(cpath, opts...); err != nil {
		sylog.Fatalf("Failed to sign container: %s", err)
	}
	fmt.Printf("Signature created and applied to %s\n", cpath)
}
