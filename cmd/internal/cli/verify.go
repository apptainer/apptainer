// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2017-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"crypto"
	"os"

	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/spf13/cobra"
)

var (
	sifGroupID   uint32 // -g groupid specification
	sifDescID    uint32 // -i id specification
	pubKeyPath   string // --key flag
	localVerify  bool   // -l flag
	jsonVerify   bool   // -j flag
	verifyAll    bool
	verifyLegacy bool
)

// -u|--url
var verifyServerURIFlag = cmdline.Flag{
	ID:           "verifyServerURIFlag",
	Value:        &keyServerURI,
	DefaultValue: "",
	Name:         "url",
	ShortHand:    "u",
	Usage:        "specify a URL for a key server",
	EnvKeys:      []string{"URL"},
}

// -g|--group-id
var verifySifGroupIDFlag = cmdline.Flag{
	ID:           "verifySifGroupIDFlag",
	Value:        &sifGroupID,
	DefaultValue: uint32(0),
	Name:         "group-id",
	ShortHand:    "g",
	Usage:        "verify objects with the specified group ID",
}

// --groupid (deprecated)
var verifyOldSifGroupIDFlag = cmdline.Flag{
	ID:           "verifyOldSifGroupIDFlag",
	Value:        &sifGroupID,
	DefaultValue: uint32(0),
	Name:         "groupid",
	Usage:        "verify objects with the specified group ID",
	Deprecated:   "use '--group-id'",
}

// -i|--sif-id
var verifySifDescSifIDFlag = cmdline.Flag{
	ID:           "verifySifDescSifIDFlag",
	Value:        &sifDescID,
	DefaultValue: uint32(0),
	Name:         "sif-id",
	ShortHand:    "i",
	Usage:        "verify object with the specified ID",
}

// --id (deprecated)
var verifySifDescIDFlag = cmdline.Flag{
	ID:           "verifySifDescIDFlag",
	Value:        &sifDescID,
	DefaultValue: uint32(0),
	Name:         "id",
	Usage:        "verify object with the specified ID",
	Deprecated:   "use '--sif-id'",
}

// --key
var verifyPublicKeyFlag = cmdline.Flag{
	ID:           "publicKeyFlag",
	Value:        &pubKeyPath,
	DefaultValue: "",
	Name:         "key",
	Usage:        "path to the public key file",
	EnvKeys:      []string{"VERIFY_KEY"},
}

// -l|--local
var verifyLocalFlag = cmdline.Flag{
	ID:           "verifyLocalFlag",
	Value:        &localVerify,
	DefaultValue: false,
	Name:         "local",
	ShortHand:    "l",
	Usage:        "only verify with local key(s) in keyring",
	EnvKeys:      []string{"LOCAL_VERIFY"},
}

// -j|--json
var verifyJSONFlag = cmdline.Flag{
	ID:           "verifyJsonFlag",
	Value:        &jsonVerify,
	DefaultValue: false,
	Name:         "json",
	ShortHand:    "j",
	Usage:        "output json",
}

// -a|--all
var verifyAllFlag = cmdline.Flag{
	ID:           "verifyAllFlag",
	Value:        &verifyAll,
	DefaultValue: false,
	Name:         "all",
	ShortHand:    "a",
	Usage:        "verify all objects",
}

// --legacy-insecure
var verifyLegacyFlag = cmdline.Flag{
	ID:           "verifyLegacyFlag",
	Value:        &verifyLegacy,
	DefaultValue: false,
	Name:         "legacy-insecure",
	Usage:        "enable verification of (insecure) legacy signatures",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(VerifyCmd)

		cmdManager.RegisterFlagForCmd(&verifyServerURIFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifGroupIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyOldSifGroupIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifDescSifIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifDescIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyPublicKeyFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyLocalFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyJSONFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyAllFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyLegacyFlag, VerifyCmd)
	})
}

// VerifyCmd apptainer verify
var VerifyCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(1),

	Run: func(cmd *cobra.Command, args []string) {
		// args[0] contains image path
		doVerifyCmd(cmd, args[0])
	},

	Use:     docs.VerifyUse,
	Short:   docs.VerifyShort,
	Long:    docs.VerifyLong,
	Example: docs.VerifyExample,
}

func doVerifyCmd(cmd *cobra.Command, cpath string) {
	var opts []apptainer.VerifyOpt

	switch {
	case cmd.Flag(verifyPublicKeyFlag.Name).Changed:
		sylog.Infof("Verifying image with key material from '%v'", pubKeyPath)

		v, err := signature.LoadVerifierFromPEMFile(pubKeyPath, crypto.SHA256)
		if err != nil {
			sylog.Fatalf("Failed to load key material: %v", err)
		}
		opts = append(opts, apptainer.OptVerifyWithVerifier(v))

	default:
		sylog.Infof("Verifying image with PGP key material")

		// Set keyserver option, if applicable.
		if localVerify {
			opts = append(opts, apptainer.OptVerifyWithPGP())
		} else {
			co, err := getKeyserverClientOpts(keyServerURI, endpoint.KeyserverVerifyOp)
			if err != nil {
				sylog.Fatalf("Error while getting keyserver client config: %v", err)
			}
			opts = append(opts, apptainer.OptVerifyWithPGP(co...))
		}
	}

	// Set group option, if applicable.
	if cmd.Flag(verifySifGroupIDFlag.Name).Changed || cmd.Flag(verifyOldSifGroupIDFlag.Name).Changed {
		opts = append(opts, apptainer.OptVerifyGroup(sifGroupID))
	}

	// Set object option, if applicable.
	if cmd.Flag(verifySifDescSifIDFlag.Name).Changed || cmd.Flag(verifySifDescIDFlag.Name).Changed {
		opts = append(opts, apptainer.OptVerifyObject(sifDescID))
	}

	// Set all option, if applicable.
	if verifyAll {
		opts = append(opts, apptainer.OptVerifyAll())
	}

	// Set legacy option, if applicable.
	if verifyLegacy {
		opts = append(opts, apptainer.OptVerifyLegacy())
	}

	// Set callback option.
	if jsonVerify {
		var kl keyList

		opts = append(opts, apptainer.OptVerifyCallback(getJSONCallback(&kl)))

		verifyErr := apptainer.Verify(cmd.Context(), cpath, opts...)

		// Always output JSON.
		if err := outputJSON(os.Stdout, kl); err != nil {
			sylog.Fatalf("Failed to output JSON: %v", err)
		}

		if verifyErr != nil {
			sylog.Fatalf("Failed to verify container: %v", verifyErr)
		}
	} else {
		opts = append(opts, apptainer.OptVerifyCallback(outputVerify))

		if err := apptainer.Verify(cmd.Context(), cpath, opts...); err != nil {
			sylog.Fatalf("Failed to verify container: %v", err)
		}

		sylog.Infof("Verified signature(s) from image '%v'", cpath)
	}
}
