//go:build !linux || !libsubid || !cgo
// +build !linux !libsubid !cgo

// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package fakeroot

import (
	"fmt"
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type set struct {
	name            string
	path            string
	uid             uint32
	expectedMapping *specs.LinuxIDMapping
}

var users = map[uint32]user.User{
	0: {
		Name:  "root",
		UID:   0,
		GID:   0,
		Dir:   "/root",
		Shell: "/bin/sh",
	},
	1: {
		Name:  "daemon",
		UID:   1,
		GID:   1,
		Dir:   "/usr/sbin",
		Shell: "/usr/sbin/nologin",
	},
	2: {
		Name:  "bin",
		UID:   2,
		GID:   2,
		Dir:   "/bin",
		Shell: "/usr/sbin/nologin",
	},
	3: {
		Name:  "sys",
		UID:   3,
		GID:   3,
		Dir:   "/dev",
		Shell: "/usr/sbin/nologin",
	},
	4: {
		Name:  "sync",
		UID:   4,
		GID:   4,
		Dir:   "/bin",
		Shell: "/usr/sbin/nologin",
	},
	5: {
		Name:  "games",
		UID:   5,
		GID:   5,
		Dir:   "/bin",
		Shell: "/usr/sbin/nologin",
	},
}

func getPwNamMock(username string) (*user.User, error) {
	for _, u := range users {
		if u.Name == username {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("no user found for %s", username)
}

func getPwUIDMock(uid uint32) (*user.User, error) {
	if u, ok := users[uid]; ok {
		return &u, nil
	}
	return nil, fmt.Errorf("no user found with ID %d", uid)
}

func testGetIDRange(t *testing.T, s set) {
	idRange, err := getIDRange(s.path, s.uid)
	if err != nil && s.expectedMapping != nil {
		t.Errorf("unexpected error for %q: %s", s.name, err)
	} else if err == nil && s.expectedMapping == nil {
		t.Errorf("unexpected success for %q", s.name)
	} else if err == nil && s.expectedMapping != nil {
		if s.expectedMapping.ContainerID != idRange.ContainerID {
			t.Errorf("bad container ID returned for %q: %d instead of %d", s.name, idRange.ContainerID, s.expectedMapping.ContainerID)
		}
		if s.expectedMapping.HostID != idRange.HostID {
			t.Errorf("bad host ID returned for %q: %d instead of %d", s.name, idRange.HostID, s.expectedMapping.HostID)
		}
		if s.expectedMapping.Size != idRange.Size {
			t.Errorf("bad size returned for %q: %d instead of %d", s.name, idRange.Size, s.expectedMapping.Size)
		}
	}
}

func TestGetIDRangePath(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	// mock user database (https://github.com/apptainer/singularity/issues/3957)
	getPwUID = getPwUIDMock
	getPwNam = getPwNamMock
	defer func() {
		getPwUID = user.GetPwUID
		getPwNam = user.GetPwNam
	}()

	f, err := fs.MakeTmpFile("", "subid-", 0o700)
	if err != nil {
		t.Fatalf("failed to create temporary file")
	}
	defer os.Remove(f.Name())

	subIDContent := `
root:100000:65536
1:165536:1
1:165536:165536
1:165536:65536
2:2000000:-1
3:-1:65536
4:2065536:1
5:5065536:131072
5:5065536:1000000
`

	f.WriteString(subIDContent)
	f.Close()

	tests := []set{
		{
			name: "empty path",
			path: "",
			uid:  0,
		},
		{
			name: "bad path",
			path: "/a/bad/path",
			uid:  0,
		},
		{
			name: "temporary file, bad uid",
			path: f.Name(),
			uid:  ^uint32(0),
		},
		{
			name: "temporary file, user root (good)",
			path: f.Name(),
			uid:  0,
			expectedMapping: &specs.LinuxIDMapping{
				ContainerID: 1,
				HostID:      100000,
				Size:        65536,
			},
		},
		{
			name: "temporary file, uid 1 (multiple good)",
			path: f.Name(),
			uid:  1,
			expectedMapping: &specs.LinuxIDMapping{
				ContainerID: 1,
				HostID:      165536,
				Size:        65536,
			},
		},
		{
			name: "temporary file, uid 2 (bad size)",
			path: f.Name(),
			uid:  2,
		},
		{
			name: "temporary file, uid 2 (bad containerID)",
			path: f.Name(),
			uid:  3,
		},
		{
			name: "temporary file, uid 4 (multiple bad)",
			path: f.Name(),
			uid:  4,
		},
		{
			name: "temporary file, uid 5 (multiple large)",
			path: f.Name(),
			uid:  5,
			expectedMapping: &specs.LinuxIDMapping{
				ContainerID: 1,
				HostID:      5065536,
				Size:        1000000,
			},
		},
		{
			name: "temporary file, uid 8 (doesn't exist)",
			path: f.Name(),
			uid:  8,
		},
	}
	for _, test := range tests {
		testGetIDRange(t, test)
	}
}
