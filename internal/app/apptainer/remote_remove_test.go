// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package apptainer

import (
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
)

// Note that the valid use cases are in remote_add_test.go. We still have tests
// here for all the corner cases of RemoteRemove()
func TestRemoteRemove(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	validCfgFile := createValidCfgFile(t) // from remote_add_test.go
	defer os.Remove(validCfgFile)

	tests := []struct {
		name       string
		cfgFile    string
		remoteName string
		shallPass  bool
	}{
		{
			name:       "empty config file; empty remote name",
			cfgFile:    "",
			remoteName: "",
			shallPass:  false,
		},
		{
			name:       "valid config file; empty remote name",
			cfgFile:    validCfgFile,
			remoteName: "",
			shallPass:  false,
		},
		{
			name:       "valid config file; valid remote name; default",
			cfgFile:    validCfgFile,
			remoteName: "cloud_testing",
			shallPass:  true,
		},
		{
			name:       "valid config file; valid remote name; not default",
			cfgFile:    validCfgFile,
			remoteName: "cloud_testing2",
			shallPass:  true,
		},
	}

	// Add remotes based on our config file
	if err := RemoteAdd(validCfgFile, "cloud_testing", "cloud.random.io", false, false, true); err != nil {
		t.Fatalf("cannot add remote \"cloud\" for testing: %s\n", err)
	}
	if err := RemoteAdd(validCfgFile, "cloud_testing2", "cloud2.random.io", false, false, false); err != nil {
		t.Fatalf("cannot add remote \"cloud\" for testing: %s\n", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test.DropPrivilege(t)
			defer test.ResetPrivilege(t)

			err := RemoteRemove(tt.cfgFile, tt.remoteName)
			if tt.shallPass == true && err != nil {
				t.Fatalf("valid case failed: %s\n", err)
			}
			if tt.shallPass == false && err == nil {
				t.Fatal("invalid case succeeded")
			}
		})
	}
}
