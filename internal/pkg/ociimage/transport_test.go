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

func TestSplitTagAndDigest(t *testing.T) {
	tests := []struct {
		name   string
		ref    string
		repo   string
		tag    string
		digest string
	}{
		{
			name:   "tag and digest",
			ref:    "docker.io/library/busybox:latest@sha256:abc",
			repo:   "docker.io/library/busybox",
			tag:    "latest",
			digest: "sha256:abc",
		},
		{
			name:   "digest only",
			ref:    "docker.io/library/busybox@sha256:abc",
			repo:   "docker.io/library/busybox",
			tag:    "",
			digest: "sha256:abc",
		},
		{
			name:   "tag only",
			ref:    "docker.io/library/busybox:latest",
			repo:   "docker.io/library/busybox",
			tag:    "latest",
			digest: "",
		},
		{
			name:   "name only",
			ref:    "docker.io/library/busybox",
			repo:   "docker.io/library/busybox",
			tag:    "",
			digest: "",
		},
		{
			name:   "registry with port and tag",
			ref:    "docker.io:5000/myrepo:tag",
			repo:   "docker.io:5000/myrepo",
			tag:    "tag",
			digest: "",
		},
		{
			name:   "registry with port, tag and digest",
			ref:    "docker.io:5000/myrepo:tag@sha256:def",
			repo:   "docker.io:5000/myrepo",
			tag:    "tag",
			digest: "sha256:def",
		},
		{
			name:   "empty",
			ref:    "",
			repo:   "",
			tag:    "",
			digest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, tag, digest := SplitTagAndDigest(tt.ref)
			if repo != tt.repo {
				t.Errorf("repo = %q, want %q", repo, tt.repo)
			}
			if tag != tt.tag {
				t.Errorf("tag = %q, want %q", tag, tt.tag)
			}
			if digest != tt.digest {
				t.Errorf("digest = %q, want %q", digest, tt.digest)
			}
		})
	}
}

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
