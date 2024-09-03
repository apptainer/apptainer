// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package registry

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	env e2e.TestEnv
}

// registryList tests the functionality of "apptainer registry list" command
func (c ctx) registryList(t *testing.T) {
	registry := fmt.Sprintf("oras://%s", c.env.TestRegistry)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("registry login"),
		e2e.WithArgs("-u", e2e.DefaultUsername, "-p", e2e.DefaultPassword, registry),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("registry list"),
		e2e.ExpectExit(0,
			e2e.ExpectOutput(
				e2e.ContainMatch,
				strings.Join([]string{
					"URI                     SECURE?",
					registry + "  ✓",
				}, "\n"))),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("registry logout"),
		e2e.WithArgs(registry),
		e2e.ExpectExit(0),
	)
}

func (c ctx) registryTestHelp(t *testing.T) {
	tests := []struct {
		name           string
		cmdArgs        []string
		expectedOutput string
	}{
		{
			name:           "list help",
			cmdArgs:        []string{"list", "--help"},
			expectedOutput: "List all OCI credentials that are configured",
		},
		{
			name:           "login help",
			cmdArgs:        []string{"login", "--help"},
			expectedOutput: "Login to an OCI/Docker registry",
		},
		{
			name:           "logout help",
			cmdArgs:        []string{"logout", "--help"},
			expectedOutput: "Logout from an OCI/Docker registry",
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("registry"),
			e2e.WithArgs(tt.cmdArgs...),
			e2e.ExpectExit(
				0,
				e2e.ExpectOutput(e2e.RegexMatch, `^`+tt.expectedOutput),
			),
		)
	}
}

func (c ctx) registryLogin(t *testing.T) {
	var (
		registry    = fmt.Sprintf("oras://%s", c.env.TestRegistry)
		badRegistry = "oras://bad_registry:5000"
	)

	tests := []struct {
		name       string
		command    string
		args       []string
		stdin      io.Reader
		expectExit int
	}{
		{
			name:       "login username and empty password",
			command:    "registry login",
			args:       []string{"-u", e2e.DefaultUsername, "-p", "", registry},
			expectExit: 255,
		},
		{
			name:       "login empty username and empty password",
			command:    "registry login",
			args:       []string{"-p", "", registry},
			expectExit: 255,
		},
		{
			name:       "login empty username and password",
			command:    "registry login",
			args:       []string{"-p", "bad", registry},
			expectExit: 255,
		},
		{
			name:       "login without scheme KO",
			command:    "registry login",
			args:       []string{"-u", e2e.DefaultUsername, "-p", e2e.DefaultPassword, c.env.TestRegistry},
			expectExit: 255,
		},
		{
			name:       "login OK",
			command:    "registry login",
			args:       []string{"-u", e2e.DefaultUsername, "-p", e2e.DefaultPassword, registry},
			expectExit: 0,
		},
		{
			name:       "login password-stdin",
			command:    "registry login",
			args:       []string{"-u", e2e.DefaultUsername, "--password-stdin", registry},
			stdin:      strings.NewReader(e2e.DefaultPassword),
			expectExit: 0,
		},
		{
			name:       "logout KO",
			command:    "registry logout",
			args:       []string{badRegistry},
			expectExit: 0, // only warn message will be printed, no more error returned.
		},
		{
			name:       "logout OK",
			command:    "registry logout",
			args:       []string{registry},
			expectExit: 0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithStdin(tt.stdin),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit),
		)
	}
}

func (c ctx) registryLoginPushPrivate(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	var (
		registry = fmt.Sprintf("oras://%s", c.env.TestRegistry)
		repo     = fmt.Sprintf("oras://%s/private/e2e:1.0.0", c.env.TestRegistry)
	)

	tests := []struct {
		name       string
		command    string
		args       []string
		expectExit int
	}{
		{
			name:       "push before login",
			command:    "push",
			args:       []string{c.env.ImagePath, repo},
			expectExit: 255,
		},
		{
			name:       "login",
			command:    "registry login",
			args:       []string{"-u", e2e.DefaultUsername, "-p", e2e.DefaultPassword, registry},
			expectExit: 0,
		},
		{
			name:       "push after login",
			command:    "push",
			args:       []string{c.env.ImagePath, repo},
			expectExit: 0,
		},
		{
			name:       "logout",
			command:    "registry logout",
			args:       []string{registry},
			expectExit: 0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit),
		)
	}
}

// Repeated logins with same URI should not create duplicate remote.yaml entries.
// If we login twice, and logout once we should not see the URI in list.
// See https://github.com/sylabs/singularity/issues/214
func (c ctx) registryLoginRepeated(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	registry := fmt.Sprintf("oras://%s", c.env.TestRegistry)

	tests := []struct {
		name       string
		command    string
		args       []string
		expectExit int
		resultOp   e2e.ApptainerCmdResultOp
	}{
		{
			name:       "FirstLogin",
			command:    "registry login",
			args:       []string{"-u", e2e.DefaultUsername, "-p", e2e.DefaultPassword, registry},
			expectExit: 0,
		},
		{
			name:       "SecondLogin",
			command:    "registry login",
			args:       []string{"-u", e2e.DefaultUsername, "-p", e2e.DefaultPassword, registry},
			expectExit: 0,
		},
		{
			name:       "logout",
			command:    "registry logout",
			args:       []string{registry},
			expectExit: 0,
		},
		{
			name:       "list",
			command:    "registry list",
			expectExit: 0,
			resultOp:   e2e.ExpectOutput(e2e.UnwantedContainMatch, registry),
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit, tt.resultOp),
		)
	}
}

// Tests authentication with registries, incl.
// https://github.com/sylabs/singularity/issues/2226, among many other flows.
func (c ctx) registryAuth(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		c.registryAuthTester(t, false)
	})
	t.Run("custom", func(t *testing.T) {
		c.registryAuthTester(t, true)
	})
}

func (c ctx) registryAuthTester(t *testing.T, withCustomAuthFile bool) {
	tmpdir, tmpdirCleanup := e2e.MakeTempDir(t, c.env.TestDir, "registry-auth", "")
	t.Cleanup(func() {
		if !t.Failed() {
			tmpdirCleanup(t)
		}
	})

	prevCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get current working directory: %s", err)
	}
	defer os.Chdir(prevCwd)
	if err = os.Chdir(tmpdir); err != nil {
		t.Fatalf("could not change cwd to %q: %s", tmpdir, err)
	}

	localAuthFileName := ""
	if withCustomAuthFile {
		localAuthFileName = "./my_local_authfile"
	}

	authFileArgs := []string{}
	if withCustomAuthFile {
		authFileArgs = []string{"--authfile", localAuthFileName}
	}

	t.Cleanup(func() {
		e2e.PrivateRepoLogout(t, c.env, e2e.UserProfile, localAuthFileName)
	})

	orasCustomPushTarget := fmt.Sprintf("oras://%s/authfile-pushtest-oras-busybox:latest", c.env.TestRegistryPrivPath)

	tests := []struct {
		name          string
		cmd           string
		args          []string
		whileLoggedIn bool
		expectExit    int
	}{
		{
			name:          "docker pull before auth",
			cmd:           "pull",
			args:          []string{"-F", "--disable-cache", "--no-https", c.env.TestRegistryPrivImage},
			whileLoggedIn: false,
			expectExit:    255,
		},
		{
			name:          "docker pull",
			cmd:           "pull",
			args:          []string{"-F", "--disable-cache", "--no-https", c.env.TestRegistryPrivImage},
			whileLoggedIn: true,
			expectExit:    0,
		},
		{
			name:          "noauth docker pull",
			cmd:           "pull",
			args:          []string{"-F", "--disable-cache", "--no-https", c.env.TestRegistryPrivImage},
			whileLoggedIn: false,
			expectExit:    255,
		},
		{
			name:          "noauth oras push",
			cmd:           "push",
			args:          []string{"my-busybox_latest.sif", orasCustomPushTarget},
			whileLoggedIn: false,
			expectExit:    255,
		},
		{
			name:          "oras push",
			cmd:           "push",
			args:          []string{"my-busybox_latest.sif", orasCustomPushTarget},
			whileLoggedIn: true,
			expectExit:    0,
		},
		{
			name:          "noauth oras pull",
			cmd:           "pull",
			args:          []string{"-F", "--disable-cache", "--no-https", orasCustomPushTarget},
			whileLoggedIn: false,
			expectExit:    255,
		},
		{
			name:          "oras pull",
			cmd:           "pull",
			args:          []string{"-F", "--disable-cache", "--no-https", orasCustomPushTarget},
			whileLoggedIn: true,
			expectExit:    0,
		},
	}

	for _, tt := range tests {
		if tt.whileLoggedIn {
			e2e.PrivateRepoLogin(t, c.env, e2e.UserProfile, localAuthFileName)
		} else {
			e2e.PrivateRepoLogout(t, c.env, e2e.UserProfile, localAuthFileName)
		}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand(tt.cmd),
			e2e.WithArgs(append(authFileArgs, tt.args...)...),
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
		"help":               c.registryTestHelp,
		"login basic":        np(c.registryLogin),
		"login push private": np(c.registryLoginPushPrivate),
		"login repeated":     np(c.registryLoginRepeated),
		"list":               np(c.registryList),
		"auth":               np(c.registryAuth),
	}
}
