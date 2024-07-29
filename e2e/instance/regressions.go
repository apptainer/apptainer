// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package instance

import (
	"fmt"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
)

func (c *ctx) issue5033(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	c.profile = e2e.RootProfile

	// pick up a random name
	instanceName := randomName(t)
	joinName := fmt.Sprintf("instance://%s", instanceName)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(c.profile),
		e2e.WithCommand("instance start"),
		e2e.WithArgs("--boot", c.env.ImagePath, instanceName),
		e2e.ExpectExit(0),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(c.profile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(joinName, "/bin/true"),
		e2e.ExpectExit(0),
	)

	c.stopInstance(t, instanceName)
}

func (c *ctx) issue2189(t *testing.T) {
	e2e.EnsureDebianImage(t, c.env)

	c.profile = e2e.FakerootProfile

	instanceName := randomName(t)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(c.profile),
		e2e.WithCommand("instance start"),
		e2e.WithArgs("--ignore-subuid", c.env.ImagePath, instanceName),
		e2e.ExpectExit(0),
	)
	c.stopInstance(t, instanceName)
}
