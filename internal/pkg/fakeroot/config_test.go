// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package fakeroot

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
	"github.com/apptainer/apptainer/internal/pkg/util/fs"
	"github.com/apptainer/apptainer/internal/pkg/util/user"
)

func getUserFn(username string) (*user.User, error) {
	var prefix string

	splitted := strings.Split(username, "_")
	prefix = splitted[0]
	uid, err := strconv.ParseUint(splitted[1], 10, 32)
	if err != nil {
		return nil, err
	}
	if prefix == "nouser" {
		return nil, fmt.Errorf("%s not found", username)
	}

	return &user.User{
		Name: prefix,
		UID:  uint32(uid),
	}, nil
}

func createConfig(t *testing.T) string {
	f, err := fs.MakeTmpFile("", "subid-", 0o644)
	if err != nil {
		t.Fatalf("failed to create temporary config: %s", err)
	}
	defer f.Close()

	var buf bytes.Buffer

	base := uint32(10)
	size := uint32(10)

	// valid users
	for i := base; i < base+size; i++ {
		line := fmt.Sprintf("valid_%d:%d:%d\n", i, startMax-((i-base)*validRangeCount), validRangeCount)
		buf.WriteString(line)
	}
	buf.WriteString("\n")
	// badstart users
	base += size
	for i := base; i < base+size; i++ {
		line := fmt.Sprintf("badstart_%d:%d:%d\n", i, -1, validRangeCount)
		buf.WriteString(line)
	}
	buf.WriteString("\n")
	// badcount users
	base += size
	for i := base; i < base+size; i++ {
		line := fmt.Sprintf("badcount_%d:%d:%d\n", i, (i+1)*validRangeCount, 0)
		buf.WriteString(line)
	}
	buf.WriteString("\n")
	// disabled users
	base += size
	for i := base; i < base+size; i++ {
		line := fmt.Sprintf("!disabled_%d:%d:%d\n", i, (i+1)*validRangeCount, validRangeCount)
		buf.WriteString(line)
	}
	// same user
	base += size
	for i := base; i < base+size; i++ {
		line := fmt.Sprintf("sameuser_%d:%d:%d\n", base, (i+1)*validRangeCount, 1)
		buf.WriteString(line)
	}
	// add a bad formatted entry
	buf.WriteString("badentry:\n")
	// add a nouser entry
	buf.WriteString("nouser_42:0:0\n")

	if _, err := f.Write(buf.Bytes()); err != nil {
		t.Fatalf("failed to write config: %s", err)
	}

	return f.Name()
}

func testGetUserEntry(t *testing.T, config *Config) {
	tests := []struct {
		desc          string
		username      string
		expectSuccess bool
	}{
		{
			desc:          "ValidUser",
			username:      "valid_10",
			expectSuccess: true,
		},
		{
			desc:          "ValidUserReportBadEntry",
			username:      "valid_10",
			expectSuccess: true,
		},
		{
			desc:          "NoUser",
			username:      "nouser_10",
			expectSuccess: false,
		},
		{
			desc:          "NoUserReportBadEntry",
			username:      "nouser_10",
			expectSuccess: false,
		},
		{
			desc:          "BadStartUser",
			username:      "badstart_20",
			expectSuccess: false,
		},
		{
			desc:          "BadStartUserReportBadEntry",
			username:      "badstart_20",
			expectSuccess: false,
		},
		{
			desc:          "DisabledUser",
			username:      "disabled_40",
			expectSuccess: true,
		},
		{
			desc:          "DisabledUserReportBadEntry",
			username:      "disabled_40",
			expectSuccess: true,
		},
		{
			desc:          "SameUser",
			username:      "sameuser_50",
			expectSuccess: false,
		},
		{
			desc:          "SameUserReportBadEntry",
			username:      "sameuser_50",
			expectSuccess: false,
		},
	}
	for _, tt := range tests {
		_, err := config.GetUserEntry(tt.username)
		if err != nil && tt.expectSuccess {
			t.Errorf("unexpected error for %q: %s", tt.desc, err)
		} else if err == nil && !tt.expectSuccess {
			t.Errorf("unexpected success for %q", tt.desc)
		}
	}
}

func testEditEntry(t *testing.T, config *Config) {
	tests := []struct {
		desc          string
		username      string
		editFn        func(string) error
		expectSuccess bool
	}{
		{
			desc:          "AddNoUser",
			username:      "nouser_10",
			editFn:        config.AddUser,
			expectSuccess: false,
		},
		{
			desc:          "RemoveNoUser",
			username:      "nouser_10",
			editFn:        config.RemoveUser,
			expectSuccess: false,
		},
		{
			desc:          "EnableNoUser",
			username:      "nouser_10",
			editFn:        config.EnableUser,
			expectSuccess: false,
		},
		{
			desc:          "DisableNoUser",
			username:      "nouser_10",
			editFn:        config.DisableUser,
			expectSuccess: false,
		},
		{
			desc:          "AddAnotherValidUser",
			username:      "valid_100",
			editFn:        config.AddUser,
			expectSuccess: true,
		},
		{
			desc:          "RemoveAnotherValidUser",
			username:      "valid_100",
			editFn:        config.RemoveUser,
			expectSuccess: true,
		},
		{
			desc:          "AddSameValidUser",
			username:      "valid_10",
			editFn:        config.AddUser,
			expectSuccess: true,
		},
		{
			desc:          "DisableValidUser",
			username:      "valid_11",
			editFn:        config.DisableUser,
			expectSuccess: true,
		},
		{
			desc:          "DisableSameValidUser",
			username:      "valid_11",
			editFn:        config.DisableUser,
			expectSuccess: true,
		},
		{
			desc:          "EnableDisabledUser",
			username:      "disabled_40",
			editFn:        config.EnableUser,
			expectSuccess: true,
		},
		{
			desc:          "EnableSameDisabledValidUser",
			username:      "disabled_40",
			editFn:        config.EnableUser,
			expectSuccess: true,
		},
		{
			desc:          "RemoveValidUser",
			username:      "valid_10",
			editFn:        config.RemoveUser,
			expectSuccess: true,
		},
		{
			desc:          "RemoveSameValidUser",
			username:      "valid_10",
			editFn:        config.RemoveUser,
			expectSuccess: false,
		},
		{
			desc:          "AddAnotherValidUser",
			username:      "valid_21",
			editFn:        config.AddUser,
			expectSuccess: true,
		},
	}
	for _, tt := range tests {
		err := tt.editFn(tt.username)
		if err != nil && tt.expectSuccess {
			t.Errorf("unexpected error for %q: %s", tt.desc, err)
		} else if err == nil && !tt.expectSuccess {
			t.Errorf("unexpected success for %q", tt.desc)
		}
	}

	file := config.file.Name()

	config.Close()

	// basic checks to verify that write works correctly
	config, err := GetConfig(file, true, getUserFn)
	if err != nil {
		t.Fatalf("unexpected error while getting config %s: %s", file, err)
	}
	defer config.Close()

	// this entry was removed
	if _, err := config.GetUserEntry("valid_10"); err == nil {
		t.Errorf("unexpected entry found for valid_10 user")
	}
	// this entry was disabled
	e, err := config.GetUserEntry("valid_11")
	if err != nil {
		t.Errorf("unexpected error for valid_11 user")
	} else if !e.disabled {
		t.Errorf("valid_11 user entry should be disabled")
	}
	// this entry was enabled
	e, err = config.GetUserEntry("disabled_40")
	if err != nil {
		t.Errorf("unexpected error for disabled_40 user")
	} else if e.disabled {
		t.Errorf("disabled_40 user entry should be enabled")
	}
	// this entry was added and range start should be
	// equal to startMax (as it replace valid_10)
	e, err = config.GetUserEntry("valid_21")
	if err != nil {
		t.Errorf("unexpected error for valid_21 user")
	} else if e.Start != startMax {
		t.Errorf("valid_21 user entry start range should be %d, got %d", startMax, e.Start)
	}
}

func TestConfig(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	file := createConfig(t)
	defer os.Remove(file)

	// test with empty path
	_, err := GetConfig("", true, nil)
	if err == nil {
		t.Fatalf("unexpected success while getting empty: %s", err)
	}

	config, err := GetConfig(file, true, getUserFn)
	if err != nil {
		t.Fatalf("unexpected error while getting config %s: %s", file, err)
	}

	testGetUserEntry(t, config)
	testEditEntry(t, config)
}
