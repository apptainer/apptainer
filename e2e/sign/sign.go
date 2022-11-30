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
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
)

type ctx struct {
	env             e2e.TestEnv
	keyringDir      string
	passphraseInput []e2e.ApptainerConsoleOp
}

const imgName = "testImage.sif"

func (c ctx) apptainerSignHelpOption(t *testing.T) {
	c.env.KeyringDir = c.keyringDir
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("sign"),
		e2e.WithArgs("--help"),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Add digital signature(s) to an image"),
		),
	)
}

func (c *ctx) prepareImage(t *testing.T) (string, func(*testing.T)) {
	// Get a refresh unsigned image
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	imgPath := filepath.Join(tempDir, imgName)

	err = fs.CopyFile(e2e.BusyboxSIF(t), imgPath, 0o755)
	if err != nil {
		t.Fatalf("failed to copy temporary image: %s", err)
	}

	return filepath.Join(tempDir, "testImage.sif"), func(t *testing.T) {
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Fatalf("failed to delete temporary directory: %s", err)
		}
	}
}

//nolint:dupl
func (c ctx) apptainerSignIDOption(t *testing.T) {
	imgPath, cleanup := c.prepareImage(t)
	defer cleanup(t)

	tests := []struct {
		name       string
		args       []string
		expectOp   e2e.ApptainerCmdResultOp
		expectExit int
	}{
		{
			name:       "sign deffile",
			args:       []string{"--sif-id", "1", imgPath},
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "Signature created and applied to "+imgPath),
			expectExit: 0,
		},
		{
			name:       "sign non-existent ID",
			args:       []string{"--sif-id", "99", imgPath},
			expectOp:   e2e.ExpectError(e2e.ContainMatch, "integrity: object not found"),
			expectExit: 255,
		},
	}

	c.env.KeyringDir = c.keyringDir

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("sign"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(c.passphraseInput...),
			e2e.ExpectExit(tt.expectExit, tt.expectOp),
		)
	}
}

func (c ctx) apptainerSignAllOption(t *testing.T) {
	imgPath, cleanup := c.prepareImage(t)
	defer cleanup(t)

	tests := []struct {
		name       string
		args       []string
		expectOp   e2e.ApptainerCmdResultOp
		expectExit int
	}{
		{
			name:       "sign default",
			args:       []string{imgPath},
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "Signature created and applied to "+imgPath),
			expectExit: 0,
		},
		{
			name:       "sign all",
			args:       []string{"--all", imgPath},
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "Signature created and applied to "+imgPath),
			expectExit: 0,
		},
	}

	c.env.KeyringDir = c.keyringDir

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("sign"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(c.passphraseInput...),
			e2e.ExpectExit(tt.expectExit, tt.expectOp),
		)
	}
}

//nolint:dupl
func (c ctx) apptainerSignGroupIDOption(t *testing.T) {
	imgPath, cleanup := c.prepareImage(t)
	defer cleanup(t)

	tests := []struct {
		name       string
		args       []string
		expectOp   e2e.ApptainerCmdResultOp
		expectExit int
	}{
		{
			name:       "groupID 0",
			args:       []string{"--group-id", "1", imgPath},
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "Signature created and applied to "+imgPath),
			expectExit: 0,
		},
		{
			name:       "groupID 5",
			args:       []string{"--group-id", "5", imgPath},
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "integrity: group not found"),
			expectExit: 255,
		},
	}

	c.env.KeyringDir = c.keyringDir

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("sign"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(c.passphraseInput...),
			e2e.ExpectExit(tt.expectExit, tt.expectOp),
		)
	}
}

func (c ctx) apptainerSignKeyidxOption(t *testing.T) {
	imgPath, cleanup := c.prepareImage(t)
	defer cleanup(t)

	cmdArgs := []string{"--keyidx", "0", imgPath}
	c.env.KeyringDir = c.keyringDir
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("sign"),
		e2e.WithArgs(cmdArgs...),
		e2e.ConsoleRun(c.passphraseInput...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Signature created and applied to "+imgPath),
		),
	)
}

func (c ctx) apptainerSignKeyOption(t *testing.T) {
	imgPath, cleanup := c.prepareImage(t)
	defer cleanup(t)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("sign"),
		e2e.WithArgs(
			"--key",
			filepath.Join("..", "test", "keys", "private.pem"),
			imgPath,
		),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Signature created and applied to "+imgPath),
		),
	)
}

func (c ctx) apptainerSignKeyEnv(t *testing.T) {
	imgPath, cleanup := c.prepareImage(t)
	defer cleanup(t)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithEnv([]string{"APPTAINER_SIGN_KEY=" + filepath.Join("..", "test", "keys", "private.pem")}),
		e2e.WithCommand("sign"),
		e2e.WithArgs(imgPath),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Signature created and applied to "+imgPath),
		),
	)
}

func (c *ctx) generateKeypair(t *testing.T) {
	keyGenInput := []e2e.ApptainerConsoleOp{
		e2e.ConsoleSendLine("e2e sign test key"),
		e2e.ConsoleSendLine("jdoe@sylabs.io"),
		e2e.ConsoleSendLine("sign e2e test"),
		e2e.ConsoleSendLine("passphrase"),
		e2e.ConsoleSendLine("passphrase"),
		e2e.ConsoleSendLine("n"),
	}

	c.env.KeyringDir = c.keyringDir
	c.env.RunApptainer(
		t,
		e2e.ConsoleRun(keyGenInput...),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("key"),
		e2e.WithArgs("newpair"),
		e2e.ExpectExit(0),
	)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"ordered": func(t *testing.T) {
			var err error
			// We need one single key pair in a single keyring for all the tests
			c.keyringDir, err = os.MkdirTemp("", "e2e-sign-keyring-")
			if err != nil {
				t.Fatalf("failed to create temporary directory: %s", err)
			}
			defer func() {
				err := os.RemoveAll(c.keyringDir)
				if err != nil {
					t.Fatalf("failed to delete temporary directory: %s", err)
				}
			}()
			c.generateKeypair(t)

			c.passphraseInput = []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("passphrase"),
			}
			t.Run("apptainerSignAllOption", c.apptainerSignAllOption)
			t.Run("apptainerSignHelpOption", c.apptainerSignHelpOption)
			t.Run("apptainerSignIDOption", c.apptainerSignIDOption)
			t.Run("apptainerSignGroupIDOption", c.apptainerSignGroupIDOption)
			t.Run("apptainerSignKeyidxOption", c.apptainerSignKeyidxOption)
			t.Run("apptainerSignKeyOption", c.apptainerSignKeyOption)
			t.Run("apptainerSignKeyEnv", c.apptainerSignKeyEnv)
		},
	}
}
