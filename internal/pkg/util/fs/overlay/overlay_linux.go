// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package overlay

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// statfs is the function pointing to unix.Statfs and
// also used by unit tests for mocking.
var statfs = unix.Statfs

type dir uint8

const (
	_ dir = 1 << iota
	lowerDir
	upperDir
	fuseDir
)

type fs struct {
	name       string
	overlayDir dir
}

const (
	Nfs    int64 = 0x6969
	Fuse   int64 = 0x65735546
	Ecrypt int64 = 0xF15F
	Lustre int64 = 0x0BD00BD0 //nolint:misspell
	Gpfs   int64 = 0x47504653
	Panfs  int64 = 0xAAD7AAEA
)

var incompatibleFs = map[int64]fs{
	// NFS filesystem
	Nfs: {
		name:       "NFS",
		overlayDir: upperDir,
	},
	// FUSE filesystem
	Fuse: {
		name:       "FUSE",
		overlayDir: upperDir | fuseDir,
	},
	// ECRYPT filesystem
	Ecrypt: {
		name:       "ECRYPT",
		overlayDir: lowerDir | upperDir,
	},
	// LUSTRE filesystem
	//nolint:misspell
	Lustre: {
		name:       "LUSTRE",
		overlayDir: lowerDir | upperDir,
	},
	// GPFS filesystem
	Gpfs: {
		name:       "GPFS",
		overlayDir: lowerDir | upperDir,
	},
	// PANFS filesystem
	Panfs: {
		name:       "PANFS",
		overlayDir: lowerDir | upperDir,
	},
}

func check(path string, d dir) error {
	stfs := &unix.Statfs_t{}

	if err := statfs(path, stfs); err != nil {
		return fmt.Errorf("could not retrieve underlying filesystem information for %s: %s", path, err)
	}

	fs, ok := incompatibleFs[int64(stfs.Type)]
	if !ok || (ok && fs.overlayDir&d == 0) {
		return nil
	}

	return &errIncompatibleFs{
		path: path,
		name: fs.name,
		dir:  d,
	}
}

// CheckUpper checks if the underlying filesystem of the
// provided path can be used as an upper overlay directory.
func CheckUpper(path string) error {
	return check(path, upperDir)
}

// CheckLower checks if the underlying filesystem of the
// provided path can be used as lower overlay directory.
func CheckLower(path string) error {
	return check(path, lowerDir)
}

// CheckFuse checks if the filesystem of the provided path
// is of type FUSE and if so return errIncompatibleFs.
func CheckFuse(path string) error {
	return check(path, fuseDir)
}

type errIncompatibleFs struct {
	path string
	name string
	dir  dir
}

func (e *errIncompatibleFs) Error() string {
	// fuseDir is checked as lower layer
	overlayDir := "lower"
	if e.dir == upperDir {
		overlayDir = "upper"
	}
	return fmt.Sprintf(
		"%s is located on a %s filesystem incompatible as overlay %s directory",
		e.path, e.name, overlayDir,
	)
}

// IsIncompatible returns if the error corresponds to
// an incompatible filesystem error.
func IsIncompatible(err error) bool {
	if _, ok := err.(*errIncompatibleFs); ok {
		return true
	}
	return false
}
