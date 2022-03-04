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
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
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
func (c *ctx) instanceApplyCgroups(t *testing.T) {
	require.Cgroups(t)
	e2e.EnsureImage(t, c.env)

	// pick up a random name
	instanceName := randomName(t)
	joinName := fmt.Sprintf("instance://%s", instanceName)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("instance start"),
		e2e.WithArgs("--apply-cgroups", "testdata/cgroups/deny_device.toml", c.env.ImagePath, instanceName),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(joinName, "cat", "/dev/null"),
		e2e.ExpectExit(1),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.RootProfile),
		e2e.WithCommand("instance stop"),
		e2e.WithArgs(instanceName),
		e2e.ExpectExit(0),
	)
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := &ctx{
		env: env,
	}

	np := testhelper.NoParallel

	return testhelper.Tests{
		"instance apply cgroups": np(env.WithCgroupManagers(c.instanceApplyCgroups)),
	}
}
