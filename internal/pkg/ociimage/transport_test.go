// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ociimage

import (
	"testing"
)

func TestSupportedTransport(t *testing.T) {
	// We individually check all the known transports. This is a
	// very naive test since mimicking the actual code but still ensures
	// that everything is consistent
	for _, transport := range ociTransports {
		if SupportedTransport(transport) == "" {
			t.Fatalf("transport %s reported as not supported", transport)
		}
	}

	// Now error cases
	tests := []struct {
		name      string
		transport string
	}{
		{
			name:      "empty",
			transport: "",
		},
		{
			name:      "random",
			transport: "fake",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if SupportedTransport(tt.transport) != "" {
				t.Fatalf("invalid transport %s reported as supported", tt.transport)
			}
		})
	}
}
