// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package remote

import (
	"log"
	"os"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	env e2e.TestEnv
}

// remoteAdd checks the functionality of "apptainer remote add" command.
// It Verifies that adding valid endpoints results in success and invalid
// one's results in failure.
func (c ctx) remoteAdd(t *testing.T) {
	config, err := os.CreateTemp(c.env.TestDir, "testConfig-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(config.Name()) // clean up

	testPass := []struct {
		name   string
		remote string
		uri    string
	}{
		{"AddCloud", "cloud", "cloud.sycloud.io"},
		{"AddOtherCloud", "other", "cloud.sycloud.io"},
	}

	for _, tt := range testPass {
		argv := []string{"--config", config.Name(), "add", "--no-login", tt.remote, tt.uri}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testFail := []struct {
		name   string
		remote string
		uri    string
	}{
		{"AddExistingRemote", "cloud", "cloud.sycloud.io"},
		{"AddExistingRemoteInvalidURI", "other", "anythingcangohere"},
	}

	for _, tt := range testFail {
		argv := []string{"--config", config.Name(), "add", "--no-login", tt.remote, tt.uri}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(255),
		)
	}
}

// remoteRemove tests the functionality of "apptainer remote remove" command.
// 1. Adds remote endpoints
// 2. Deletes the already added entries
// 3. Verifies that removing an invalid entry results in a failure
func (c ctx) remoteRemove(t *testing.T) {
	config, err := os.CreateTemp(c.env.TestDir, "testConfig-")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(config.Name()) // clean up

	// Prep config by adding multiple remotes
	add := []struct {
		name   string
		remote string
		uri    string
	}{
		{"addCloud", "cloud", "cloud.sycloud.io"},
		{"addOther", "other", "cloud.sycloud.io"},
	}

	for _, tt := range add {
		argv := []string{"--config", config.Name(), "add", "--no-login", tt.remote, tt.uri}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testPass := []struct {
		name   string
		remote string
	}{
		{"RemoveCloud", "cloud"},
		{"RemoveOther", "other"},
	}

	for _, tt := range testPass {
		argv := []string{"--config", config.Name(), "remove", tt.remote}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testFail := []struct {
		name   string
		remote string
	}{
		{"RemoveNonExistingRemote", "cloud"},
	}

	for _, tt := range testFail {
		argv := []string{"--config", config.Name(), "remove", tt.remote}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(255),
		)
	}
}

// remoteUse tests the functionality of "apptainer remote use" command.
// 1. Tries to use non-existing remote entry
// 2. Adds remote entries and tries to use those
func (c ctx) remoteUse(t *testing.T) {
	config, err := os.CreateTemp(c.env.TestDir, "testConfig-")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(config.Name()) // clean up

	testFail := []struct {
		name   string
		remote string
	}{
		{"UseNonExistingRemote", "cloud"},
	}

	for _, tt := range testFail {
		argv := []string{"--config", config.Name(), "use", tt.remote}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(255),
		)
	}

	// Prep config by adding multiple remotes
	add := []struct {
		name   string
		remote string
		uri    string
	}{
		{"addCloud", "cloud", "cloud.sycloud.io"},
		{"addOther", "other", "cloud.sycloud.io"},
	}

	for _, tt := range add {
		argv := []string{"--config", config.Name(), "add", "--no-login", tt.remote, tt.uri}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testPass := []struct {
		name   string
		remote string
	}{
		{"UseFromNothingToRemote", "cloud"},
		{"UseFromRemoteToRemote", "other"},
	}

	for _, tt := range testPass {
		argv := []string{"--config", config.Name(), "use", tt.remote}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}
}

// remoteStatus tests the functionality of "apptainer remote status" command.
// 1. Adds remote endpoints
// 2. Verifies that remote status command succeeds on existing endpoints
// 3. Verifies that remote status command fails on non-existing endpoints
func (c ctx) remoteStatus(t *testing.T) {
	config, err := os.CreateTemp(c.env.TestDir, "testConfig-")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(config.Name()) // clean up

	// Prep config by adding multiple remotes
	add := []struct {
		name   string
		remote string
		uri    string
	}{
		{"addCloud", "cloud", "cloud.sycloud.io"},
		{"addInvalidRemote", "invalid", "notarealendpoint"},
	}

	for _, tt := range add {
		argv := []string{"--config", config.Name(), "add", "--no-login", tt.remote, tt.uri}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testPass := []struct {
		name   string
		remote string
	}{
		{"ValidRemote", "cloud"},
	}

	for _, tt := range testPass {
		argv := []string{"--config", config.Name(), "status", tt.remote}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testFail := []struct {
		name   string
		remote string
	}{
		{"NonExistingRemote", "notaremote"},
		{"NonExistingEndpoint", "invalid"},
	}

	for _, tt := range testFail {
		argv := []string{"--config", config.Name(), "status", tt.remote}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(255),
		)
	}
}

// remoteList tests the functionality of "apptainer remote list" command
func (c ctx) remoteList(t *testing.T) {
	config, err := os.CreateTemp(c.env.TestDir, "testConfig-")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(config.Name()) // clean up

	testPass := []struct {
		name string
	}{
		{"EmptyConfig"},
	}

	for _, tt := range testPass {
		argv := []string{"--config", config.Name(), "list"}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	// Prep config by adding multiple remotes
	add := []struct {
		name   string
		remote string
		uri    string
	}{
		{"addCloud", "cloud", "cloud.sycloud.io"},
		{"addRemote", "remote", "cloud.sycloud.io"},
	}

	for _, tt := range add {
		argv := []string{"--config", config.Name(), "add", "--no-login", tt.remote, tt.uri}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testPass = []struct {
		name string
	}{
		{"PopulatedConfig"},
	}

	for _, tt := range testPass {
		argv := []string{"--config", config.Name(), "list"}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	// Prep config by selecting a remote to default to
	use := []struct {
		name   string
		remote string
	}{
		{"useCloud", "cloud"},
	}

	for _, tt := range use {
		argv := []string{"--config", config.Name(), "use", tt.remote}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}

	testPass = []struct {
		name string
	}{
		{"PopulatedConfigWithDefault"},
	}

	for _, tt := range testPass {
		argv := []string{"--config", config.Name(), "list"}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(argv...),
			e2e.ExpectExit(0),
		)
	}
}

func (c ctx) remoteTestHelp(t *testing.T) {
	tests := []struct {
		name           string
		cmdArgs        []string
		expectedOutput string
	}{
		{
			name:           "add help",
			cmdArgs:        []string{"add", "--help"},
			expectedOutput: "Add a new apptainer remote endpoint",
		},
		{
			name:           "list help",
			cmdArgs:        []string{"list", "--help"},
			expectedOutput: "List all apptainer remote endpoints that are configured",
		},
		{
			name:           "login help",
			cmdArgs:        []string{"login", "--help"},
			expectedOutput: "Login to an apptainer remote endpoint",
		},
		{
			name:           "remove help",
			cmdArgs:        []string{"remove", "--help"},
			expectedOutput: "Remove an existing apptainer remote endpoint",
		},
		{
			name:           "status help",
			cmdArgs:        []string{"status", "--help"},
			expectedOutput: "Check the status of the apptainer services at an endpoint",
		},
		{
			name:           "use help",
			cmdArgs:        []string{"use", "--help"},
			expectedOutput: "Set an Apptainer remote endpoint to be actively used",
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("remote"),
			e2e.WithArgs(tt.cmdArgs...),
			e2e.ExpectExit(
				0,
				e2e.ExpectOutput(e2e.RegexMatch, `^`+tt.expectedOutput),
			),
		)
	}
}

func (c ctx) remoteUseExclusive(t *testing.T) {
	var (
		defaultRemote = "DefaultRemote"
		testRemote    = "e2e"
	)

	tests := []struct {
		name       string
		command    string
		args       []string
		expectExit int
		profile    e2e.Profile
	}{
		{
			name:       "use exclusive as user",
			command:    "remote use",
			args:       []string{"--exclusive", "--global", testRemote},
			expectExit: 255,
			profile:    e2e.UserProfile,
		},
		{
			name:       "add remote",
			command:    "remote add",
			args:       []string{"--global", testRemote, "cloud.test.com"},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote exclusive with global as root",
			command:    "remote use",
			args:       []string{"--exclusive", "--global", testRemote},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote DefaultCloud as user KO",
			command:    "remote use",
			args:       []string{defaultRemote},
			expectExit: 255,
			profile:    e2e.UserProfile,
		},
		{
			name:       "remove e2e remote",
			command:    "remote remove",
			args:       []string{"--global", testRemote},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote DefaultCloud as user OK",
			command:    "remote use",
			args:       []string{defaultRemote},
			expectExit: 0,
			profile:    e2e.UserProfile,
		},
		{
			name:       "add remote",
			command:    "remote add",
			args:       []string{"--global", testRemote, "cloud.test.com"},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote exclusive without global as root",
			command:    "remote use",
			args:       []string{"--exclusive", testRemote},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote DefaultCloud as exclusive",
			command:    "remote use",
			args:       []string{"--exclusive", defaultRemote},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote e2e as exclusive",
			command:    "remote use",
			args:       []string{"--exclusive", testRemote},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote DefaultCloud as user KO",
			command:    "remote use",
			args:       []string{defaultRemote},
			expectExit: 255,
			profile:    e2e.UserProfile,
		},
		{
			name:       "remove e2e remote",
			command:    "remote remove",
			args:       []string{"--global", testRemote},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
		{
			name:       "no default remote set",
			command:    "key search",
			args:       []string{"@"},
			expectExit: 255,
			profile:    e2e.RootProfile,
		},
		{
			name:       "use remote DefaultCloud global",
			command:    "remote use",
			args:       []string{"--global", defaultRemote},
			expectExit: 0,
			profile:    e2e.RootProfile,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit),
		)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"add":           c.remoteAdd,
		"list":          c.remoteList,
		"remove":        c.remoteRemove,
		"status":        c.remoteStatus,
		"test help":     c.remoteTestHelp,
		"use":           c.remoteUse,
		"use exclusive": np(c.remoteUseExclusive),
	}
}
