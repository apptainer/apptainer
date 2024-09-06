// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"testing"
)

func PrivateRepoLogin(t *testing.T, env TestEnv, profile Profile, reqAuthFile string) {
	args := []string{}
	if reqAuthFile != "" {
		args = append(args, "--authfile", reqAuthFile)
	}
	args = append(args, "-u", DefaultUsername, "-p", DefaultPassword, env.TestRegistryPrivURI)
	env.RunApptainer(
		t,
		WithProfile(profile),
		WithCommand("registry login"),
		WithArgs(args...),
		ExpectExit(0),
	)
}

func PrivateRepoLogout(t *testing.T, env TestEnv, profile Profile, reqAuthFile string) {
	args := []string{}
	if reqAuthFile != "" {
		args = append(args, "--authfile", reqAuthFile)
	}
	args = append(args, env.TestRegistryPrivURI)
	env.RunApptainer(
		t,
		WithProfile(profile),
		WithCommand("registry logout"),
		WithArgs(args...),
		ExpectExit(0),
	)
}
