// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package bin

import (
	"os"
	"os/exec"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/util/env"
)

func TestFindOnPath(t *testing.T) {
	// findOnPath should give same as exec.LookPath, but additionally work
	// in the case where $PATH doesn't include default sensible directories
	// as these are added to $PATH before the lookup.

	// Find the true path of 'cp' under a sensible PATH=env.DefaultPath
	// Forcing this avoid issues with PATH across sudo calls for the tests,
	// differing orders, /usr/bin -> /bin symlinks etc.
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", env.DefaultPath)
	defer os.Setenv("PATH", oldPath)
	truePath, err := exec.LookPath("cp")
	if err != nil {
		t.Fatalf("exec.LookPath failed to find cp: %v", err)
	}

	t.Run("sensible path", func(t *testing.T) {
		gotPath, err := findOnPath("cp", false)
		if err != nil {
			t.Errorf("unexpected error from findOnPath: %v", err)
		}
		if gotPath != truePath {
			t.Errorf("Got %q, expected %q", gotPath, truePath)
		}
	})

	t.Run("bad path", func(t *testing.T) {
		// Force a PATH that doesn't contain cp
		os.Setenv("PATH", "/invalid/dir:/another/invalid/dir")

		gotPath, err := findOnPath("cp", false)
		if err != nil {
			t.Errorf("unexpected error from findOnPath: %v", err)
		}
		if gotPath != truePath {
			t.Errorf("Got %q, expected %q", gotPath, truePath)
		}
	})
}
