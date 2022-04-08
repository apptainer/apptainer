// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package bin

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/util/env"
	"github.com/apptainer/apptainer/pkg/util/apptainerconf"
)

func TestFindOnPath(t *testing.T) {
	// findOnPath should give same as exec.LookPath, but additionally work
	// in the case where $PATH doesn't include default sensible directories
	// as these are added to $PATH before the lookup.

	// Find the true path of 'cp' under a sensible PATH=env.DefaultPath
	// Forcing this avoid issues with PATH across sudo calls for the tests,
	// differing orders, /usr/bin -> /bin symlinks etc.
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", env.DefaultPath)
	defer os.Setenv("PATH", oldPath)
	truePath, err := exec.LookPath("cp")
	if err != nil {
		t.Fatalf("exec.LookPath failed to find cp: %v", err)
	}

	t.Run("sensible path", func(t *testing.T) {
		gotPath, err := findOnPath("cp")
		if err != nil {
			t.Errorf("unexpected error from findOnPath: %v", err)
		}
		if gotPath != truePath {
			t.Errorf("Got %q, expected %q", gotPath, truePath)
		}
	})

	t.Run("bad path", func(t *testing.T) {
		// Force a PATH that doesn't contain cp
		os.Setenv("PATH", "/invalid/dir:/another/invalid/dir")

		gotPath, err := findOnPath("cp")
		if err != nil {
			t.Errorf("unexpected error from findOnPath: %v", err)
		}
		if gotPath != truePath {
			t.Errorf("Got %q, expected %q", gotPath, truePath)
		}
	})
}

func TestFindFromConfigOrPath(t *testing.T) {
	//nolint:dupl
	cases := []struct {
		name          string
		bin           string
		expectSuccess bool
		configKey     string
		configVal     string
		expectPath    string
	}{
		{
			name:          "go valid",
			bin:           "go",
			configKey:     "go path",
			configVal:     "_LOOKPATH_",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "go invalid",
			bin:           "go",
			configKey:     "go path",
			configVal:     "/invalid/dir/go",
			expectSuccess: false,
		},
		{
			name:          "go empty",
			bin:           "go",
			configKey:     "go path",
			configVal:     "",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "mksquashfs valid",
			bin:           "mksquashfs",
			configKey:     "mksquashfs path",
			configVal:     "_LOOKPATH_",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "mksquashfs invalid",
			bin:           "mksquashfs",
			configKey:     "mksquashfs path",
			configVal:     "/invalid/dir/go",
			expectSuccess: false,
		},
		{
			name:          "mksquashfs empty",
			bin:           "mksquashfs",
			configKey:     "mksquashfs path",
			configVal:     "",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "unsquashfs valid",
			bin:           "unsquashfs",
			configKey:     "unsquashfs path",
			configVal:     "_LOOKPATH_",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "unsquashfs invalid",
			bin:           "unsquashfs",
			configKey:     "unsquashfs path",
			configVal:     "/invalid/dir/go",
			expectSuccess: false,
		},
		{
			name:          "unsquashfs empty",
			bin:           "unsquashfs",
			configKey:     "unsquashfs path",
			configVal:     "",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "nvidia-container-cli valid",
			bin:           "nvidia-container-cli",
			configKey:     "nvidia-container-cli path",
			configVal:     "_LOOKPATH_",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "nvidia-container-cli invalid",
			bin:           "nvidia-container-cli",
			configKey:     "nvidia-container-cli path",
			configVal:     "/invalid/dir/go",
			expectSuccess: false,
		},
		{
			name:          "nvidia-container-cli empty",
			bin:           "nvidia-container-cli",
			configKey:     "nvidia-container-cli path",
			configVal:     "",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "cryptsetup valid",
			bin:           "cryptsetup",
			configKey:     "cryptsetup path",
			configVal:     "_LOOKPATH_",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "cryptsetup invalid",
			bin:           "cryptsetup",
			configKey:     "cryptsetup path",
			configVal:     "/invalid/dir/cryptsetup",
			expectSuccess: false,
		},
		{
			name:          "cryptsetup empty",
			bin:           "cryptsetup",
			configKey:     "cryptsetup path",
			configVal:     "",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "ldconfig valid",
			bin:           "ldconfig",
			configKey:     "ldconfig path",
			configVal:     "_LOOKPATH_",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
		{
			name:          "ldconfig invalid",
			bin:           "ldconfig",
			configKey:     "ldconfig path",
			configVal:     "/invalid/dir/go",
			expectSuccess: false,
		},
		{
			name:          "ldconfig empty",
			bin:           "ldconfig",
			configKey:     "ldconfig path",
			configVal:     "",
			expectPath:    "_LOOKPATH_",
			expectSuccess: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if (tc.configVal == "_LOOKPATH_") || tc.expectPath == "_LOOKPATH_" {
				lookPath, err := findOnPath(tc.bin)
				if err != nil {
					t.Skipf("Error from exec.LookPath for %q: %v", tc.bin, err)
				}

				if tc.configVal == "_LOOKPATH_" {
					tc.configVal = lookPath
				}

				if tc.expectPath == "_LOOKPATH_" {
					tc.expectPath = lookPath
				}
			}

			f, err := ioutil.TempFile("", "test.conf")
			if err != nil {
				t.Fatalf("cannot create temporary test configuration: %+v", err)
			}
			f.Close()
			defer os.Remove(f.Name())

			cfg := fmt.Sprintf("%s = %s\n", tc.configKey, tc.configVal)
			ioutil.WriteFile(f.Name(), []byte(cfg), 0o644)

			conf, err := apptainerconf.Parse(f.Name())
			if err != nil {
				t.Errorf("Error parsing test apptainerconf: %v", err)
			}
			apptainerconf.SetCurrentConfig(conf)

			path, err := findFromConfigOrPath(tc.bin)

			if tc.expectSuccess && err == nil {
				// expect success, no error, check path
				if path != tc.expectPath {
					t.Errorf("Expecting %q, got %q", tc.expectPath, path)
				}
			}

			if tc.expectSuccess && err != nil {
				// expect success, got error
				t.Errorf("unexpected error: %v", err)
			}

			if !tc.expectSuccess && err == nil {
				// expect failure, got no error
				t.Errorf("expected error, got %q", path)
			}
		})
	}
}
