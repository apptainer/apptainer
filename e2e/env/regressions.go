// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainerenv

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/apptainer/apptainer/e2e/internal/e2e"
	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/pkg/util/rlimit"
)

// Check that an old-style `/environment` file is interpreted
// and can set PATH.
func (c ctx) issue5426(t *testing.T) {
	sandboxDir, cleanup := e2e.MakeTempDir(t, c.env.TestDir, "sandbox-", "")
	defer cleanup(t)

	// Build a current sandbox
	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("build"),
		e2e.WithArgs("--force", "--sandbox", sandboxDir, e2e.BusyboxSIF(t)),
		e2e.ExpectExit(0),
	)

	// Remove the /.singularity.d
	if err := os.RemoveAll(path.Join(sandboxDir, ".singularity.d")); err != nil {
		t.Fatalf("Could not remove sandbox /.singularity.d: %s", err)
	}
	// Remove the /environment symlink
	if err := os.Remove(path.Join(sandboxDir, "environment")); err != nil {
		t.Fatalf("Could not remove sandbox /environment symlink: %s", err)
	}
	// Copy in the test environment file
	testEnvironment := path.Join("testdata", "regressions", "legacy-environment")
	if err := fs.CopyFile(testEnvironment, path.Join(sandboxDir, "environment"), 0o755); err != nil {
		t.Fatalf("Could not add legacy /environment to sandbox: %s", err)
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs(sandboxDir, "/bin/sh", "-c", "echo $PATH"),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ContainMatch, "/canary/path")),
	)
}

// Check that we hit engine configuration size limit with a rather big
// configuration by passing some big environment variables.
func (c ctx) issue5057(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	cur, _, err := rlimit.Get("RLIMIT_STACK")
	if err != nil {
		t.Fatalf("Could not determine stack size limit: %s", err)
	}
	if buildcfg.MAX_ENGINE_CONFIG_SIZE >= cur/4 {
		t.Skipf("stack limit too low")
	}

	maxChunkSize := uint64(buildcfg.MAX_CHUNK_SIZE)

	big := make([]byte, maxChunkSize)
	for i := uint64(0); i < maxChunkSize; i++ {
		big[i] = 'A'
	}
	bigEnv := make([]string, buildcfg.MAX_ENGINE_CONFIG_CHUNK)
	for i := range bigEnv {
		bigEnv[i] = fmt.Sprintf("B%d=%s", i, string(big))
	}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithEnv(bigEnv),
		e2e.WithArgs(c.env.ImagePath, "true"),
		e2e.ExpectExit(255),
	)

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithEnv(bigEnv[:buildcfg.MAX_ENGINE_CONFIG_CHUNK-1]),
		e2e.WithArgs(c.env.ImagePath, "true"),
		e2e.ExpectExit(0),
	)
}

// If a $ in a APPTAINERENV_ env var is escaped, it should become a
// literal $ in the container env var.
// This allows setting e.g. LD_PRELOAD=/foo/bar/$LIB/baz.so
func (c ctx) issue43(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	env := []string{`APPTAINERENV_LD_PRELOAD=/foo/bar/\$LIB/baz.so`}
	args := []string{c.env.ImagePath, "/bin/sh", "-c", "echo \"${LD_PRELOAD}\""}

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithEnv(env),
		e2e.WithArgs(args...),
		e2e.ExpectExit(
			0,
			e2e.ExpectOutput(e2e.ExactMatch, `/foo/bar/$LIB/baz.so`),
		),
	)
}

// https://github.com/sylabs/singularity/issues/1263
// With --env-file we should avoid any override of EUID/UID/GID that are set readonly by bash.
func (c ctx) issue1263(t *testing.T) {
	e2e.EnsureImage(t, c.env)

	// An empty env file is sufficient, as EUID/UID/GID come from the mvdan.cc/sh evaluation of it.
	envFile, err := e2e.WriteTempFile(c.env.TestDir, "env-file", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Remove(envFile)
	})

	c.env.RunApptainer(
		t,
		e2e.WithProfile(e2e.UserProfile),
		e2e.WithCommand("exec"),
		e2e.WithArgs("--env-file", envFile, c.env.ImagePath, "/bin/true"),
		e2e.ExpectExit(
			0,
			e2e.ExpectError(e2e.UnwantedContainMatch, "readonly variable"),
		),
	)
}
