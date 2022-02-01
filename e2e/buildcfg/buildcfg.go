// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package buildcfg

import (
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
)

type buildcfgTests struct {
	env e2e.TestEnv
}

func (c buildcfgTests) buildcfgBasic(t *testing.T) {
	tests := []struct {
		name    string
		cmdArgs []string
		exit    int
		op      e2e.ApptainerCmdResultOp
	}{
		{
			name:    "help",
			cmdArgs: []string{"--help"},
			exit:    0,
			op: e2e.ExpectOutput(
				e2e.RegexMatch,
				"^Output the currently set compile-time parameters",
			),
		},
		{
			name:    "sessiondir",
			cmdArgs: []string{"SESSIONDIR"},
			exit:    0,
			op: e2e.ExpectOutput(
				e2e.ExactMatch,
				buildcfg.SESSIONDIR,
			),
		},
		{
			name:    "unknown",
			cmdArgs: []string{"UNKNOWN"},
			exit:    1,
		},
		{
			name:    "all",
			cmdArgs: []string{},
			exit:    0,
			op: e2e.ExpectOutput(
				e2e.ContainMatch,
				"SESSIONDIR="+buildcfg.SESSIONDIR,
			),
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.UserProfile),
			e2e.WithCommand("buildcfg"),
			e2e.WithArgs(tt.cmdArgs...),
			e2e.ExpectExit(
				tt.exit,
				tt.op,
			),
		)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := buildcfgTests{
		env: env,
	}

	return testhelper.Tests{
		"basic": c.buildcfgBasic,
	}
}
