// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package files

import (
	"bytes"
	"os"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
)

func TestGroup(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	var gids []int
	uid := os.Getuid()

	_, err := Group("/fake", uid, gids, nil)
	if err == nil {
		t.Errorf("should have failed with bad group file")
	}
	_, err = Group("/etc/group", uid, gids, nil)
	if err != nil {
		t.Errorf("should have passed with correct group file")
	}
	// with an empty file
	f, err := os.CreateTemp("", "empty-group-")
	if err != nil {
		t.Error(err)
	}
	emptyGroup := f.Name()
	defer os.Remove(emptyGroup)
	f.Close()

	_, err = Group(emptyGroup, uid, gids, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestHostname(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	_, err := Hostname("")
	if err == nil {
		t.Errorf("should have failed with empty hostname")
	}
	content, err := Hostname("mycontainer")
	if err != nil {
		t.Errorf("should have passed with correct hostname")
	}
	if !bytes.Equal(content, []byte("mycontainer\n")) {
		t.Errorf("Hostname returns a bad content")
	}
	_, err = Hostname("bad|hostname")
	if err == nil {
		t.Errorf("should have failed with non valid hostname")
	}
}

func TestResolvConf(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	_, err := ResolvConf([]string{})
	if err == nil {
		t.Errorf("should have failed with empty dns")
	}
	_, err = ResolvConf([]string{"test"})
	if err == nil {
		t.Errorf("should have failed with bad dns")
	}
	content, err := ResolvConf([]string{"8.8.8.8"})
	if err != nil {
		t.Errorf("should have passed with valid dns")
	}
	if !bytes.Equal(content, []byte("nameserver 8.8.8.8\n")) {
		t.Errorf("ResolvConf returns a bad content")
	}
}
