// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package priv

import (
	"os"
	"runtime"
	"syscall"
)

// Escalate escalates privileges of the thread or process.
// Since Go 1.16 syscall.Setresuid is an all-thread operation.
// A runtime.LockOSThread operation remains for older versions of Go.
func Escalate() error {
	runtime.LockOSThread()
	uid := os.Getuid()
	return syscall.Setresuid(0, 0, uid)
}

// Drop drops privileges of the thread or process.
// Since Go 1.16 syscall.Setresuid is an all-thread operation.
// A runtime.LockOSThread operation remains for older versions of Go.
func Drop() error {
	defer runtime.UnlockOSThread()
	uid := os.Getuid()
	return syscall.Setresuid(uid, uid, 0)
}
