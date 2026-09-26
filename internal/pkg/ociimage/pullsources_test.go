// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ociimage

import (
	"os"
	"path/filepath"
	"testing"

	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/types"
)

func TestPullSources(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "registries.conf")
	err := os.WriteFile(conf, []byte(`
[[registry]]
prefix = "docker.io"
location = "rewrite.example.com"
[[registry.mirror]]
location = "mirror.example.com"

[[registry]]
prefix = "quay.io"
location = "quay-mirror.example.com"

[[registry]]
location = "blocked.example.com"
blocked = true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	sys := &types.SystemContext{SystemRegistriesConfPath: conf, SystemRegistriesConfDirPath: "/nonexistent"}

	tests := []struct {
		ref     string
		want    []string
		wantErr bool
	}{
		// mirror first, then the rewritten primary location; never docker.io itself
		{"busybox:1.36", []string{"mirror.example.com/library/busybox:1.36", "rewrite.example.com/library/busybox:1.36"}, false},
		// location-only rewrite
		{"quay.io/prometheus/busybox:latest", []string{"quay-mirror.example.com/prometheus/busybox:latest"}, false},
		// no matching [[registry]]: unchanged
		{"ghcr.io/containerd/busybox:1.36", []string{"ghcr.io/containerd/busybox:1.36"}, false},
		{"blocked.example.com/x:1", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			named, err := reference.ParseNormalizedNamed(tt.ref)
			if err != nil {
				t.Fatal(err)
			}
			got, err := PullSources(sys, named)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d sources %v, want %v", len(got), got, tt.want)
			}
			for i := range got {
				if got[i].Reference.String() != tt.want[i] {
					t.Errorf("source %d: got %s, want %s", i, got[i].Reference, tt.want[i])
				}
			}
		})
	}
}
