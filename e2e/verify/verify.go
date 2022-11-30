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
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/buger/jsonparser"
	"github.com/pkg/errors"
)

type ctx struct {
	env            e2e.TestEnv
	corruptedImage string
	successImage   string
}

type verifyOutput struct {
	name        string
	fingerprint string
	local       bool
	keyCheck    bool
	dataCheck   bool
}

const (
	successURL   = "oras://ghcr.io/apptainer/verify_success:1.1.0"
	corruptedURL = "oras://ghcr.io/apptainer/verify_corrupted:1.0.2"
)

func getNameJSON(keyNum int) []string {
	return []string{"SignerKeys", fmt.Sprintf("[%d]", keyNum), "Signer", "Name"}
}

func getFingerprintJSON(keyNum int) []string {
	return []string{"SignerKeys", fmt.Sprintf("[%d]", keyNum), "Signer", "Fingerprint"}
}

func getLocalJSON(keyNum int) []string {
	return []string{"SignerKeys", fmt.Sprintf("[%d]", keyNum), "Signer", "KeyLocal"}
}

func getKeyCheckJSON(keyNum int) []string {
	return []string{"SignerKeys", fmt.Sprintf("[%d]", keyNum), "Signer", "KeyCheck"}
}

func getDataCheckJSON(keyNum int) []string {
	return []string{"SignerKeys", fmt.Sprintf("[%d]", keyNum), "Signer", "DataCheck"}
}

func (c ctx) apptainerVerifyAllKeyNum(t *testing.T) {
	keyNumPath := []string{"Signatures"}

	tests := []struct {
		name         string
		expectNumOut int64  // Is the expected number of Signatures
		imageURL     string // Is the URL to the container
		imagePath    string // Is the path to the container
		expectExit   int
	}{
		{
			name:         "verify number signers fail",
			expectNumOut: 0,
			imageURL:     corruptedURL,
			imagePath:    c.corruptedImage,
			expectExit:   255,
		},
		{
			name:         "verify number signers success",
			expectNumOut: 2,
			imageURL:     successURL,
			imagePath:    c.successImage,
			expectExit:   0,
		},
	}

	for _, tt := range tests {
		if !fs.IsFile(tt.imagePath) {
			t.Fatalf("image file (%s) does not exist", tt.imagePath)
		}

		verifyOutput := func(t *testing.T, r *e2e.ApptainerCmdResult) {
			// Get the Signatures and compare it
			eNum, err := jsonparser.GetInt(r.Stdout, keyNumPath...)
			if err != nil {
				err = errors.Wrap(err, "getting key number from JSON")
				t.Fatalf("unable to get expected output from json: %+v", err)
			}
			if eNum != tt.expectNumOut {
				t.Fatalf("unexpected failure: got: '%d', expecting: '%d'", eNum, tt.expectNumOut)
			}
		}

		// Inspect the container, and get the output
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("verify"),
			e2e.WithArgs("--legacy-insecure", "--all", "--json", tt.imagePath),
			e2e.ExpectExit(tt.expectExit, verifyOutput),
		)
	}
}

func (c ctx) apptainerVerifySigner(t *testing.T) {
	tests := []struct {
		expectOutput []verifyOutput
		name         string
		imagePath    string
		imageURL     string
		expectExit   int
		verifyLocal  bool
	}{
		// corrupted verify
		{
			name:         "corrupted signatures",
			verifyLocal:  false,
			imageURL:     corruptedURL,
			imagePath:    c.corruptedImage,
			expectExit:   255,
			expectOutput: []verifyOutput{},
		},

		// corrupted verify with --local
		{
			name:         "corrupted signatures local",
			imageURL:     corruptedURL,
			imagePath:    c.corruptedImage,
			verifyLocal:  true,
			expectExit:   255,
			expectOutput: []verifyOutput{},
		},

		// Verify 'verify_container_success.sif'
		{
			name:        "verify success",
			verifyLocal: false,
			imageURL:    successURL,
			imagePath:   c.successImage,
			expectExit:  0,
			expectOutput: []verifyOutput{
				{
					name:        "Dave Dykstra",
					fingerprint: "1b0f8f770b92a4672059ff8234a4a0a7189c0e94",
					local:       false,
					keyCheck:    true,
					dataCheck:   true,
				},
			},
		},

		// Verify 'verify_container_success.sif' with --local
		{
			name:         "verify non local fail",
			imageURL:     successURL,
			imagePath:    c.successImage,
			verifyLocal:  true,
			expectExit:   255,
			expectOutput: []verifyOutput{},
		},
	}

	for _, tt := range tests {
		verifyOutput := func(t *testing.T, r *e2e.ApptainerCmdResult) {
			for keyNum, vo := range tt.expectOutput {
				eName, err := jsonparser.GetString(r.Stdout, getNameJSON(keyNum)...)
				if err != nil {
					err = errors.Wrap(err, "getting string from JSON")
					t.Fatalf("unable to get expected output from json: %+v", err)
				}
				if !strings.HasPrefix(eName, vo.name) {
					t.Fatalf("unexpected failure: got: '%s', expecting to start with: '%s'", eName, vo.name)
				}

				// Get the Fingerprint and compare it
				eFingerprint, err := jsonparser.GetString(r.Stdout, getFingerprintJSON(keyNum)...)
				if err != nil {
					err = errors.Wrap(err, "getting string from JSON")
					t.Fatalf("unable to get expected output from json: %+v", err)
				}
				if eFingerprint != vo.fingerprint {
					t.Fatalf("unexpected failure: got: '%s', expecting: '%s'", eFingerprint, vo.fingerprint)
				}

				// Get the Local and compare it
				eLocal, err := jsonparser.GetBoolean(r.Stdout, getLocalJSON(keyNum)...)
				if err != nil {
					err = errors.Wrap(err, "getting boolean from JSON")
					t.Fatalf("unable to get expected output from json: %+v", err)
				}
				if eLocal != vo.local {
					t.Fatalf("unexpected failure: got: '%v', expecting: '%v'", eLocal, vo.local)
				}

				// Get the KeyCheck and compare it
				eKeyCheck, err := jsonparser.GetBoolean(r.Stdout, getKeyCheckJSON(keyNum)...)
				if err != nil {
					err = errors.Wrap(err, "getting boolean from JSON")
					t.Fatalf("unable to get expected output from json: %+v", err)
				}
				if eKeyCheck != vo.keyCheck {
					t.Fatalf("unexpected failure: got: '%v', expecting: '%v'", eKeyCheck, vo.keyCheck)
				}

				// Get the DataCheck and compare it
				eDataCheck, err := jsonparser.GetBoolean(r.Stdout, getDataCheckJSON(keyNum)...)
				if err != nil {
					err = errors.Wrap(err, "getting boolean from JSON")
					t.Fatalf("unable to get expected output from json: %+v", err)
				}
				if eDataCheck != vo.dataCheck {
					t.Fatalf("unexpected failure: got: '%v', expecting: '%v'", eDataCheck, vo.dataCheck)
				}
			}
		}

		if !fs.IsFile(tt.imagePath) {
			t.Fatalf("image file (%s) does not exist", tt.imagePath)
		}

		args := []string{"--legacy-insecure", "--json"}
		if tt.verifyLocal {
			args = append(args, "--local")
		}
		args = append(args, tt.imagePath)

		// Inspect the container, and get the output
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("verify"),
			e2e.WithArgs(args...),
			e2e.ExpectExit(tt.expectExit, verifyOutput),
		)
	}
}

func (c ctx) checkGroupidOption(t *testing.T) {
	cmdArgs := []string{"--legacy-insecure", "--group-id", "1", c.successImage}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("verify"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.RegexMatch, "Container verified: .*/verify_success.sif"),
		),
	)
}

func (c ctx) checkIDOption(t *testing.T) {
	cmdArgs := []string{"--legacy-insecure", "--sif-id", "1", c.successImage}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("verify"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.RegexMatch, "Container verified: .*/verify_success.sif"),
		),
	)
}

func (c ctx) checkAllOption(t *testing.T) {
	cmdArgs := []string{"--legacy-insecure", "--all", c.successImage}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("verify"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.RegexMatch, "Container verified: .*/verify_success.sif"),
		),
	)
}

func (c ctx) checkURLOption(t *testing.T) {
	if !fs.IsFile(c.successImage) {
		t.Fatalf("image file (%s) does not exist", c.successImage)
	}

	cmdArgs := []string{"--legacy-insecure", "--url", "https://keys.production.sycloud.io", c.successImage}
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("verify"),
		e2e.WithArgs(cmdArgs...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.RegexMatch, "Container verified: .*/verify_success.sif"),
		),
	)
}

func (c ctx) apptainerVerifyKeyOption(t *testing.T) {
	imagePath := filepath.Join("..", "test", "images", "one-group-signed-dsse.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("verify"),
		e2e.WithArgs(
			"--key",
			filepath.Join("..", "test", "keys", "public.pem"),
			imagePath,
		),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Container verified: "+imagePath),
		),
	)
}

func (c ctx) apptainerVerifyKeyEnv(t *testing.T) {
	imagePath := filepath.Join("..", "test", "images", "one-group-signed-dsse.sif")

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithEnv([]string{"APPTAINER_VERIFY_KEY=" + filepath.Join("..", "test", "keys", "public.pem")}),
		e2e.WithCommand("verify"),
		e2e.WithArgs(imagePath),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "Container verified: "+imagePath),
		),
	)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env:            env,
		corruptedImage: filepath.Join(env.TestDir, "verify_corrupted.sif"),
		successImage:   filepath.Join(env.TestDir, "verify_success.sif"),
	}

	return testhelper.Tests{
		"ordered": func(t *testing.T) {
			// We pull the two images required for the tests once
			// We should be able to sign amd64 on other archs too!
			e2e.PullImage(t, c.env, successURL, "amd64", c.successImage)
			e2e.PullImage(t, c.env, corruptedURL, "amd64", c.corruptedImage)

			t.Run("checkAllOption", c.checkAllOption)
			t.Run("apptainerVerifyAllKeyNum", c.apptainerVerifyAllKeyNum)
			t.Run("apptainerVerifySigner", c.apptainerVerifySigner)
			t.Run("apptainerVerifyGroupIdOption", c.checkGroupidOption)
			t.Run("apptainerVerifyIDOption", c.checkIDOption)
			t.Run("apptainerVerifyURLOption", c.checkURLOption)
			t.Run("apptainerVerifyKeyOption", c.apptainerVerifyKeyOption)
			t.Run("apptainerVerifyKeyEnv", c.apptainerVerifyKeyEnv)
		},
	}
}
