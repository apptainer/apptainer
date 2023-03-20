// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apptainer/apptainer/e2e/verify/ocspresponder"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	e2e.TestEnv
}

func (c *ctx) verify(t *testing.T) {
	pubKeyPath := filepath.Join("..", "test", "keys", "ed25519-public.pem")
	priKeyPath := filepath.Join("..", "test", "keys", "ed25519-private.pem")

	certPath := filepath.Join("..", "test", "certs", "leaf.pem")
	intPath := filepath.Join("..", "test", "certs", "intermediate.pem")
	rootPath := filepath.Join("..", "test", "certs", "root.pem")

	ocspOk := true
	if err := c.startOCSPResponder(priKeyPath, rootPath); err != nil {
		t.Errorf("OCSP responder could not start: %s", err)
		ocspOk = false
	}

	tests := []struct {
		name       string
		envs       []string
		flags      []string
		imagePath  string
		needOCSP   bool
		expectCode int
		expectOps  []e2e.ApptainerCmdResultOp
	}{
		{
			name:  "Help",
			flags: []string{"--help"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectOutput(e2e.ContainMatch, "Verify digital signature(s) within an image"),
			},
		},
		{
			name:      "OK",
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-pgp.sif"),
			flags:     []string{"--local"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with PGP key material"),
				e2e.ExpectOutput(e2e.ContainMatch, "Signing entity: SingularityCE Tests <singularityce@example.com>"),
				e2e.ExpectOutput(e2e.ContainMatch, "Fingerprint: F34371D0ACD5D09EB9BD853A80600A5FA11BBD29"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name:      "LegacyObjectIDFlag",
			flags:     []string{"--local", "--legacy-insecure", "--sif-id", "2"},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-legacy.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with PGP key material"),
				e2e.ExpectOutput(e2e.ContainMatch, "Signing entity: SingularityCE Tests <singularityce@example.com>"),
				e2e.ExpectOutput(e2e.ContainMatch, "Fingerprint: F34371D0ACD5D09EB9BD853A80600A5FA11BBD29"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name:       "LegacyObjectIDFlagNotFound",
			flags:      []string{"--local", "--legacy-insecure", "--sif-id", "9"},
			imagePath:  filepath.Join("..", "test", "images", "one-group-signed-legacy.sif"),
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "integrity: object not found"),
			},
		},
		{
			name:      "LegacyGroupIDFlag",
			flags:     []string{"--local", "--legacy-insecure", "--group-id", "1"},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-legacy-group.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with PGP key material"),
				e2e.ExpectOutput(e2e.ContainMatch, "Signing entity: SingularityCE Tests <singularityce@example.com>"),
				e2e.ExpectOutput(e2e.ContainMatch, "Fingerprint: F34371D0ACD5D09EB9BD853A80600A5FA11BBD29"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name:       "LegacyGroupIDFlagNotFound",
			flags:      []string{"--local", "--legacy-insecure", "--group-id", "5"},
			imagePath:  filepath.Join("..", "test", "images", "one-group-signed-legacy-group.sif"),
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "integrity: group not found"),
			},
		},
		{
			name:      "LegacyAllFlag",
			flags:     []string{"--local", "--legacy-insecure", "--all"},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-legacy-all.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with PGP key material"),
				e2e.ExpectOutput(e2e.ContainMatch, "Signing entity: SingularityCE Tests <singularityce@example.com>"),
				e2e.ExpectOutput(e2e.ContainMatch, "Fingerprint: F34371D0ACD5D09EB9BD853A80600A5FA11BBD29"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name:      "JSONFlag",
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-pgp.sif"),
			flags:     []string{"--local", "--json"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with PGP key material"),
			},
		},
		{
			name:      "KeyFlag",
			flags:     []string{"--key", pubKeyPath},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with key material from '"+pubKeyPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name:      "KeyEnvVar",
			envs:      []string{"APPTAINER_VERIFY_KEY=" + pubKeyPath},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with key material from '"+pubKeyPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name: "CertificateFlags",
			flags: []string{
				"--certificate", certPath,
				"--certificate-intermediates", intPath,
				"--certificate-roots", rootPath,
			},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with key material from certificate '"+certPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name: "CertificateEnvVars",
			envs: []string{
				"APPTAINER_VERIFY_CERTIFICATE=" + certPath,
				"APPTAINER_VERIFY_INTERMEDIATES=" + intPath,
				"APPTAINER_VERIFY_ROOTS=" + rootPath,
			},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with key material from certificate '"+certPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name: "OCSPFlags",
			flags: []string{
				"--certificate", certPath,
				"--certificate-intermediates", intPath,
				"--certificate-roots", rootPath,
				"--ocsp-verify",
			},
			imagePath:  filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			needOCSP:   true,
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				// Expect OCSP to fail due to https://github.com/sylabs/singularity/issues/1152
				e2e.ExpectError(e2e.ContainMatch, "Failed to verify container: OCSP verification has failed"),
			},
		},
		{
			name: "OCSPEnvVars",
			envs: []string{
				"APPTAINER_VERIFY_CERTIFICATE=" + certPath,
				"APPTAINER_VERIFY_INTERMEDIATES=" + intPath,
				"APPTAINER_VERIFY_ROOTS=" + rootPath,
				"APPTAINER_VERIFY_OCSP=true",
			},
			imagePath:  filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			needOCSP:   true,
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				// Expect OCSP to fail due to https://github.com/sylabs/singularity/issues/1152
				e2e.ExpectError(e2e.ContainMatch, "Failed to verify container: OCSP verification has failed"),
			},
		},
		{
			name: "OCSPThirdPartyChain",
			flags: []string{
				"--certificate", filepath.Join("./verify", "ocspcertificates", "leaf.pem"),
				"--certificate-intermediates", filepath.Join("./verify", "ocspcertificates", "intermediate.pem"),
				"--ocsp-verify",
			},
			imagePath:  filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			needOCSP:   true,
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Failed to verify container: x509: certificate specifies an incompatible key usage"),
				// https://github.com/sylabs/singularity/pull/1213#pullrequestreview-1240524316
				// Error Expect OCSP to succeed, but signature verification to fail.
				// e2e.ExpectError(e2e.ContainMatch, "Failed to verify container: integrity: signature object 3 not valid: dsse: verify envelope failed: Accepted signatures do not match threshold, Found: 0, Expected 1"),
			},
		},
	}

	for _, tt := range tests {
		c.RunApptainer(t,
			e2e.AsSubtest(tt.name),
			e2e.PreRun(func(t *testing.T) {
				if tt.needOCSP && !ocspOk {
					t.Skip("OCSP responder not available")
				}
			}),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithEnv(tt.envs),
			e2e.WithCommand("verify"),
			e2e.WithArgs(append(tt.flags, tt.imagePath)...),
			e2e.ExpectExit(tt.expectCode, tt.expectOps...),
		)
	}
}

func (c *ctx) importPGPKeypairs(t *testing.T) {
	c.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("key import"),
		e2e.WithArgs(filepath.Join("..", "test", "keys", "pgp-public.asc")),
		e2e.ExpectExit(0),
	)
}

func (c *ctx) startOCSPResponder(rootKeyPath string, rootCertPath string) error {
	responderErr := make(chan error, 1)

	// initiate OCSP responder to validate the apptainer certificate chain
	go func() {
		args := ocspresponder.ResponderArgs{
			IndexFile:    filepath.Join("./verify", "ocspresponder", "index.txt"),
			ServerPort:   "9999",
			OCSPKeyPath:  rootKeyPath,
			OCSPCertPath: rootCertPath,
			CACertPath:   rootCertPath,
		}

		if err := ocspresponder.StartOCSPResponder(args); err != nil {
			responderErr <- fmt.Errorf("responder initialization has failed due to '%s'", err)
		}
	}()

	// Assume if there's no error after 5 seconds then the responder is running.
	select {
	case err := <-responderErr:
		return err
	case <-time.After(5 * time.Second):
		return nil
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		TestEnv: env,
	}

	return testhelper.Tests{
		"ordered": func(t *testing.T) {
			var err error

			// Create a temporary PGP keyring.
			c.KeyringDir, err = os.MkdirTemp("", "e2e-sign-keyring-")
			if err != nil {
				t.Fatalf("failed to create temporary directory: %s", err)
			}
			defer func() {
				err := os.RemoveAll(c.KeyringDir)
				if err != nil {
					t.Fatalf("failed to delete temporary directory: %s", err)
				}
			}()

			c.importPGPKeypairs(t)

			t.Run("Verify", c.verify)
		},
	}
}
