// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build apparmor

package apparmor

import (
	"fmt"
	"os"

	"github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/procfs"
	"golang.org/x/sys/unix"
)

// Enabled returns whether AppArmor is enabled.
func Enabled() bool {
	data, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
	if err == nil && len(data) > 0 && data[0] == 'Y' {
		return true
	}
	return false
}

// LoadProfile loads the specified AppArmor profile.
func LoadProfile(profile string) error {
	// We must make sure we are actually opening and writing to a real attr/exec
	// in a real procfs so that the profile takes effect. Using
	// pathrs-lite/procfs as below accomplishes this.
	proc, err := procfs.OpenProcRoot()
	if err != nil {
		return err
	}
	defer proc.Close()

	attrExec, closer, err := proc.OpenThreadSelf("attr/exec")
	if err != nil {
		return err
	}
	defer closer()
	defer attrExec.Close()

	f, err := pathrs.Reopen(attrExec, unix.O_WRONLY|unix.O_CLOEXEC)
	if err != nil {
		return err
	}
	defer f.Close()

	p := "exec " + profile
	if _, err := f.Write([]byte(p)); err != nil {
		return fmt.Errorf("failed to set apparmor profile (%s)", err)
	}
	return nil
}
