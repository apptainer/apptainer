// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
/*
Contains code adapted from:

	https://github.com/moby/moby/blob/master/daemon/internal/system

Copyright 2013-2018 Docker, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	    https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/
package archive

import (
	"errors"
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Used by Chtimes
var unixEpochTime, unixMaxTime time.Time

func init() {
	unixEpochTime = time.Unix(0, 0)
	if unsafe.Sizeof(syscall.Timespec{}.Nsec) == 8 {
		// This is a 64 bit timespec
		// os.Chtimes limits time to the following
		//
		// Note that this intentionally sets nsec (not sec), which sets both sec
		// and nsec internally in time.Unix();
		// https://github.com/golang/go/blob/go1.19.2/src/time/time.go#L1364-L1380
		unixMaxTime = time.Unix(0, 1<<63-1)
	} else {
		// This is a 32 bit timespec
		unixMaxTime = time.Unix(1<<31-1, 0)
	}
}

// Chtimes changes the access time and modified time of a file at the given path.
// If the modified time is prior to the Unix Epoch (unixMinTime), or after the
// end of Unix Time (unixEpochTime), os.Chtimes has undefined behavior. In this
// case, Chtimes defaults to Unix Epoch, just in case.
func Chtimes(name string, atime time.Time, mtime time.Time) error {
	if atime.Before(unixEpochTime) || atime.After(unixMaxTime) {
		atime = unixEpochTime
	}

	if mtime.Before(unixEpochTime) || mtime.After(unixMaxTime) {
		mtime = unixEpochTime
	}

	if err := os.Chtimes(name, atime, mtime); err != nil {
		return err
	}

	return nil
}

// LUtimesNano is used to change access and modification time of the specified path.
// It's used for symbol link file because unix.UtimesNano doesn't support a NOFOLLOW flag atm.
func LUtimesNano(path string, ts []syscall.Timespec) error {
	uts := []unix.Timespec{
		unix.NsecToTimespec(syscall.TimespecToNsec(ts[0])),
		unix.NsecToTimespec(syscall.TimespecToNsec(ts[1])),
	}
	err := unix.UtimesNanoAt(unix.AT_FDCWD, path, uts, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil && !errors.Is(err, unix.ENOSYS) {
		return err
	}

	return nil
}

type XattrError struct {
	Op   string
	Attr string
	Path string
	Err  error
}

func (e *XattrError) Error() string { return e.Op + " " + e.Attr + " " + e.Path + ": " + e.Err.Error() }

func (e *XattrError) Unwrap() error { return e.Err }

// Timeout reports whether this error represents a timeout.
func (e *XattrError) Timeout() bool {
	t, ok := e.Err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}

// Lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// It returns a nil slice and nil error if the xattr is not set.
func Lgetxattr(path string, attr string) ([]byte, error) {
	sysErr := func(err error) ([]byte, error) {
		return nil, &XattrError{Op: "lgetxattr", Attr: attr, Path: path, Err: err}
	}

	// Start with a 128 length byte array
	dest := make([]byte, 128)
	sz, errno := unix.Lgetxattr(path, attr, dest)

	for errors.Is(errno, unix.ERANGE) {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, errno = unix.Lgetxattr(path, attr, []byte{})
		if errno != nil {
			return sysErr(errno)
		}
		dest = make([]byte, sz)
		sz, errno = unix.Lgetxattr(path, attr, dest)
	}

	switch {
	case errors.Is(errno, unix.ENODATA):
		return nil, nil
	case errno != nil:
		return sysErr(errno)
	}

	return dest[:sz], nil
}

// Lsetxattr sets the value of the extended attribute identified by attr
// and associated with the given path in the file system.
func Lsetxattr(path string, attr string, data []byte, flags int) error {
	err := unix.Lsetxattr(path, attr, data, flags)
	if err != nil {
		return &XattrError{Op: "lsetxattr", Attr: attr, Path: path, Err: err}
	}
	return nil
}
