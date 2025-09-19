// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build seccomp

package seccomp

import (
	"os"
	"syscall"
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/runtime/engine/config/oci/generate"
	"github.com/apptainer/apptainer/internal/pkg/test"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func defaultProfile() *specs.LinuxSeccomp {
	syscalls := []specs.LinuxSyscall{
		{
			Names:  []string{"fchmod"},
			Action: specs.ActErrno,
			Args: []specs.LinuxSeccompArg{
				{
					Index: 1,
					Value: 0o777,
					Op:    specs.OpEqualTo,
				},
			},
		},
	}
	return &specs.LinuxSeccomp{
		DefaultAction: specs.ActAllow,
		Syscalls:      syscalls,
	}
}

func testFchmod(t *testing.T) {
	tmpfile, err := os.CreateTemp(t.TempDir(), "chmod_file-")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpfile.Close()

	if hasConditionSupport() {
		// all modes except 0777 are permitted
		if err := syscall.Fchmod(int(tmpfile.Fd()), 0o755); err != nil {
			t.Errorf("fchmod syscall failed: %s", err)
		}
		if err := syscall.Fchmod(int(tmpfile.Fd()), 0o777); err == nil {
			t.Errorf("fchmod syscall didn't return operation not permitted")
		}
	} else {
		if err := syscall.Fchmod(int(tmpfile.Fd()), 0o755); err == nil {
			t.Errorf("fchmod syscall didn't return operation not permitted")
		}
	}
}

func TestLoadSeccompConfig(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	if err := LoadSeccompConfig(nil, false, 1); err == nil {
		t.Errorf("should have failed with an empty config")
	}
	if err := LoadSeccompConfig(defaultProfile(), true, 1); err != nil {
		t.Errorf("%s", err)
	}

	testFchmod(t)
}

func TestLoadProfileFromFile(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	gen := generate.New(nil)

	if err := LoadProfileFromFile("test_profile/fake.json", gen); err == nil {
		t.Errorf("should have failed with nonexistent file")
	}

	if err := LoadProfileFromFile("test_profile/test.json", gen); err != nil {
		t.Error(err)
	}

	if err := LoadSeccompConfig(gen.Config.Linux.Seccomp, true, 1); err != nil {
		t.Errorf("%s", err)
	}

	testFchmod(t)
}
