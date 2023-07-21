// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build !apptainer_engine

package bin

import (
	"os/exec"
)

// findOnPath falls back to exec.LookPath when not built as part of Apptainer.
func findOnPath(name string, useSuidPath bool) (path string, err error) {
	return exec.LookPath(name)
}
