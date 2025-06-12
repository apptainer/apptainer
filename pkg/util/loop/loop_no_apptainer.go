// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2021-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build !apptainer_engine

package loop

// GetMaxLoopDevices Return the maximum number of loop devices allowed
func GetMaxLoopDevices() (int, error) {
	// externally imported package, use the default value
	return 256, nil
}
