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
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestChtimesATime tests Chtimes access time on a tempfile.
func TestChtimesATime(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exist")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	beforeUnixEpochTime := unixEpochTime.Add(-100 * time.Second)
	afterUnixEpochTime := unixEpochTime.Add(100 * time.Second)

	// Test both aTime and mTime set to Unix Epoch
	t.Run("both aTime and mTime set to Unix Epoch", func(t *testing.T) {
		if err := Chtimes(file, unixEpochTime, unixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		stat := f.Sys().(*syscall.Stat_t) //nolint:forcetypeassert
		aTime := time.Unix(stat.Atim.Unix())
		if aTime != unixEpochTime {
			t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
		}
	})

	// Test aTime before Unix Epoch and mTime set to Unix Epoch
	t.Run("aTime before Unix Epoch and mTime set to Unix Epoch", func(t *testing.T) {
		if err := Chtimes(file, beforeUnixEpochTime, unixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		stat := f.Sys().(*syscall.Stat_t) //nolint:forcetypeassert
		aTime := time.Unix(stat.Atim.Unix())
		if aTime != unixEpochTime {
			t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
		}
	})

	// Test aTime set to Unix Epoch and mTime before Unix Epoch
	t.Run("aTime set to Unix Epoch and mTime before Unix Epoch", func(t *testing.T) {
		if err := Chtimes(file, unixEpochTime, beforeUnixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		stat := f.Sys().(*syscall.Stat_t) //nolint:forcetypeassert
		aTime := time.Unix(stat.Atim.Unix())
		if aTime != unixEpochTime {
			t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
		}
	})

	// Test both aTime and mTime set to after Unix Epoch (valid time)
	t.Run("both aTime and mTime set to after Unix Epoch (valid time)", func(t *testing.T) {
		if err := Chtimes(file, afterUnixEpochTime, afterUnixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		stat := f.Sys().(*syscall.Stat_t) //nolint:forcetypeassert
		aTime := time.Unix(stat.Atim.Unix())
		if aTime != afterUnixEpochTime {
			t.Fatalf("Expected: %s, got: %s", afterUnixEpochTime, aTime)
		}
	})

	// Test both aTime and mTime set to Unix max time
	t.Run("both aTime and mTime set to Unix max time", func(t *testing.T) {
		if err := Chtimes(file, unixMaxTime, unixMaxTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		stat := f.Sys().(*syscall.Stat_t) //nolint:forcetypeassert
		aTime := time.Unix(stat.Atim.Unix())
		if aTime.Truncate(time.Second) != unixMaxTime.Truncate(time.Second) {
			t.Fatalf("Expected: %s, got: %s", unixMaxTime.Truncate(time.Second), aTime.Truncate(time.Second))
		}
	})
}

// prepareFiles creates files for testing in the temp directory
func prepareFiles(t *testing.T) (file, invalid, symlink string) {
	t.Helper()
	dir := t.TempDir()

	file = filepath.Join(dir, "exist")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	invalid = filepath.Join(dir, "doesnt-exist")
	symlink = filepath.Join(dir, "symlink")
	if err := os.Symlink(file, symlink); err != nil {
		t.Fatal(err)
	}

	return file, invalid, symlink
}

func TestLUtimesNano(t *testing.T) {
	file, invalid, symlink := prepareFiles(t)

	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	ts := []syscall.Timespec{{Sec: 0, Nsec: 0}, {Sec: 0, Nsec: 0}}
	if err := LUtimesNano(symlink, ts); err != nil {
		t.Fatal(err)
	}

	symlinkInfo, err := os.Lstat(symlink)
	if err != nil {
		t.Fatal(err)
	}
	if before.ModTime().Unix() == symlinkInfo.ModTime().Unix() {
		t.Fatal("The modification time of the symlink should be different")
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if before.ModTime().Unix() != fileInfo.ModTime().Unix() {
		t.Fatal("The modification time of the file should be same")
	}

	if err := LUtimesNano(invalid, ts); err == nil {
		t.Fatal("Doesn't return an error on a non-existing file")
	}
}
