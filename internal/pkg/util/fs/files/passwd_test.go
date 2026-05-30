// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
)

func TestPasswd(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	uid := os.Getuid()

	// Test how Passwd() works with a bad passwd file
	_, err := Passwd("/fake", "/fake", uid, nil)
	if err == nil {
		t.Errorf("should have failed with bad passwd file")
	}

	// Adding current user to an empty file
	f, err := os.CreateTemp(t.TempDir(), "empty-passwd-")
	if err != nil {
		t.Fatal(err)
	}
	emptyPasswd := f.Name()
	f.Close()
	_, err = Passwd(emptyPasswd, "/home", uid, nil)
	if err != nil {
		t.Fatalf("Unexpected error in Passwd() when adding uid %d: %v", uid, err)
	}

	// Modifying root user in test file
	inputPasswdFilePath := filepath.Join(".", "testdata", "passwd.in")
	outputPasswd, err := Passwd(inputPasswdFilePath, "/tmp", 0, nil)
	if err != nil {
		t.Fatalf("Unexpected error in Passwd() when modifying root entry: %v", err)
	}

	// Username and gecos should be preserved for uid 0 (fakeroot safety)
	expectRootEntry := "root:x:0:0:root:/tmp:/bin/ash\n"
	if !strings.HasPrefix(string(outputPasswd), expectRootEntry) {
		t.Errorf("Expected root entry %q, not found in:\n%s", expectRootEntry, string(outputPasswd))
	}

	// For non-root users, username should be overwritten with host user's name
	pwInfo, err := user.GetPwUID(uint32(uid))
	if err != nil {
		t.Fatal(err)
	}
	passwdWithUser := filepath.Join(t.TempDir(), "passwd")
	wrongName := fmt.Sprintf("wrongname:x:%d:%d:wrong gecos:/old/home:/bin/sh\n", uid, pwInfo.GID)
	if err := os.WriteFile(passwdWithUser, []byte(wrongName), 0o644); err != nil {
		t.Fatal(err)
	}
	outputPasswd, err = Passwd(passwdWithUser, "/new/home", uid, nil)
	if err != nil {
		t.Fatalf("Unexpected error in Passwd() when modifying non-root entry: %v", err)
	}
	expectEntry := fmt.Sprintf("%s:x:%d:%d:%s:/new/home:/bin/sh\n", pwInfo.Name, uid, pwInfo.GID, pwInfo.Gecos)
	if !strings.HasPrefix(string(outputPasswd), expectEntry) {
		t.Errorf("Expected entry %q, not found in:\n%s", expectEntry, string(outputPasswd))
	}
}
