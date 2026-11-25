// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainerconf

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"golang.org/x/sys/unix"
)

// ParseWithDropIns parses path, then applies regular *.conf files from the
// adjacent conf.d directory in filename order. Each file replaces all prior
// values of the directives it sets. If requireRoot is true, the directory
// and its drop-ins must be root-owned and writable only by root.
func ParseWithDropIns(path string, requireRoot bool) (*File, error) {
	directives, err := readConfigDirectives(path)
	if err != nil {
		return nil, err
	}

	dirPath := filepath.Join(filepath.Dir(path), "conf.d")
	dir, err := os.OpenFile(dirPath, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return GetConfig(directives)
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	if requireRoot {
		info, err := dir.Stat()
		if err != nil {
			return nil, err
		}
		if err := checkDropInOwnership(info, dirPath); err != nil {
			return nil, err
		}
	}

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		return cmp.Compare(a.Name(), b.Name())
	})
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".conf" {
			continue
		}
		dropIn, err := readDropIn(dir, entry.Name(), requireRoot)
		if err != nil {
			return nil, err
		}
		maps.Copy(directives, dropIn)
	}
	return GetConfig(directives)
}

func readDropIn(dir *os.File, name string, requireRoot bool) (Directives, error) {
	path := filepath.Join(dir.Name(), name)
	// Open relative to the directory we checked, even if it has been renamed.
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("while opening %s: %w", path, err)
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", path)
	}
	if requireRoot {
		if err := checkDropInOwnership(info, path); err != nil {
			return nil, err
		}
	}

	directives, err := getDirectives(f, true)
	if err != nil {
		return nil, fmt.Errorf("while parsing %s: %w", path, err)
	}
	return directives, nil
}

func checkDropInOwnership(info os.FileInfo, path string) error {
	if info.Sys().(*syscall.Stat_t).Uid != 0 || info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s must be owned by root and writable only by root", path)
	}
	return nil
}
