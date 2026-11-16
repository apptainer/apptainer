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
	"strings"
	"testing"

	ggcr "github.com/google/go-containerregistry/pkg/v1"
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

func TestBaseName(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		tag     string
		digest  string
		want    string
		wantErr bool
	}{
		{
			name:    "tag preferred over digest",
			repo:    "docker.io/library/busybox",
			tag:     "latest",
			digest:  "sha256:abc",
			want:    defaultRegistry + "/library/busybox:latest",
			wantErr: false,
		},
		{
			name:    "tag preferred over digest, custom registry",
			repo:    "registry.local:5000/my/repo",
			tag:     "tag",
			digest:  "sha256:def",
			want:    "registry.local:5000/my/repo:tag",
			wantErr: false,
		},
		{
			name:    "digest when no tag",
			repo:    "docker.io/library/busybox",
			tag:     "",
			digest:  "sha256:abc",
			want:    defaultRegistry + "/library/busybox@sha256:abc",
			wantErr: false,
		},
		{
			name:    "default tag when neither",
			repo:    "docker.io/library/busybox",
			tag:     "",
			digest:  "",
			want:    defaultRegistry + "/library/busybox:latest",
			wantErr: false,
		},
		{
			name:    "unqualified repo canonicalized to default registry",
			repo:    "busybox",
			tag:     "latest",
			digest:  "",
			want:    defaultRegistry + "/library/busybox:latest",
			wantErr: false,
		},
		{
			name:    "unqualified repo canonicalized, no tag or digest",
			repo:    "busybox",
			tag:     "",
			digest:  "",
			want:    defaultRegistry + "/library/busybox:latest",
			wantErr: false,
		},
		{
			name:    "empty repo errors",
			repo:    "",
			tag:     "latest",
			digest:  "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BaseName(tt.repo, tt.tag, tt.digest)
			if tt.wantErr {
				if err == nil {
					t.Errorf("BaseName(%q, %q, %q) expected error", tt.repo, tt.tag, tt.digest)
				}
				return
			}
			if err != nil {
				t.Fatalf("BaseName(%q, %q, %q) unexpected error: %v", tt.repo, tt.tag, tt.digest, err)
			}
			if got != tt.want {
				t.Errorf("BaseName(%q, %q, %q) = %q, want %q", tt.repo, tt.tag, tt.digest, got, tt.want)
			}
		})
	}
}

func TestDigestReference(t *testing.T) {
	digest := ggcr.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}
	tests := []struct {
		name     string
		ref      string
		noHTTPS  bool
		expected string
	}{
		{
			name:     "tagged reference",
			ref:      "oras://alpine:latest",
			expected: defaultRegistry + "/library/alpine@" + digest.String(),
		},
		{
			name:     "reference with registry",
			ref:      "//registry.local/team/image:tag",
			noHTTPS:  true,
			expected: "registry.local/team/image@" + digest.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := digestReference(tt.ref, digest, tt.noHTTPS)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}
