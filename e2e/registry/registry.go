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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/pkg/syfs"
	useragent "github.com/apptainer/apptainer/pkg/util/user-agent"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
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
			expectExit: 255,
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

// JSON files created by our `remote login` flow should be usable in execution
// flows that use containers/image APIs.
// See https://github.com/sylabs/singularity/issues/2226
func (c ctx) registryIssue2226(t *testing.T) {
	testRegistry := c.env.TestRegistry
	testRegistryURI := fmt.Sprintf("docker://%s", testRegistry)
	privRepo := fmt.Sprintf("%s/private/e2eprivrepo", testRegistry)
	privRepoURI := fmt.Sprintf("docker://%s", privRepo)

	tmpdir, tmpdirCleanup := e2e.MakeTempDir(t, "", "issue2226", "")
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

	areWeLoggedIn := false

	privRepoLogin := func() {
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("registry login"),
			e2e.WithArgs("-u", e2e.DefaultUsername, "-p", e2e.DefaultPassword, testRegistryURI),
			e2e.ExpectExit(0),
		)
		areWeLoggedIn = true
	}
	privRepoLogout := func() {
		c.env.RunApptainer(
			t,
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("registry logout"),
			e2e.WithArgs(testRegistryURI),
			e2e.ExpectExit(0),
		)
		areWeLoggedIn = false
	}

	privRepoLogin()
	t.Cleanup(func() {
		if areWeLoggedIn {
			privRepoLogout()
		}
	})

	policy := &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		t.Fatalf("failed to create new policy context: %v", err)
	}

	sourceCtx := &types.SystemContext{
		OCIInsecureSkipTLSVerify:    false,
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(false),
		DockerRegistryUserAgent:     useragent.Value(),
	}
	destCtx := &types.SystemContext{
		OCIInsecureSkipTLSVerify:    true,
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(true),
		DockerRegistryUserAgent:     useragent.Value(),
	}

	u := e2e.CurrentUser(t)
	configPath := filepath.Join(u.Dir, ".singularity", syfs.DockerConfFile)
	sourceCtx.AuthFilePath = configPath
	destCtx.AuthFilePath = configPath

	source := "docker://alpine:latest"
	dest := fmt.Sprintf("%s/my-alpine:latest", privRepoURI)
	sourceRef, err := docker.ParseReference(strings.TrimPrefix(source, "docker:"))
	if err != nil {
		t.Fatalf("failed to parse %s reference: %s", source, err)
	}
	destRef, err := docker.ParseReference(strings.TrimPrefix(dest, "docker:"))
	if err != nil {
		t.Fatalf("failed to parse %s reference: %s", dest, err)
	}

	_, err = copy.Image(context.Background(), policyCtx, destRef, sourceRef, &copy.Options{
		ReportWriter:   io.Discard,
		SourceCtx:      sourceCtx,
		DestinationCtx: destCtx,
	})
	if err != nil {
		var e docker.ErrUnauthorizedForCredentials
		if errors.As(err, &e) {
			t.Fatalf("Authentication info written by 'registry login' did not work when trying to copy OCI image to private repo (%v)", e)
		}
		t.Fatalf("Failed to copy %s to %s: %s", source, dest, err)
	}

	privRepoLogout()

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("noauth"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull --no-https"),
		e2e.WithArgs(dest),
		e2e.ExpectExit(255),
	)

	privRepoLogin()

	c.env.RunApptainer(
		t,
		e2e.AsSubtest("auth"),
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("pull --no-https"),
		e2e.WithArgs(dest),
		e2e.ExpectExit(0),
	)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"test registry help":          c.registryTestHelp,
		"registry login basic":        np(c.registryLogin),
		"registry login push private": np(c.registryLoginPushPrivate),
		"registry login repeated":     np(c.registryLoginRepeated),
		"registry list":               np(c.registryList),
		"registry issue 2226":         np(c.registryIssue2226),
	}
}
