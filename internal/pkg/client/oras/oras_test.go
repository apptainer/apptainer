// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package oras

import (
	"testing"
)

func TestNormalizeRef(t *testing.T) {
	tests := []struct {
		name     string
		orasRef  string
		expected string
	}{
		{"with tag", "oras://alpine:latest", defaultRegistry + "/library/alpine:latest"},
		{"fully qualified with tag", "oras://user/collection/container:2.0.0", defaultRegistry + "/user/collection/container:2.0.0"},
		{"without tag", "oras://alpine", defaultRegistry + "/library/alpine:" + defaultTag},
		{"with tag variation", "oras://alpine:1.0.1", defaultRegistry + "/library/alpine:1.0.1"},
		{"with registry", "oras://registry.local/collection/container/image:tag1", "registry.local/collection/container/image:tag1"},
		{"with registry without tag", "oras://registry.local/collection/container/image", "registry.local/collection/container/image:" + defaultTag},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeRef(tt.orasRef)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
