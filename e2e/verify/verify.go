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
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	e2e.TestEnv
}

func (c *ctx) verify(t *testing.T) {
	keyPath := filepath.Join("..", "test", "keys", "ed25519-public.pem")

	tests := []struct {
		name       string
		envs       []string
		flags      []string
		imagePath  string
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
			flags:     []string{"--key", keyPath},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with key material from '"+keyPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
		{
			name:      "KeyEnvVar",
			envs:      []string{"APPTAINER_VERIFY_KEY=" + keyPath},
			imagePath: filepath.Join("..", "test", "images", "one-group-signed-dsse.sif"),
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Verifying image with key material from '"+keyPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Verified signature(s) from image"),
			},
		},
	}

	for _, tt := range tests {
		c.RunApptainer(t,
			e2e.AsSubtest(tt.name),
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
