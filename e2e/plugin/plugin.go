// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	env e2e.TestEnv
}

func (c ctx) testPluginBasic(t *testing.T) {
	pluginName := "github.com/apptainer/apptainer/e2e-plugin"

	// plugin code directory
	pluginDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "plugin-dir-", "")
	defer cleanup(t)

	// plugin sif file
	sifFile := filepath.Join(pluginDir, "plugin.sif")

	tests := []struct {
		name       string
		profile    e2e.Profile
		command    string
		args       []string
		expectExit int
		expectOp   e2e.ApptainerCmdResultOp
	}{
		{
			name:       "Create",
			profile:    e2e.UserProfile,
			command:    "plugin create",
			args:       []string{pluginDir, pluginName},
			expectExit: 0,
		},
		{
			name:       "ListNoPlugins",
			profile:    e2e.UserProfile,
			command:    "plugin list",
			args:       []string{},
			expectExit: 0,
			expectOp:   e2e.ExpectOutput(e2e.ExactMatch, "There are no plugins installed."),
		},
		{
			name:       "Compile",
			profile:    e2e.UserProfile,
			command:    "plugin compile",
			args:       []string{"--out", sifFile, pluginDir},
			expectExit: 0,
		},
		{
			name:       "Install",
			profile:    e2e.RootProfile,
			command:    "plugin install",
			args:       []string{sifFile},
			expectExit: 0,
		},
		{
			name:       "InstallAsUser",
			profile:    e2e.UserProfile,
			command:    "plugin install",
			args:       []string{sifFile},
			expectExit: 255,
		},
		{
			name:       "ListAfterInstall",
			profile:    e2e.UserProfile,
			command:    "plugin list",
			args:       []string{},
			expectExit: 0,
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "yes  "+pluginName),
		},
		{
			name:       "Disable",
			profile:    e2e.RootProfile,
			command:    "plugin disable",
			args:       []string{pluginName},
			expectExit: 0,
		},
		{
			name:       "ListAfterDisable",
			profile:    e2e.UserProfile,
			command:    "plugin list",
			args:       []string{},
			expectExit: 0,
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "no  "+pluginName),
		},
		{
			name:       "DisableAsUser",
			profile:    e2e.UserProfile,
			command:    "plugin disable",
			args:       []string{pluginName},
			expectExit: 255,
		},
		{
			name:       "Enable",
			profile:    e2e.RootProfile,
			command:    "plugin enable",
			args:       []string{pluginName},
			expectExit: 0,
		},
		{
			name:       "ListAfterEnable",
			profile:    e2e.UserProfile,
			command:    "plugin list",
			args:       []string{},
			expectExit: 0,
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "yes  "+pluginName),
		},
		{
			name:       "EnableAsUser",
			profile:    e2e.UserProfile,
			command:    "plugin enable",
			args:       []string{pluginName},
			expectExit: 255,
		},
		{
			name:       "InspectFromName",
			profile:    e2e.UserProfile,
			command:    "plugin inspect",
			args:       []string{pluginName},
			expectExit: 0,
		},
		{
			name:       "InspectFromSIF",
			profile:    e2e.UserProfile,
			command:    "plugin inspect",
			args:       []string{sifFile},
			expectExit: 0,
			expectOp:   e2e.ExpectOutput(e2e.ContainMatch, "Name: "+pluginName),
		},
		{
			name:       "UninstallAsUser",
			profile:    e2e.UserProfile,
			command:    "plugin uninstall",
			args:       []string{pluginName},
			expectExit: 255,
		},
		{
			name:       "Uninstall",
			profile:    e2e.RootProfile,
			command:    "plugin uninstall",
			args:       []string{pluginName},
			expectExit: 0,
		},
		{
			name:       "ListAfterUninstall",
			profile:    e2e.UserProfile,
			command:    "plugin list",
			args:       []string{},
			expectExit: 0,
			expectOp:   e2e.ExpectOutput(e2e.ExactMatch, "There are no plugins installed."),
		},
	}

	for _, tt := range tests {
		var stderr, stdout string
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit, tt.expectOp, e2e.GetStreams(&stdout, &stderr)),
		)
		t.Logf("stdout:\n%s\n\nstderr:\n%s\n\n", stdout, stderr)
	}
}

func (c ctx) testCLICallbacks(t *testing.T) {
	pluginDir := "./plugin/testdata/cli"
	pluginName := "github.com/apptainer/apptainer/e2e-cli-plugin"

	// plugin sif file
	sifFile := filepath.Join(c.env.TestDir, "plugin.sif")
	defer os.Remove(sifFile)

	tests := []struct {
		name       string
		profile    e2e.Profile
		command    string
		args       []string
		expectExit int
	}{
		{
			name:       "Compile",
			profile:    e2e.UserProfile,
			command:    "plugin compile",
			args:       []string{"--out", sifFile, pluginDir},
			expectExit: 0,
		},
		{
			name:       "Install",
			profile:    e2e.RootProfile,
			command:    "plugin install",
			args:       []string{sifFile},
			expectExit: 0,
		},
		{
			name:       "CLICallback",
			profile:    e2e.UserProfile,
			command:    "exit",
			args:       []string{"42"},
			expectExit: 42,
		},
		{
			name:       "ApptainerConfigCallback",
			profile:    e2e.UserProfile,
			command:    "shell",
			args:       []string{c.env.TestDir},
			expectExit: 43,
		},
		{
			name:       "Uninstall",
			profile:    e2e.RootProfile,
			command:    "plugin uninstall",
			args:       []string{pluginName},
			expectExit: 0,
		},
	}

	for _, tt := range tests {
		var stderr, stdout string
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithGlobalOptions("--debug"),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit, e2e.GetStreams(&stdout, &stderr)),
		)
		t.Logf("stdout:\n%s\n\nstderr:\n%s\n\n", stdout, stderr)
	}
}

func (c ctx) testApptainerCallbacks(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	pluginDir := "./plugin/testdata/runtime_apptainer"
	pluginName := "github.com/apptainer/apptainer/e2e-runtime-plugin"

	// plugin sif file
	sifFile := filepath.Join(c.env.TestDir, "plugin.sif")
	defer os.Remove(sifFile)

	tests := []struct {
		name       string
		profile    e2e.Profile
		command    string
		args       []string
		expectExit int
	}{
		{
			name:       "Compile",
			profile:    e2e.UserProfile,
			command:    "plugin compile",
			args:       []string{"--out", sifFile, pluginDir},
			expectExit: 0,
		},
		{
			name:       "Install",
			profile:    e2e.RootProfile,
			command:    "plugin install",
			args:       []string{sifFile},
			expectExit: 0,
		},
		{
			name:       "MonitorCallback",
			profile:    e2e.UserProfile,
			command:    "exec",
			args:       []string{c.env.ImagePath, "true"},
			expectExit: 42,
		},
		{
			name:       "PostStartProcessCallback",
			profile:    e2e.UserProfile,
			command:    "exec",
			args:       []string{"--contain", c.env.ImagePath, "true"},
			expectExit: 43,
		},
		{
			name:       "Uninstall",
			profile:    e2e.RootProfile,
			command:    "plugin uninstall",
			args:       []string{pluginName},
			expectExit: 0,
		},
	}

	for _, tt := range tests {
		var stderr, stdout string
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithGlobalOptions("--debug"),
			e2e.WithProfile(tt.profile),
			e2e.WithCommand(tt.command),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit, e2e.GetStreams(&stdout, &stderr)),
		)
		t.Logf("stdout:\n%s\n\nstderr:\n%s\n\n", stdout, stderr)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"basic":               np(c.testPluginBasic),
		"CLI_callbacks":       np(c.testCLICallbacks),
		"Apptainer_callbacks": np(c.testApptainerCallbacks),
	}
}
