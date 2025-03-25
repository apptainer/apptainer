// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package user

import (
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/ccoveille/go-safecast"
)

func TestGetPwUID(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	u, err := GetPwUID(0)
	if err != nil {
		t.Fatalf("Failed to retrieve information for UID 0")
	}
	if u.Name != "root" {
		t.Fatalf("UID 0 doesn't correspond to root user")
	}
}

func TestGetPwNam(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	u, err := GetPwNam("root")
	if err != nil {
		t.Fatalf("Failed to retrieve information for root user")
	}
	if u.UID != 0 {
		t.Fatalf("root user doesn't have UID 0")
	}
}

func TestGetGrGID(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	group, err := GetGrGID(0)
	if err != nil {
		t.Fatalf("Failed to retrieve information for GID 0")
	}
	if group.Name != "root" {
		t.Fatalf("GID 0 doesn't correspond to root group")
	}
}

func TestGetGrNam(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	group, err := GetGrNam("root")
	if err != nil {
		t.Fatalf("Failed to retrieve information for root group")
	}
	if group.GID != 0 {
		t.Fatalf("root group doesn't have GID 0")
	}
}

func testCurrent(t *testing.T, fn func() (*User, error)) {
	uid, err := safecast.ToUint32(os.Getuid())
	if err != nil {
		t.Fatal(err)
	}

	u, err := fn()
	if err != nil {
		t.Fatalf("Failed to retrieve information for current user")
	}
	if u.UID != uid {
		t.Fatalf("returned UID (%d) doesn't match current UID (%d)", uid, u.UID)
	}
}

func TestCurrent(t *testing.T) {
	// as a regular user
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	testCurrent(t, Current)

	// as root
	test.ResetPrivilege(t)

	testCurrent(t, Current)
}

func TestCurrentOriginal(t *testing.T) {
	// as a regular user
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	// to fully test CurrentOriginal, we would need
	// to execute it from a user namespace, actually
	// we just ensure that current user information
	// are returned
	testCurrent(t, CurrentOriginal)

	// as root
	test.ResetPrivilege(t)

	testCurrent(t, CurrentOriginal)
}
