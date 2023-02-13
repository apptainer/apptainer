// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package delete

import (
	"bytes"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
)

type ctx struct {
	env e2e.TestEnv
}

func (c ctx) testDeleteCmd(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		agree      string
		expectExit int
		expect     e2e.ApptainerCmdResultOp
		disabled   bool
	}{
		{
			name:       "delete unauthorized arch",
			args:       []string{"--arch=amd64", "oras://ghcr.io/apptainer/test:v0.0.3"},
			agree:      "y",
			expectExit: 255,
		},
		{
			name:       "delete unauthorized no arch",
			args:       []string{"oras://ghcr.io/apptainer/test:v0.0.3"},
			agree:      "y",
			expectExit: 255,
		},
		{
			name:       "delete disagree arch",
			args:       []string{"--arch=amd64", "oras://ghcr.io/apptainer/test:v0.0.3"},
			agree:      "n",
			expectExit: 0,
			disabled:   true,
		},
		{
			name:       "delete disagree noarch",
			args:       []string{"oras://ghcr.io/apptainer/test:v0.0.3"},
			agree:      "n",
			expectExit: 0,
			disabled:   true,
		},
		{
			name:       "delete unauthorized force arch",
			args:       []string{"--force", "--arch=amd64", "oras://ghcr.io/apptainer/test:v0.0.3"},
			agree:      "",
			expectExit: 255,
		},
		{
			name:       "delete unauthorized force noarch",
			args:       []string{"--force", "oras://ghcr.io/apptainer/test:v0.0.3"},
			agree:      "",
			expectExit: 255,
		},
		{
			name:       "delete unauthorized custom library",
			args:       []string{"--library=https://ghcr.io", "oras://ghcr.io/apptainer/test:v0.0.3"},
			agree:      "y",
			expectExit: 255,
		},
		{
			name:       "delete host in uri",
			args:       []string{"oras://oras.example.com/test/default/test:v0.0.3"},
			agree:      "y",
			expectExit: 255,
			expect:     e2e.ExpectError(e2e.ContainMatch, "no such host"),
			disabled:   true,
		},
	}

	for _, tt := range tests {
		if tt.disabled {
			continue
		}
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("delete"),
			e2e.WithArgs(tt.args...),
			e2e.WithStdin(bytes.NewBufferString(tt.agree)),
			e2e.ExpectExit(tt.expectExit, tt.expect),
		)
	}
}

// E2ETests is the main func to trigger the test suite.
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := ctx{
		env: env,
	}

	return testhelper.Tests{
		"delete": c.testDeleteCmd,
	}
}
