// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package key

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/syecl"
)

type ctx struct {
	env                    e2e.TestEnv
	publicExportPath       string
	publicExportASCIIPath  string
	privateExportPath      string
	privateExportASCIIPath string
	keyRing                string
}

func buildConsoleLines(lines ...string) []e2e.ApptainerConsoleOp {
	consoleLines := make([]e2e.ApptainerConsoleOp, 0, len(lines))
	for _, line := range lines {
		consoleLines = append(consoleLines, e2e.ConsoleSendLine(line))
	}

	return consoleLines
}

func (c *ctx) apptainerKeyList(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdout string
		exit   int
	}{
		{
			name:   "key list help",
			args:   []string{"list", "--help"},
			stdout: "^List keys in your local or in the global keyring",
		},
		{
			name:   "key list",
			args:   []string{"list"},
			stdout: "^Public key listing",
		},
		{
			name:   "key list secret",
			args:   []string{"list", "--secret"},
			stdout: "^Private key listing",
		},
		{
			name:   "key list global secret",
			args:   []string{"list", "--global", "--secret"},
			stdout: "^Private key listing",
			exit:   255,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exit, e2e.ExpectOutput(e2e.RegexMatch, tt.stdout)),
		)
	}
}

func (c *ctx) apptainerKeySearch(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stdout string
	}{
		{
			name:   "key search help",
			args:   []string{"search", "--help"},
			stdout: "^Search for keys on a key server",
		},
		{
			name:   "key search 0x<key fingerprint>",
			args:   []string{"search", "-u", "https://keys.production.sycloud.io", "0x7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search <key fingerprint>",
			args:   []string{"search", "-u", "https://keys.production.sycloud.io", "7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search -u https://keys.openpgp.org 0x<key fingerprint>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "0x7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search -u https://keys.openpgp.org <key fingerprint>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "7605BC2716168DF057D6C600ACEEC62C8BD91BEE"},
			stdout: "^Showing 1 results",
		},
		{
			name:   "key search <key with at least two emails>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "dwd@fnal.gov"},
			stdout: "\n  .*@",
		},
		{
			name:   "key search -l <key with at least two emails>",
			args:   []string{"search", "-u", "https://keys.openpgp.org", "-l", "dwd@fnal.gov"},
			stdout: "\n  .*@",
		},
		{
			name:   "key search --url <open key server> <name>",
			args:   []string{"search", "--url", "https://keyserver.ubuntu.com", "ftpmaster@ubuntu.com"},
			stdout: "^Showing",
		},
		{
			name:   "key search --url <open key server> <key id>",
			args:   []string{"search", "--url", "https://keyserver.ubuntu.com", "0x991BC93C"},
			stdout: "^Showing 1 results",
		},
		// TODO: add tests for --long-list after #4156 is solved
		//{
		//	name:   "key search --long-list <key id>",
		//	args:   []string{"search", "--long-list", "0x0x8BD91BEE"},
		//	stdout: "^Showing 1 results",
		//},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.RegexMatch, tt.stdout)),
		)
	}
}

func (c *ctx) apptainerKeyNewpair(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdout     string
		consoleOps []string
	}{
		{
			name:   "newpair help",
			args:   []string{"newpair", "--help"},
			stdout: "Create a new key pair",
		},
		{
			name: "newpair",
			args: []string{"newpair"},
			consoleOps: []string{
				"e2e test key",
				"westley@apptainer.org",
				"for e2e tests",
				"e2etests",
				"e2etests",
			},
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.ContainMatch, tt.stdout)),
		)
	}
}

// apptainerKeyExport will export a private, and public (binary and ASCII) key.
func (c *ctx) apptainerKeyExport(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdout     string
		consoleOps []string
	}{
		{
			name:   "export help",
			args:   []string{"export", "--help"},
			stdout: "Export a public or private key into a specific file",
		},
		{
			name: "export public binary",
			args: []string{"export", c.publicExportPath},
			consoleOps: []string{
				"0",
			},
			stdout: "Public key with fingerprint",
		},
		{
			name: "export private binary",
			args: []string{"export", "--secret", c.privateExportPath},
			consoleOps: []string{
				"0",
				"e2etests",
			},
			stdout: "Private key with fingerprint",
		},
		{
			name: "export public ascii",
			args: []string{"export", "--armor", c.publicExportASCIIPath},
			consoleOps: []string{
				"0",
			},
			stdout: "Public key with fingerprint",
		},
		{
			name: "export private ascii",
			args: []string{"export", "--secret", "--armor", c.privateExportASCIIPath},
			consoleOps: []string{
				"0",
				"e2etests",
			},
			stdout: "Private key with fingerprint",
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.ContainMatch, tt.stdout)),
		)
	}
}

// apptainerKeyImport will export a private, and public (binary and ASCII) key.
// And will try (and fail) to import a key with the wrong password.
func (c *ctx) apptainerKeyImport(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		consoleOps []string
		stdout     string
		expectExit int
	}{
		{
			name:       "import help",
			args:       []string{"import", "--help"},
			stdout:     "Import a local key into the local or global keyring",
			expectExit: 0,
		},
		{
			name:       "import public binary",
			args:       []string{"import", c.publicExportPath},
			stdout:     "successfully added to the public keyring",
			expectExit: 0,
		},
		{
			name: "import private binary wrong password",
			args: []string{"import", c.privateExportPath},
			consoleOps: []string{
				"theWrongPassword", // The wrong password to decrypt the key (will fail)
				"somethingElse",
				"somethingElse",
			},
			stdout:     "openpgp: invalid data: private key checksum failure",
			expectExit: 2,
		},
		{
			name: "import private binary",
			args: []string{"import", c.privateExportPath},
			consoleOps: []string{
				"e2etests", // The password to decrypt the key
				"e2etests", // Then the new password
				"e2etests", // Confirm the password
			},
			stdout:     "successfully added to the private keyring",
			expectExit: 0,
		},
		{
			name:       "import public ascii",
			args:       []string{"import", c.publicExportASCIIPath},
			expectExit: 0,
			stdout:     "successfully added to the public keyring",
		},
		{
			name: "import private ascii wrong password",
			args: []string{"import", c.privateExportASCIIPath},
			consoleOps: []string{
				"theWrongPassword", // The wrong password to decrypt the key (will fail)
				"somethingElse",
				"somethingElse",
			},
			stdout:     "openpgp: invalid data: private key checksum failure",
			expectExit: 2,
		},
		{
			name: "import private ascii",
			args: []string{"import", c.privateExportASCIIPath},
			consoleOps: []string{
				"e2etests", // The password to decrypt the key
				"e2etests", // Then the new password
				"e2etests", // Confirm the password
			},
			stdout:     "successfully added to the private keyring",
			expectExit: 0,
		},
	}

	for _, tt := range tests {
		c.apptainerResetKeyring(t) // Remove the tmp keyring before each import
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.ExpectExit(tt.expectExit, e2e.ExpectOutput(e2e.ContainMatch, tt.stdout)),
		)
	}
}

func (c *ctx) apptainerResetKeyring(t *testing.T) {
	// TODO: run this as non-root
	err := os.RemoveAll(c.keyRing)
	if os.IsNotExist(err) && err != nil {
		t.Fatalf("unable to remove tmp keyring directory: %s", err)
	}
}

func (c *ctx) apptainerKeyPush(t *testing.T) {
	tests := []struct {
		name          string
		cmdArgs       []string
		expectedExit  int
		expectedRegex string
	}{
		{
			name:          "push help",
			cmdArgs:       []string{"--help"},
			expectedExit:  0,
			expectedRegex: `^Upload a public key to a key server`,
		},
	}
	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.AsSubtest(tt.name),
			e2e.WithCommand("key"),
			e2e.WithArgs(append([]string{"push"}, tt.cmdArgs...)...),
			e2e.ExpectExit(tt.expectedExit, e2e.ExpectOutput(e2e.RegexMatch, tt.expectedRegex)),
		)
	}
}

func (c *ctx) apptainerKeyPull(t *testing.T) {
	tests := []struct {
		name          string
		cmdArgs       []string
		expectedExit  int
		expectedRegex string
	}{
		{
			name:          "pull help",
			cmdArgs:       []string{"--help"},
			expectedExit:  0,
			expectedRegex: `^Download a public key from a key server`,
		},
	}
	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.AsSubtest(tt.name),
			e2e.WithCommand("key"),
			e2e.WithArgs(append([]string{"pull"}, tt.cmdArgs...)...),
			e2e.ExpectExit(tt.expectedExit, e2e.ExpectOutput(e2e.RegexMatch, tt.expectedRegex)),
		)
	}
}

func (c *ctx) apptainerKeyRemove(t *testing.T) {
	tests := []struct {
		name          string
		cmdArgs       []string
		expectedExit  int
		expectedRegex string
	}{
		{
			name:          "remove help",
			cmdArgs:       []string{"--help"},
			expectedExit:  0,
			expectedRegex: `^Remove a local public key from your local or the global keyring`,
		},
	}
	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.AsSubtest(tt.name),
			e2e.WithCommand("key"),
			e2e.WithArgs(append([]string{"remove"}, tt.cmdArgs...)...),
			e2e.ExpectExit(tt.expectedExit, e2e.ExpectOutput(e2e.RegexMatch, tt.expectedRegex)),
		)
	}
}

func (c *ctx) apptainerKeyRemoveOpts(t *testing.T) {
	keyMap := map[string]string{
		"key1": "0C5B8C9A5FFC44E2A0AC79851CD6FA281D476DD1",
		"key2": "78F8AD36B0DCB84B707F23853D608DAE21C8CA10",
	}

	tests := []struct {
		name               string
		command            string
		args               []string
		profile            e2e.Profile
		consoleOps         []e2e.ApptainerConsoleOp
		resultOp           []e2e.ApptainerCmdResultOp
		config             *syecl.EclConfig
		expectedRegex      string
		expectedErrorRegex string
		exit               int
	}{
		{
			name:    "import pubkey1 as user should succeed",
			command: "key import",
			profile: e2e.UserProfile,
			args:    []string{"testdata/ecl-pgpkeys/pubkey1.asc"},
			exit:    0,
		},
		{
			name:    "import pubkey2 as user should succeed",
			command: "key import",
			profile: e2e.UserProfile,
			args:    []string{"testdata/ecl-pgpkeys/pubkey2.asc"},
			exit:    0,
		},
		{
			name:    "remove public key should succeed",
			command: "key remove",
			profile: e2e.UserProfile,
			args:    []string{"--public", keyMap["key1"]},
			exit:    0,
		},
		{
			name:    "remove both should succeed",
			command: "key remove",
			profile: e2e.UserProfile,
			args:    []string{"--both", keyMap["key2"]},
			exit:    0,
		},
		{
			name:    "import key1 as user should succeed",
			command: "key import",
			profile: e2e.UserProfile,
			args:    []string{"testdata/ecl-pgpkeys/key1.asc"},
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("e2e"),
			},
			exit: 0,
		},
		{
			name:    "remove secret key should succeed",
			command: "key remove",
			profile: e2e.UserProfile,
			args:    []string{"--secret", keyMap["key1"]},
			exit:    0,
		},
		{
			name:    "import key2 as user should succeed",
			command: "key import",
			profile: e2e.UserProfile,
			args:    []string{"testdata/ecl-pgpkeys/key2.asc"},
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("e2e"),
			},
			exit: 0,
		},
		{
			name:    "remove both should succeed",
			command: "key remove",
			profile: e2e.UserProfile,
			args:    []string{"--both", keyMap["key2"]},
			exit:    0,
		},
		{
			name:          "key list should return empty because all keys have been removed",
			command:       "key list",
			profile:       e2e.UserProfile,
			expectedRegex: "^Public key listing.*\n",
			exit:          0,
		},
		{
			name:          "key list --secret should return empty because all keys have been removed",
			command:       "key list",
			profile:       e2e.UserProfile,
			args:          []string{"--secret"},
			expectedRegex: "^Private key listing.*\n",
			exit:          0,
		},
		{
			name:               "remove secret key should fail because key does not exist in keyring",
			command:            "key remove",
			profile:            e2e.UserProfile,
			args:               []string{"--secret", keyMap["key1"]},
			expectedErrorRegex: "FATAL:",
			exit:               255,
		},
		{
			name:    "import pubkey1 as global user (regression test)",
			command: "key import",
			profile: e2e.RootProfile,
			args:    []string{"--global", "testdata/ecl-pgpkeys/pubkey1.asc"},
			exit:    0,
		},
		{
			name:    "remove pubkey1 from global (regression test)",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--global", "--public", keyMap["key1"]},
			exit:    0,
		},
	}

	for _, tt := range tests {
		cmdOps := []e2e.ApptainerCmdOp{
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.PreRun(func(t *testing.T) {
				if tt.config == nil {
					return
				}
				fn := func(t *testing.T) {
					if err := tt.config.ValidateConfig(); err != nil {
						t.Errorf("while validating ecl config: %s", err)
					}
					err := syecl.PutConfig(*tt.config, buildcfg.ECL_FILE)
					if err != nil {
						t.Errorf("while creating ecl config: %s", err)
					}
				}
				e2e.Privileged(fn)(t)
			}),
			e2e.ExpectExit(tt.exit, e2e.ExpectOutput(e2e.RegexMatch, tt.expectedRegex), e2e.ExpectError(e2e.ContainMatch, tt.expectedErrorRegex)),
		}

		if tt.consoleOps != nil {
			cmdOps = append(cmdOps, e2e.ConsoleRun(tt.consoleOps...))
		}

		c.env.RunApptainer(
			t,
			cmdOps...,
		)
	}
}

func (c ctx) apptainerKeyNewpairWithLen(t *testing.T) {
	// Create a unique keyring shared for all these tests
	tempKeyring, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "keyring-", "")
	defer cleanup(t)
	c.env.KeyringDir = tempKeyring

	tests := []struct {
		name              string
		args              []string
		stdout            string
		consoleOps        []string
		expectedKeyLength int
	}{
		{
			name: "newpair bitlength 1024",
			args: []string{"newpair", "--bit-length", "1024"},
			consoleOps: []string{
				"e2e test key",
				"jdoe@apptainer.org",
				" for e2e tests",
				"e2etests",
				"e2etests",
				"n",
			},
			expectedKeyLength: 1024,
		},
		{
			name: "newpair bitlength 0",
			args: []string{"newpair", "--bit-length", "0"},
			consoleOps: []string{
				"e2e test key",
				"jdoe@apptainer.org",
				" for e2e tests",
				"e2etests",
				"e2etests",
				"n",
			},
			expectedKeyLength: 2048,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.AsSubtest(tt.name),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.WithCommand("key"),
			e2e.WithArgs(tt.args...),
			e2e.PostRun(func(t *testing.T) {
				c.checkKeyLength(t, tt.expectedKeyLength)
				c.apptainerResetKeyring(t)
			}),
			e2e.ExpectExit(0, e2e.ExpectOutput(e2e.RegexMatch, tt.stdout)),
		)
	}
}

func (c *ctx) checkKeyLength(t *testing.T, expectedKeyLength int) {
	if expectedKeyLength >= 0 {
		cmdArgs := []string{"list"}
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("key"),
			e2e.WithArgs(cmdArgs...),
			e2e.ExpectExit(
				0,
				e2e.ExpectOutput(e2e.ContainMatch, "L: "+strconv.Itoa(expectedKeyLength)),
			),
		)
	}
}

func (c *ctx) globalKeyring(t *testing.T) {
	keyMap := map[string]string{
		"key1": "0C5B8C9A5FFC44E2A0AC79851CD6FA281D476DD1",
		"key2": "78F8AD36B0DCB84B707F23853D608DAE21C8CA10",
	}

	tests := []struct {
		name     string
		command  string
		args     []string
		profile  e2e.Profile
		resultOp []e2e.ApptainerCmdResultOp
		exit     int
	}{
		{
			name:    "import pubkey1 global as user",
			command: "key import",
			profile: e2e.UserProfile,
			args:    []string{"--global", "testdata/ecl-pgpkeys/pubkey1.asc"},
			exit:    255,
		},
		{
			name:    "import pubkey1 global",
			command: "key import",
			profile: e2e.RootProfile,
			args:    []string{"--global", "testdata/ecl-pgpkeys/pubkey1.asc"},
			exit:    0,
		},
		{
			name:    "import pubkey2 global",
			command: "key import",
			profile: e2e.RootProfile,
			args:    []string{"--global", "testdata/ecl-pgpkeys/pubkey2.asc"},
			exit:    0,
		},
		{
			name:    "list global",
			command: "key list",
			profile: e2e.UserProfile,
			args:    []string{"--global"},
			resultOp: []e2e.ApptainerCmdResultOp{
				e2e.ExpectOutput(e2e.ContainMatch, keyMap["key1"]),
				e2e.ExpectOutput(e2e.ContainMatch, keyMap["key2"]),
			},
			exit: 0,
		},
		{
			name:    "newpair with global flag",
			command: "key newpair",
			profile: e2e.RootProfile,
			args:    []string{"--global"},
			exit:    1,
		},
		{
			name:    "search with global flag",
			command: "key search",
			profile: e2e.UserProfile,
			args:    []string{"--global", "test"},
			exit:    1,
		},
		{
			name:    "remove unknown from global",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--global", "0100000000000001"},
			exit:    255,
		},
		{
			name:    "remove pubkey1 from global",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--global", keyMap["key1"]},
			exit:    0,
		},
		{
			name:    "remove pubkey2 from global",
			command: "key remove",
			profile: e2e.RootProfile,
			args:    []string{"--global", keyMap["key2"]},
			exit:    0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exit, tt.resultOp...),
		)
	}
}

// As we support setting keyring dir path through the cli (add a flag '--keysdir'), this test is for regression purpose
func (c *ctx) apptainerLocalKeyDirFlagRegression(t *testing.T) {
	tempKeyring, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "regression-keyring-", "")
	keysdir := fmt.Sprintf("%s/newpair", tempKeyring)
	defer cleanup(t)

	tests := []struct {
		name       string
		command    string
		args       []string
		consoleOps []string
		resultOp   []e2e.ApptainerCmdResultOp
		exit       int
	}{
		{
			name:    "newpair regression with customized keydirs",
			command: "key newpair",
			args:    []string{"--keysdir", keysdir},
			consoleOps: []string{
				"e2e test key",
				"westley@apptainer.org",
				"for e2e tests",
				"e2etests",
				"e2etests",
			},
			exit: 0,
		},
		{
			name:    "key list regression test should succeed and return value",
			command: "key list",
			args:    []string{"--secret", "--keysdir", keysdir},
			resultOp: []e2e.ApptainerCmdResultOp{
				e2e.ExpectOutput(e2e.ContainMatch, "e2e test key"),
			},
			exit: 0,
		},
		{
			name:    "key list regression test should succeed but should return nothing because keysdir value is invalid",
			command: "key list",
			args:    []string{"--secret"},
			resultOp: []e2e.ApptainerCmdResultOp{
				e2e.ExpectOutput(e2e.UnwantedContainMatch, "e2e test key"),
			},
			exit: 0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.ConsoleRun(buildConsoleLines(tt.consoleOps...)...),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.exit, tt.resultOp...),
		)
	}
}

// Run the 'key' tests in order
func (c ctx) apptainerKeyCmd(t *testing.T) {
	c.apptainerKeySearch(t)
	c.apptainerKeyList(t)
	c.apptainerKeyNewpair(t)
	c.apptainerKeyExport(t)
	c.apptainerKeyImport(t)
	c.apptainerKeyExport(t)
	c.apptainerKeyImport(t)
	c.apptainerKeyList(t)
	c.apptainerKeyPull(t)
	c.apptainerKeyPush(t)
	c.apptainerKeyRemove(t)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env:                    env,
		publicExportPath:       filepath.Join(env.TestDir, "public_key.asc"),
		publicExportASCIIPath:  filepath.Join(env.TestDir, "public_ascii_key.asc"),
		privateExportPath:      filepath.Join(env.TestDir, "private_key.asc"),
		privateExportASCIIPath: filepath.Join(env.TestDir, "private_ascii_key.asc"),
		keyRing:                filepath.Join(env.TestDir, "sypgp-test-keyring"),
	}
	c.env.KeyringDir = c.keyRing

	return testhelper.Tests{
		"global": testhelper.NoParallel(c.globalKeyring), // global keyring
		"ordered": func(t *testing.T) {
			t.Run("keyCmd", c.apptainerKeyCmd)                                 // Run all the tests in order
			t.Run("keyNewpairWithLen", c.apptainerKeyNewpairWithLen)           // We run a separate test for `key newpair --bit-length` because it requires handling a keyring a specific way
			t.Run("keyRemoveOpts", c.apptainerKeyRemoveOpts)                   // run a separated test for `key remove --public/--secret/--both`
			t.Run("keyDirCmdRegression", c.apptainerLocalKeyDirFlagRegression) // run a separated test for regression purpose after we add a new feature of manually setting --keysdir through cli
		},
	}
}
