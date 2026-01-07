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
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
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

	rootUser, err := user.Lookup("root")
	if err != nil {
		t.Fatal(err)
	}
	expectRootEntry := fmt.Sprintf("root:x:0:0:%s:/tmp:/bin/ash\n", rootUser.Name)
	if !strings.HasPrefix(string(outputPasswd), expectRootEntry) {
		t.Errorf("Expected root entry %q, not found in:\n%s", expectRootEntry, string(outputPasswd))
	}
}
