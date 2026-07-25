// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"os"

	"golang.org/x/sys/unix"
)

// tempDir returns the default directory to use for temporary files.
// it is a replacement for os.TempDir() that does not return a tmpfs.
func tempDir() string {
	dir := os.Getenv("TMPDIR")
	if dir == "" {
		dir = "/tmp"
		tmpfs, err := isTmpFS(dir)
		if err == nil && tmpfs {
			dir = "/var/tmp"
		}
	}
	return dir
}

func isTmpFS(path string) (bool, error) {
	var sf unix.Statfs_t
	if err := unix.Statfs(path, &sf); err != nil {
		return false, err
	}
	return sf.Type == unix.TMPFS_MAGIC, nil
}
