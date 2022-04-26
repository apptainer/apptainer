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

// moved from INSTANCE suite, as testing with systemd cgroup manager requires
// e2e to be run without PID namespace
func (c *ctx) instanceApply(t *testing.T, profile e2e.Profile) {
	e2e.EnsureImage(t, c.env)
	// pick up a random name
	instanceName := randomName(t)
	joinName := fmt.Sprintf("instance://%s", instanceName)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(profile),
		e2e.WithRootlessEnv(),
		e2e.WithCommand("instance start"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, instanceName),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(profile),
		e2e.WithRootlessEnv(),
		e2e.WithCommand("exec"),
		e2e.WithArgs(joinName, "cat", "/dev/null"),
		e2e.ExpectExit(1, e2e.ExpectError(e2e.ContainMatch, "Operation not permitted")),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(profile),
		e2e.WithRootlessEnv(),
		e2e.WithCommand("instance stop"),
		e2e.WithArgs(instanceName),
		e2e.ExpectExit(0),
	)
}

func (c *ctx) instanceApplyRoot(t *testing.T) {
	c.instanceApply(t, e2e.RootProfile)
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

	// Applies a memory limit so small that it should result in us being killed OOM (137)
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("memory"),
		e2e.WithProfile(profile),
		e2e.WithRootlessEnv(),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/memory_limit.toml", c.env.ImagePath, "/bin/sleep", "5"),
		e2e.ExpectExit(137),
	)

	// Rootfull cgroups should be able to limit access to devices
	if profile.Privileged() {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest("device"),
			e2e.WithProfile(profile),
			e2e.WithCommand("exec"),
			e2e.WithArgs("--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"),
			e2e.ExpectExit(1,
				e2e.ExpectError(e2e.ContainMatch, "Operation not permitted")),
		)
		return
	}

	// Cgroups v2 device limits are via ebpf and rootless cannot apply them.
	// Check that attempting to apply a device limit warns that it won't take effect.
	c.env.RunApptainer(
		t,
		e2e.AsSubtest("device"),
		e2e.WithProfile(profile),
		e2e.WithRootlessEnv(),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, "cat", "/dev/null"),
		e2e.ExpectExit(0,
			e2e.ExpectError(e2e.ContainMatch, "Device limits will not be applied with rootless cgroups")),
	)
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
		"instance root cgroups":     np(env.WithRootManagers(c.instanceApplyRoot)),
		"instance rootless cgroups": np(env.WithRootlessManagers(c.instanceApplyRootless)),
		"action root cgroups":       np(env.WithRootManagers(c.actionApplyRoot)),
		"action rootless cgroups":   np(env.WithRootlessManagers(c.actionApplyRootless)),
	}
}
