// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"os"
	"runtime"
	"sync"
	"testing"
)

var (
	ensureMutex sync.Mutex
	pullMutex   sync.Mutex
)

// EnsureImage checks if e2e test image is already built or built
// it otherwise.
func EnsureImage(t *testing.T, env TestEnv) {
	ensureMutex.Lock()
	defer ensureMutex.Unlock()

	switch _, err := os.Stat(env.ImagePath); {
	case err == nil:
		// OK: file exists, return
		return

	case os.IsNotExist(err):
		// OK: file does not exist, continue

	default:
		// FATAL: something else is wrong
		t.Fatalf("Failed when checking image %q: %+v\n",
			env.ImagePath,
			err)
	}

	env.RunApptainer(
		t,
		WithProfile(RootProfile),
		WithCommand("build"),
		WithArgs("--force", env.ImagePath, "testdata/Apptainer"),
		ExpectExit(0),
	)
}

// EnsureSingularityImage checks if e2e test singularity image is already
// built or built it otherwise.
func EnsureSingularityImage(t *testing.T, env TestEnv) {
	ensureMutex.Lock()
	defer ensureMutex.Unlock()

	switch _, err := os.Stat(env.SingularityImagePath); {
	case err == nil:
		// OK: file exists, return
		return

	case os.IsNotExist(err):
		// OK: file does not exist, continue

	default:
		// FATAL: something else is wrong
		t.Fatalf("Failed when checking image %q: %+v\n",
			env.SingularityImagePath,
			err)
	}

	env.RunApptainer(
		t,
		WithProfile(RootProfile),
		WithCommand("build"),
		WithArgs("--force", env.SingularityImagePath, "testdata/Singularity_legacy.def"),
		ExpectExit(0),
	)
}

// PullImage will pull a test image.
func PullImage(t *testing.T, env TestEnv, imageURL string, arch string, path string) {
	pullMutex.Lock()
	defer pullMutex.Unlock()

	if arch == "" {
		arch = runtime.GOARCH
	}

	switch _, err := os.Stat(path); {
	case err == nil:
		// OK: file exists, return
		return

	case os.IsNotExist(err):
		// OK: file does not exist, continue

	default:
		// FATAL: something else is wrong
		t.Fatalf("Failed when checking image %q: %+v\n", path, err)
	}

	env.RunApptainer(
		t,
		WithProfile(UserProfile),
		WithCommand("pull"),
		WithArgs("--force", "--allow-unsigned", "--arch", arch, path, imageURL),
		ExpectExit(0),
	)
}
