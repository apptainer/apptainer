// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2022 Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package e2e

import (
	"os"
	"runtime"
	"syscall"
	"testing"

	"github.com/pkg/errors"
)

var (
	// uid of original user running test.
	origUID = os.Getuid()
	// gid of original group running test.
	origGID = os.Getgid()
)

// OrigUID returns the UID of the user running the test suite.
func OrigUID() int {
	return origUID
}

// OrigGID returns the GID of the user running the test suite.
func OrigGID() int {
	return origGID
}

// ThreadSetresuid performs a syscall setting uid for the current thread only.
// This is required as in Go 1.16 syscall.Setresuid is all-threads, and the
// newest x/sys/unix functions use this, so are all threads.
func ThreadSetresuid(ruid, euid, suid int) (err error) {
	if _, _, e1 := syscall.Syscall(syscall.SYS_SETRESUID, uintptr(ruid), uintptr(euid), uintptr(suid)); e1 != 0 {
		err = syscall.Errno(e1)
	}
	return err
}

// ThreadSetresgid performs a syscall setting gid for the current thread only. This
// is required as in Go 1.16 syscall.Setresuid is all-threads, and the newest
// x/sys/unix functions use this, so are all threads.
func ThreadSetresgid(rgid, egid, sgid int) (err error) {
	if _, _, e1 := syscall.Syscall(syscall.SYS_SETRESGID, uintptr(rgid), uintptr(egid), uintptr(sgid)); e1 != 0 {
		err = syscall.Errno(e1)
	}
	return err
}

// Privileged wraps the supplied test function with calls to ensure the test is
// run with elevated privileges applied to the current thread, and the current
// goroutine locked to this thread.
func Privileged(f func(*testing.T)) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()

		runtime.LockOSThread()

		if err := ThreadSetresuid(0, 0, origUID); err != nil {
			err = errors.Wrap(err, "changing user ID to 0")
			t.Fatalf("privileges escalation failed: %+v", err)
		}
		if err := ThreadSetresgid(0, 0, origGID); err != nil {
			err = errors.Wrap(err, "changing group ID to 0")
			t.Fatalf("privileges escalation failed: %+v", err)
		}

		defer func() {
			if err := ThreadSetresgid(origGID, origGID, 0); err != nil {
				err = errors.Wrapf(err, "changing group ID to %d", origUID)
				t.Fatalf("privileges drop failed: %+v", err)
			}
			if err := ThreadSetresuid(origUID, origUID, 0); err != nil {
				err = errors.Wrapf(err, "changing group ID to %d", origGID)
				t.Fatalf("privileges drop failed: %+v", err)
			}
			runtime.UnlockOSThread()
		}()

		f(t)
	}
}
