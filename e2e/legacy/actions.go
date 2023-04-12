// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package legacy

import (
	"fmt"
	"os"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/e2e/internal/testhelper"
	"github.com/apptainer/apptainer/internal/pkg/test/tool/require"
	"github.com/pkg/errors"
)

type legacyTests struct {
	env e2e.TestEnv
}

const containerTestImage = "/test.img"

// run singularity inside singularity legacy container.
func (c legacyTests) singularityCmd(command string, args ...string) []string {
	apptainerArgs := []string{
		"--bind", fmt.Sprintf("%s:%s", c.env.ImagePath, containerTestImage),
		c.env.SingularityImagePath,
		"singularity",
		command,
	}
	return append(apptainerArgs, args...)
}

// run tests min fuctionality for apptainer run
func (c legacyTests) runLegacy(t *testing.T) {
	require.Arch(t, "amd64")

	e2e.EnsureImage(t, c.env)
	e2e.EnsureSingularityImage(t, c.env)

	tests := []struct {
		name string
		argv []string
		exit int
	}{
		{
			name: "NoCommand",
			argv: c.singularityCmd("run", containerTestImage),
			exit: 0,
		},
		{
			name: "True",
			argv: c.singularityCmd("run", containerTestImage, "true"),
			exit: 0,
		},
		{
			name: "False",
			argv: c.singularityCmd("run", containerTestImage, "false"),
			exit: 1,
		},
		{
			name: "ScifTestAppGood",
			argv: c.singularityCmd("run", "--app", "testapp", containerTestImage),
			exit: 0,
		},
		{
			name: "ScifTestAppBad",
			argv: c.singularityCmd("run", "--app", "fakeapp", containerTestImage),
			exit: 1,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.argv...),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// exec tests min fuctionality for apptainer exec
func (c legacyTests) execLegacy(t *testing.T) {
	require.Arch(t, "amd64")

	e2e.EnsureImage(t, c.env)
	e2e.EnsureSingularityImage(t, c.env)

	tests := []struct {
		name string
		argv []string
		exit int
	}{
		{
			name: "NoCommand",
			argv: c.singularityCmd("exec", containerTestImage),
			exit: 1,
		},
		{
			name: "True",
			argv: c.singularityCmd("exec", containerTestImage, "true"),
			exit: 0,
		},
		{
			name: "TrueAbsPAth",
			argv: c.singularityCmd("exec", containerTestImage, "/bin/true"),
			exit: 0,
		},
		{
			name: "False",
			argv: c.singularityCmd("exec", containerTestImage, "false"),
			exit: 1,
		},
		{
			name: "FalseAbsPath",
			argv: c.singularityCmd("exec", containerTestImage, "/bin/false"),
			exit: 1,
		},
		// Scif apps tests
		{
			name: "ScifTestAppGood",
			argv: c.singularityCmd("exec", "--app", "testapp", containerTestImage, "testapp.sh"),
			exit: 0,
		},
		{
			name: "ScifTestAppBad",
			argv: c.singularityCmd("exec", "--app", "fakeapp", containerTestImage, "testapp.sh"),
			exit: 1,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-d", "/scif"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-d", "/scif/apps"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-d", "/scif/data"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-d", "/scif/apps/foo"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-d", "/scif/apps/bar"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-f", "/scif/apps/foo/filefoo.exec"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-f", "/scif/apps/bar/filebar.exec"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-d", "/scif/data/foo/output"),
			exit: 0,
		},
		{
			name: "ScifTestfolderOrg",
			argv: c.singularityCmd("exec", containerTestImage, "test", "-d", "/scif/data/foo/input"),
			exit: 0,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(tt.argv...),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// Shell interaction tests
func (c legacyTests) shellLegacy(t *testing.T) {
	require.Arch(t, "amd64")

	e2e.EnsureImage(t, c.env)
	e2e.EnsureSingularityImage(t, c.env)

	hostname, err := os.Hostname()
	err = errors.Wrap(err, "getting hostname")
	if err != nil {
		t.Fatalf("could not get hostname: %+v", err)
	}

	tests := []struct {
		name       string
		consoleOps []e2e.ApptainerConsoleOp
		exit       int
	}{
		{
			name: "ShellExit",
			consoleOps: []e2e.ApptainerConsoleOp{
				// "cd /" to work around issue where a long
				// working directory name causes the test
				// to fail because the "Apptainer" that
				// we are looking for is chopped from the
				// front.
				// TODO(mem): This test was added back in 491a71716013654acb2276e4b37c2e015d2dfe09
				e2e.ConsoleSendLine("cd /"),
				e2e.ConsoleExpect("Singularity"),
				e2e.ConsoleSendLine("exit"),
			},
			exit: 0,
		},
		{
			name: "ShellHostname",
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("hostname"),
				e2e.ConsoleExpect(hostname),
				e2e.ConsoleSendLine("exit"),
			},
			exit: 0,
		},
		{
			name: "ShellBadCommand",
			consoleOps: []e2e.ApptainerConsoleOp{
				e2e.ConsoleSendLine("_a_fake_command"),
				e2e.ConsoleSendLine("exit"),
			},
			exit: 127,
		},
	}

	for _, tt := range tests {
		c.env.RunApptainer(
			t,
			e2e.AsSubtest(tt.name),
			e2e.WithProfile(e2e.RootProfile),
			e2e.WithCommand("exec"),
			e2e.WithArgs(c.singularityCmd("shell", containerTestImage)...),
			e2e.ConsoleRun(tt.consoleOps...),
			e2e.ExpectExit(tt.exit),
		)
	}
}

// E2ETests is the main func to trigger the test suite
func E2ETests(env e2e.TestEnv) testhelper.Tests {
	c := legacyTests{
		env: env,
	}

	// legacy tests run sequentially due to loop device issue,
	// see https://github.com/apptainer/apptainer/issues/1272
	np := testhelper.NoParallel

	return testhelper.Tests{
		"run legacy":   np(c.runLegacy),
		"shell legacy": np(c.shellLegacy),
		"exec legacy":  np(c.execLegacy),
	}
}
