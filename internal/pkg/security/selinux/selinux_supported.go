// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build selinux
// +build selinux

package selinux

import "github.com/opencontainers/selinux/go-selinux"

// Enabled returns whether SELinux is enabled.
func Enabled() bool {
	return selinux.GetEnabled()
}

// SetExecLabel sets the SELinux label for current process.
func SetExecLabel(label string) error {
	return selinux.SetExecLabel(label)
}
