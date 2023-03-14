// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sign

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	e2e.TestEnv
}

func getImage(t *testing.T) string {
	dst, err := os.CreateTemp("", "e2e-sign-keyring-*")
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	src, err := os.Open(filepath.Join("..", "test", "images", "one-group.sif"))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}

	return dst.Name()
}

func (c *ctx) sign(t *testing.T) {
	keyPath := filepath.Join("..", "test", "keys", "ed25519-private.pem")

	tests := []struct {
		name       string
		envs       []string
		flags      []string
		expectCode int
		expectOps  []e2e.ApptainerCmdResultOp
	}{
		{
			name:  "Help",
			flags: []string{"--help"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectOutput(e2e.ContainMatch, "Add digital signature(s) to an image"),
			},
		},
		{
			name: "OK",
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "Signature created and applied"),
			},
		},
		{
			name:  "ObjectIDFlag",
			flags: []string{"--sif-id", "1"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "Signature created and applied"),
			},
		},
		{
			name:       "ObjectIDFlagNotFound",
			flags:      []string{"--sif-id", "9"},
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "integrity: object not found"),
			},
		},
		{
			name:  "GroupIDFlag",
			flags: []string{"--group-id", "1"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "Signature created and applied"),
			},
		},
		{
			name:       "GroupIDFlagNotFound",
			flags:      []string{"--group-id", "5"},
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "integrity: group not found"),
			},
		},
		{
			name:  "AllFlag",
			flags: []string{"--all"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "Signature created and applied"),
			},
		},
		{
			name:  "KeyIndexFlag",
			flags: []string{"--keyidx", "0"},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "Signature created and applied"),
			},
		},
		{
			name:       "KeyIndexFlagOutOfRange",
			flags:      []string{"--keyidx", "1"},
			expectCode: 255,
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with PGP key material"),
				e2e.ExpectError(e2e.ContainMatch, "Failed to sign container: index out of range"),
			},
		},
		{
			name:  "KeyFlag",
			flags: []string{"--key", keyPath},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with key material from '"+keyPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Signature created and applied"),
			},
		},
		{
			name: "KeyEnvVar",
			envs: []string{"APPTAINER_SIGN_KEY=" + keyPath},
			expectOps: []e2e.ApptainerCmdResultOp{
				e2e.ExpectError(e2e.ContainMatch, "Signing image with key material from '"+keyPath+"'"),
				e2e.ExpectError(e2e.ContainMatch, "Signature created and applied"),
			},
		},
	}

	for _, tt := range tests {
		imgPath := getImage(t)
		defer os.Remove(imgPath)

		c.RunApptainer(t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithEnv(tt.envs),
			e2e.WithCommand("sign"),
			e2e.WithArgs(append(tt.flags, imgPath)...),
			e2e.ExpectExit(tt.expectCode, tt.expectOps...),
		)
	}
}

func (c *ctx) importPGPKeypairs(t *testing.T) {
	c.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("key import"),
		e2e.WithArgs(filepath.Join("..", "test", "keys", "pgp-private.asc")),
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

			t.Run("Sign", c.sign)
		},
	}
}
