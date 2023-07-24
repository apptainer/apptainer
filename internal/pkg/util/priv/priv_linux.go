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

type DropPrivFunc func() error

// Escalate escalates privileges of the thread or process.
// Since Go 1.16 syscall.Setresuid is an all-thread operation,
// keep calling syscall directly to restore old behavior of
// changing the UID for the locked thread only.
func Escalate() (DropPrivFunc, error) {
	runtime.LockOSThread()

	uid := os.Getuid()

	_, _, errno := syscall.Syscall(syscall.SYS_SETRESUID, 0, 0, uintptr(uid))
	if errno != 0 {
		return nil, errno
	}

	return func() error {
		_, _, errno := syscall.Syscall(syscall.SYS_SETRESUID, uintptr(uid), uintptr(uid), 0)

		runtime.UnlockOSThread()

		if errno != 0 {
			return errno
		}

		return nil
	}, nil
}
