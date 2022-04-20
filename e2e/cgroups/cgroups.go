// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cgroups

import (
	"fmt"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/google/uuid"
)

//  NOTE
//  ----
//  Tests in this package/topic are run in a a mount namespace only. There is
//  no PID namespace, in order that the systemd cgroups manager functionality
//  can be exercised.
//
//  You must take extra care not to leave detached process etc. that will
//  pollute the host PID namespace.
//

// randomName generates a random name instance or OCI container name based on a UUID.
func randomName(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

type ctx struct {
	env e2e.TestEnv
}

// instanceStats tests an instance ability to output stats
func (c *ctx) instanceStats(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	// All tests require root
	tests := []struct {
		name           string
		createArgs     []string
		startErrorCode int
		statsErrorCode int
	}{
		{
			name:           "basic stats create",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/cpu_success.toml", c.env.ImagePath},
			statsErrorCode: 0,
			startErrorCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// stats only for privileged atm
			if !profile.Privileged() {
				t.Skip()
			}

			// We always expect stats output, not create
			createExitFunc := []e2e.ApptainerCmdResultOp{}
			instanceName := randomName(t)

			// Start the instance with cgroups for stats
			createArgs := append(tt.createArgs, instanceName)
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("start"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance start"),
				e2e.WithArgs(createArgs...),
				e2e.ExpectExit(tt.startErrorCode, createExitFunc...),
			)

			// Get stats for the instance
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("stats"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance stats"),
				e2e.WithArgs(instanceName),
				e2e.ExpectExit(tt.statsErrorCode,
					e2e.ExpectOutput(e2e.ContainMatch, instanceName),
					e2e.ExpectOutput(e2e.ContainMatch, "INSTANCE NAME"),
					e2e.ExpectOutput(e2e.ContainMatch, "CPU USAGE"),
					e2e.ExpectOutput(e2e.ContainMatch, "MEM USAGE / LIMIT"),
					e2e.ExpectOutput(e2e.ContainMatch, "MEM %"),
					e2e.ExpectOutput(e2e.ContainMatch, "BLOCK I/O"),
					e2e.ExpectOutput(e2e.ContainMatch, "PIDS"),
				),
			)
			c.env.RunApptainer(
				t,
				e2e.AsSubtest("stop"),
				e2e.WithProfile(profile),
				e2e.WithCommand("instance stop"),
				e2e.WithArgs(instanceName),
				e2e.ExpectExit(0),
			)
		})
	}
}

// moved from INSTANCE suite, as testing with systemd cgroup manager requires
// e2e to be run without PID namespace
func (c *ctx) instanceApply(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	tests := []struct {
		name           string
		createArgs     []string
		execArgs       []string
		startErrorCode int
		startErrorOut  string
		execErrorCode  int
		execErrorOut   string
		rootfull       bool
		rootless       bool
	}{
		{
			name:           "nonexistent toml",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/doesnotexist.toml", c.env.ImagePath},
			startErrorCode: 255,
			// e2e test currently only captures the error from the CLI process, not the error displayed by the
			// starter process, so we check for the generic CLI error.
			startErrorOut: "no such file or directory",
			rootfull:      true,
			rootless:      true,
		},
		{
			name:           "invalid toml",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/invalid.toml", c.env.ImagePath},
			startErrorCode: 255,
			// e2e test currently only captures the error from the CLI process, not the error displayed by the
			// starter process, so we check for the generic CLI error.
			startErrorOut: "parsing error",
			rootfull:      true,
			rootless:      true,
		},
		{
			name:       "memory limit",
			createArgs: []string{"--apply-cgroups", "testdata/cgroups/memory_limit.toml", c.env.ImagePath},
			// We get a CLI 255 error code, not the 137 that the starter receives for an OOM kill
			startErrorCode: 255,
			rootfull:       true,
			rootless:       true,
		},
		{
			name:           "cpu success",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/cpu_success.toml", c.env.ImagePath},
			startErrorCode: 0,
			execArgs:       []string{"/bin/true"},
			execErrorCode:  0,
			rootfull:       true,
			rootless:       true,
		},
		{
			name:           "device deny",
			createArgs:     []string{"--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath},
			startErrorCode: 0,
			execArgs:       []string{"cat", "/dev/null"},
			execErrorCode:  1,
			execErrorOut:   "Operation not permitted",
			rootfull:       true,
			rootless:       false,
		},
	}

	for _, tt := range tests {
		if profile.Privileged() && !tt.rootfull {
			t.Skip()
		}
		if !profile.Privileged() && !tt.rootless {
			t.Skip()
		}

		createExitFunc := []e2e.ApptainerCmdResultOp{}
		if tt.startErrorOut != "" {
			createExitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectError(e2e.ContainMatch, tt.startErrorOut)}
		}
		execExitFunc := []e2e.ApptainerCmdResultOp{}
		if tt.execErrorOut != "" {
			execExitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectError(e2e.ContainMatch, tt.execErrorOut)}
		}
		// pick up a random name
		instanceName := randomName(t)
		joinName := fmt.Sprintf("instance://%s", instanceName)

		createArgs := append(tt.createArgs, instanceName)
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name+"/start"),
			e2e.WithProfile(profile),
			e2e.WithCommand("instance start"),
			e2e.WithArgs(createArgs...),
			e2e.ExpectExit(tt.startErrorCode, createExitFunc...),
		)
		if tt.startErrorCode != 0 {
			continue
		}

		execArgs := append([]string{joinName}, tt.execArgs...)
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name+"/exec"),
			e2e.WithProfile(profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(execArgs...),
			e2e.ExpectExit(tt.execErrorCode, execExitFunc...),
		)

		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name+"/stop"),
			e2e.WithProfile(profile),
			e2e.WithCommand("instance stop"),
			e2e.WithArgs(instanceName),
			e2e.ExpectExit(0),
		)
	}
}

func (c *ctx) instanceApplyRoot(t *testing.T) {
	c.instanceApply(t, e2e.RootProfile)
}

func (c *ctx) instanceStatsRoot(t *testing.T) {
	c.instanceStats(t, e2e.RootProfile)
}

// TODO - when instance support for rootless cgroups is ready, this
// should instead call instanceApply over the user profiles.
func (c *ctx) instanceApplyRootless(t *testing.T) {
	e2e.EnsureImage(t, c.env)
	// pick up a random name
	instanceName := randomName(t)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithRootlessEnv(),
		e2e.WithCommand("instance start"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/memory_limit.toml", c.env.ImagePath, instanceName),
		e2e.ExpectExit(255,
			e2e.ExpectError(e2e.ContainMatch, "Instances do not currently support rootless cgroups")),
	)
}

func (c *ctx) actionApply(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)

	tests := []struct {
		name            string
		args            []string
		expectErrorCode int
		expectErrorOut  string
		rootfull        bool
		rootless        bool
	}{
		{
			name:            "nonexistent toml",
			args:            []string{"--apply-cgroups", "testdata/cgroups/doesnotexist.toml", c.env.ImagePath, "/bin/sleep", "5"},
			expectErrorCode: 255,
			expectErrorOut:  "no such file or directory",
			rootfull:        true,
			rootless:        true,
		},
		{
			name:            "invalid toml",
			args:            []string{"--apply-cgroups", "testdata/cgroups/invalid.toml", c.env.ImagePath, "/bin/sleep", "5"},
			expectErrorCode: 255,
			expectErrorOut:  "parsing error",
			rootfull:        true,
			rootless:        true,
		},
		{
			name:            "memory limit",
			args:            []string{"--apply-cgroups", "testdata/cgroups/memory_limit.toml", c.env.ImagePath, "/bin/sleep", "5"},
			expectErrorCode: 137,
			rootfull:        true,
			rootless:        true,
		},
		{
			name:            "cpu success",
			args:            []string{"--apply-cgroups", "testdata/cgroups/cpu_success.toml", c.env.ImagePath, "/bin/true"},
			expectErrorCode: 0,
			rootfull:        true,
			// This currently fails in the e2e scenario due to the way we are using a mount namespace.
			// It *does* work if you test it, directly calling the apptainer CLI.
			// Reason is believed to be: https://github.com/opencontainers/runc/issues/3026
			rootless: false,
		},
		// Device access is allowed by default.
		{
			name:            "device allow default",
			args:            []string{"--apply-cgroups", "testdata/cgroups/null.toml", c.env.ImagePath, "cat", "/dev/null"},
			expectErrorCode: 0,
			rootfull:        true,
			rootless:        true,
		},
		// Device limits are properly applied only in rootful mode. Rootless will ignore them with a warning.
		{
			name:            "device deny",
			args:            []string{"--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"},
			expectErrorCode: 1,
			expectErrorOut:  "Operation not permitted",
			rootfull:        true,
			rootless:        false,
		},
		{
			name:            "device ignored",
			args:            []string{"--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"},
			expectErrorCode: 0,
			expectErrorOut:  "Operation not permitted",
			rootfull:        false,
			rootless:        true,
		},
	}

	for _, tt := range tests {
		if profile.Privileged() && !tt.rootfull {
			t.Skip()
		}
		if !profile.Privileged() && !tt.rootless {
			t.Skip()
		}
		exitFunc := []e2e.ApptainerCmdResultOp{}
		if tt.expectErrorOut != "" {
			exitFunc = []e2e.ApptainerCmdResultOp{e2e.ExpectError(e2e.ContainMatch, tt.expectErrorOut)}
		}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectErrorCode, exitFunc...),
		)
	}
}

func (c *ctx) actionApplyRoot(t *testing.T) {
	c.actionApply(t, e2e.RootProfile)
}

func (c *ctx) actionApplyRootless(t *testing.T) {
	for _, profile := range []e2e.Profile{e2e.UserProfile, e2e.UserNamespaceProfile, e2e.FakerootProfile} {
		t.Run(profile.String(), func(t *testing.T) {
			c.actionApply(t, profile)
		})
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := &ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"instance stats":            np(env.WithRootManagers(c.instanceStatsRoot)),
		"instance root cgroups":     np(env.WithRootManagers(c.instanceApplyRoot)),
		"instance rootless cgroups": np(env.WithRootlessManagers(c.instanceApplyRootless)),
		"action root cgroups":       np(env.WithRootManagers(c.actionApplyRoot)),
		"action rootless cgroups":   np(env.WithRootlessManagers(c.actionApplyRootless)),
	}
}
