// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2022 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import "testing"

// WithCgroupManagers is a wrapper to call test function f in both the systemd and
// cgroupfs cgroup manager configurations. It *must* be run noparallel, as the
// cgroup manager setting is set / read from global configuration.
func (env TestEnv) WithCgroupManagers(f func(t *testing.T)) func(t *testing.T) {
	return func(t *testing.T) {
		env.RunApptainer(
			t,
			WithProfile(RootProfile),
			WithCommand("config global"),
			WithArgs("--set", "systemd cgroups", "yes"),
			ExpectExit(0),
		)

		defer env.RunApptainer(
			t,
			WithProfile(RootProfile),
			WithCommand("config global"),
			WithArgs("--reset", "systemd cgroups"),
			ExpectExit(0),
		)

		t.Run("systemd", f)

		env.RunApptainer(
			t,
			WithProfile(RootProfile),
			WithCommand("config global"),
			WithArgs("--set", "systemd cgroups", "no"),
			ExpectExit(0),
		)

		t.Run("cgroupfs", f)
	}
}
