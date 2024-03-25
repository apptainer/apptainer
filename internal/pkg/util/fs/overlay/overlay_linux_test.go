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
	"testing"

	"golang.org/x/sys/unix"
)

func TestCheckLowerUpper(t *testing.T) {
	tests := []struct {
		name                  string
		path                  string
		fsName                string
		fsType                int64
		dir                   dir
		expectedSuccess       bool
		expectIncompatibleErr bool
	}{
		{
			name:                  "Root filesystem lower",
			path:                  "/",
			fsName:                "none",
			dir:                   lowerDir,
			expectedSuccess:       true,
			expectIncompatibleErr: false,
		},
		{
			name:                  "Root filesystem upper",
			path:                  "/",
			fsName:                "none",
			dir:                   upperDir,
			expectedSuccess:       true,
			expectIncompatibleErr: false,
		},
		{
			name:                  "Non existent path lower",
			path:                  "/non/existent/path",
			fsName:                "none",
			dir:                   lowerDir,
			expectedSuccess:       false,
			expectIncompatibleErr: false,
		},
		{
			name:                  "Non existent path upper",
			path:                  "/non/existent/path",
			fsName:                "none",
			dir:                   upperDir,
			expectedSuccess:       false,
			expectIncompatibleErr: false,
		},
		{
			name:                  "NFS mock lower",
			path:                  "/",
			fsName:                "NFS",
			dir:                   lowerDir,
			fsType:                Nfs,
			expectedSuccess:       true,
			expectIncompatibleErr: false,
		},
		{
			name:                  "NFS mock upper",
			path:                  "/",
			fsName:                "NFS",
			dir:                   upperDir,
			fsType:                Nfs,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		{
			name:                  "FUSE mock lower",
			path:                  "/",
			fsName:                "FUSE",
			dir:                   lowerDir,
			fsType:                Fuse,
			expectedSuccess:       true,
			expectIncompatibleErr: false,
		},
		{
			name:                  "FUSE mock upper",
			path:                  "/",
			fsName:                "FUSE",
			dir:                   upperDir,
			fsType:                Fuse,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		{
			name:                  "ECRYPT mock lower",
			path:                  "/",
			fsName:                "ECRYPT",
			dir:                   lowerDir,
			fsType:                Ecrypt,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		{
			name:                  "ECRYPT mock upper",
			path:                  "/",
			fsName:                "ECRYPT",
			dir:                   upperDir,
			fsType:                Ecrypt,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		//nolint:misspell
		{
			name:                  "LUSTRE mock lower",
			path:                  "/",
			fsName:                "LUSTRE",
			dir:                   lowerDir,
			fsType:                Lustre,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		//nolint:misspell
		{
			name:                  "LUSTRE mock upper",
			path:                  "/",
			fsName:                "LUSTRE",
			dir:                   upperDir,
			fsType:                Lustre,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		{
			name:                  "GPFS mock lower",
			path:                  "/",
			fsName:                "GPFS",
			dir:                   lowerDir,
			fsType:                Gpfs,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		{
			name:                  "GPFS mock upper",
			path:                  "/",
			fsName:                "GPFS",
			dir:                   upperDir,
			fsType:                Gpfs,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		{
			name:                  "PANFS mock lower",
			path:                  "/",
			fsName:                "PANFS",
			dir:                   lowerDir,
			fsType:                Panfs,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
		{
			name:                  "PANFS mock upper",
			path:                  "/",
			fsName:                "PANFS",
			dir:                   upperDir,
			fsType:                Panfs,
			expectedSuccess:       false,
			expectIncompatibleErr: true,
		},
	}

	if IsIncompatible(nil) {
		t.Errorf("IsIncompatible with nil error returned true")
	}

	for _, tt := range tests {
		var err error

		// mock statfs
		if tt.fsType > 0 {
			statfs = func(_ string, st *unix.Statfs_t) error {
				st.Type = tt.fsType
				return nil
			}
		} else {
			statfs = unix.Statfs
		}

		switch tt.dir {
		case lowerDir:
			err = CheckLower(tt.path)
		case upperDir:
			err = CheckUpper(tt.path)
		}

		if err != nil && tt.expectedSuccess {
			t.Errorf("unexpected error for %q: %s", tt.name, err)
		} else if err == nil && !tt.expectedSuccess {
			t.Errorf("unexpected success for %q", tt.name)
		} else if err != nil && !tt.expectedSuccess {
			if !tt.expectIncompatibleErr {
				continue
			}
			expectedError := &errIncompatibleFs{
				path: tt.path,
				name: tt.fsName,
				dir:  tt.dir,
			}
			if IsIncompatible(err) {
				if expectedError.Error() == err.Error() {
					continue
				}
			}
			t.Errorf("unexpected error for %q: %q instead of %q", tt.name, err, expectedError)
		}
	}
}
