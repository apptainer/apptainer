// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package tag

import (
	"fmt"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	env e2e.TestEnv
}

func (c *ctx) setup(t *testing.T) {
	e2e.EnsureORASImage(t, c.env)
}

func (c ctx) testTagCmd(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		expectExit int
	}{
		{
			name:       "tag existing image",
			args:       []string{c.env.OrasTestImage, "v1"},
			expectExit: 0,
		},
		{
			name:       "tag non-existing image",
			args:       []string{fmt.Sprintf("oras://%s/oras_test_sif:foo", c.env.TestRegistry), "v2"},
			expectExit: 255,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("tag"),
			e2e.WithArgs(tt.args...),
			e2e.ExpectExit(tt.expectExit),
		)
	}
}

// E2ETests is the main func to trigger the test suite.
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"ordered": func(t *testing.T) {
			// Setup a test registry to tag in (for oras).
			c.setup(t)
			t.Run("tag", c.testTagCmd)
		},
	}
}
