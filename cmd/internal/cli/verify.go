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
	"crypto/x509"
	"fmt"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/apptainer/apptainer/docs"
	"github.com/apptainer/apptainer/internal/app/apptainer"
	"github.com/apptainer/apptainer/internal/pkg/remote/endpoint"
	"github.com/apptainer/apptainer/pkg/cmdline"
	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/apptainer/sif/v2/pkg/integrity"
	"github.com/apptainer/sif/v2/pkg/sif"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	sifGroupID   uint32 // -g groupid specification
	sifDescID    uint32 // -i id specification
	localVerify  bool   // -l flag
	jsonVerify   bool   // -j flag
	verifyAll    bool
	verifyLegacy bool

	x509RootCA            string // --x509RootCA
	x509IntermediateCerts string // --x509IntermediateCerts
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

// -c |--x509cert
var verifyX509CertFlag = cmdline.Flag{
	ID:           "verifyX509CertFlag",
	Value:        &x509Cert,
	DefaultValue: "~/.apptainer/keys/cert.pem",
	Name:         "x509Cert",
	ShortHand:    "c",
	Usage:        "verify x509 signature using the cert",
}

// --x509RootCA
var verifyX509RootCAFlag = cmdline.Flag{
	ID:           "verifyX509RootCAFlag",
	Value:        &x509RootCA,
	DefaultValue: "~/.apptainer/keys/x509rootCA.pem",
	Name:         "x509RootCA",
	Usage:        "verify x509 cert using the root CA",
}

// --x509IntermediateCerts
var verifyX509IntermediateCertsFlag = cmdline.Flag{
	ID:           "verifyX509IntermediateCertsFlag",
	Value:        &x509IntermediateCerts,
	DefaultValue: "~/.apptainer/keys/x509intermediateCerts.pem",
	Name:         "x509IntermediateCerts",
	Usage:        "verify x509 cert using intermediate certs",
}

func init() {
	addCmdInit(func(cmdManager *cmdline.CommandManager) {
		cmdManager.RegisterCmd(VerifyCmd)

		cmdManager.RegisterFlagForCmd(&verifyServerURIFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifGroupIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyOldSifGroupIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifDescSifIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifySifDescIDFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyLocalFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyJSONFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyAllFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyLegacyFlag, VerifyCmd)

		cmdManager.RegisterFlagForCmd(&verifyX509CertFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyX509RootCAFlag, VerifyCmd)
		cmdManager.RegisterFlagForCmd(&verifyX509IntermediateCertsFlag, VerifyCmd)
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

	// Set X509 verification, if applicable.
	if cmd.Flag(verifyX509CertFlag.Name).Changed {
		// Load the signature-signing X509 Certificate
		leafCerts, err := integrity.NewChainedCertificates(x509Cert)
		if err != nil {
			sylog.Fatalf("Failed to get the signature-signing X509 certificate: %s", err)
		}

		signatureSigningCert, err := leafCerts.GetCertificate()
		if err != nil {
			sylog.Fatalf("Failed to get the signature-signing X509 certificate: %s", err)
		}

		// If the certificate is not self-signed, then we need the certificate of the authority who
		// signed the certificate.
		if signatureSigningCert.Issuer.String() != signatureSigningCert.Subject.String() {
			sylog.Infof("Certificate '%s' requires Intermediate or Root CA certificates", x509Cert)

			// Load Root CA Certificates (to validate intermediate or root certificates)
			// If user options are not defined, use the default system cert pool.
			if !cmd.Flag(verifyX509RootCAFlag.Name).Changed {
				x509RootCA = ""
			}
			rootCerts, err := integrity.NewChainedCertificates(x509RootCA)
			if err != nil {
				sylog.Fatalf("Failed to get the Root CA x509 certificate: %s", err)
			}

			// Load intermediate Certificates (to validate the leaf certificate)
			var intermediateCerts integrity.ChainedCertificates

			if cmd.Flag(verifyX509IntermediateCertsFlag.Name).Changed {
				certs, err := integrity.NewChainedCertificates(x509IntermediateCerts)
				if err != nil {
					sylog.Fatalf("Failed to get the intermediate X509 certificates: %s", err)
				}

				intermediateCerts = certs
			} else {
				sylog.Infof("Skip intermediate certificates.")
			}

			// Offline verification of leafCerts
			if err := leafCerts.Verify(intermediateCerts, rootCerts); err != nil {
				sylog.Fatalf("validation of leaf certificate failed. Err: %s", err)
			}

			// Online revocation check
			if err := leafCerts.RevocationCheck(intermediateCerts, rootCerts); err != nil {
				sylog.Fatalf("Online revocation check failed. Err: %s", err)
			}
		}

		// Validate the signature using X509 certificate
		opts = append(opts, apptainer.OptVerifyUseX509Cert(signatureSigningCert))

	} else {
		// Set PGP keyserver option, if applicable.
		if !localVerify {
			co, err := getKeyserverClientOpts(keyServerURI, endpoint.KeyserverVerifyOp)
			if err != nil {
				sylog.Fatalf("Error while getting keyserver client config: %v", err)
			}

			opts = append(opts, apptainer.OptVerifyUseKeyServer(co...))
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
			sylog.Fatalf("Failed to verify container: %s", verifyErr)
		}
	} else {
		opts = append(opts, apptainer.OptVerifyCallback(outputVerify))

		fmt.Printf("Verifying image: %s\n", cpath)

		if err := apptainer.Verify(cmd.Context(), cpath, opts...); err != nil {
			sylog.Fatalf("Failed to verify container: %s", err)
		}

		fmt.Printf("Container verified: %s\n", cpath)
	}
}

// outputVerify outputs a textual representation of r to stdout.
func outputVerify(f *sif.FileImage, r integrity.VerifyResult) bool {
	// Print signing entity info.
	switch e := r.Entity().(type) {
	case *openpgp.Entity:
		if e == nil {
			// This may happen if the image is signed with X509, but PGP flags are used.
			sylog.Warningf("PGP Signer identity unknown")
			return false
		}

		prefix := color.New(color.FgYellow).Sprint("[REMOTE]")

		if isGlobal(e) {
			prefix = color.New(color.FgCyan).Sprint("[GLOBAL]")
		} else if isLocal(e) {
			prefix = color.New(color.FgGreen).Sprint("[LOCAL]")
		}

		// Print identity, if possible.
		if id := primaryIdentity(e); id != nil {
			fmt.Printf("%-18v Signing entity: %v\n", prefix, id.Name)
		} else {
			sylog.Warningf("Primary identity unknown")
		}

		// Always print fingerprint.
		fmt.Printf("%-18v Fingerprint: %X\n", prefix, e.PrimaryKey.Fingerprint)
	case *x509.Certificate:
		if e == nil {
			// This may happen if the image is signed with PGP, but X509 flags are used.
			sylog.Warningf("X509 Signer identity unknown")
			return false
		}

		prefix := color.New(color.FgYellow).Sprint("[X509]")

		// Always print fingerprint.
		fmt.Printf("%-18v Subject: %s\n", prefix, e.Subject.String())
	default:
		sylog.Fatalf("unsupported method %s", e)
	}

	// Print table of signed objects.
	if len(r.Verified()) > 0 {
		fmt.Printf("Objects verified:\n")
		fmt.Printf("%-4s|%-8s|%-8s|%s\n", "ID", "GROUP", "LINK", "TYPE")
		fmt.Print("------------------------------------------------\n")
	}

	for _, od := range r.Verified() {
		group := "NONE"
		if gid := od.GroupID(); gid != 0 {
			group = fmt.Sprintf("%d", gid)
		}

		link := "NONE"
		if l, isGroup := od.LinkedID(); l != 0 {
			if isGroup {
				link = fmt.Sprintf("%d (G)", l)
			} else {
				link = fmt.Sprintf("%d", l)
			}
		}

		fmt.Printf("%-4d|%-8s|%-8s|%s\n", od.ID(), group, link, od.DataType())
	}

	if err := r.Error(); err != nil {
		fmt.Printf("\nError encountered during signature verification: %v\n", err)
	}

	return false
}
