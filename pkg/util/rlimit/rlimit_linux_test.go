// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2025, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package rlimit

import (
	"testing"

	"github.com/apptainer/apptainer/internal/pkg/test"
)

func TestGetSet(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	fileCur, fileMax, err := Get("RLIMIT_NOFILE")
	if err != nil {
		t.Error(err)
	}

	if err := Set("RLIMIT_NOFILE", fileCur, fileMax); err != nil {
		t.Error(err)
	}

	fileMax++

	if err := Set("RLIMIT_NOFILE", fileCur, fileMax); err == nil {
		t.Errorf("process doesn't have privileges to do that")
	}

	fileCur, fileMax, err = Get("RLIMIT_FAKE")
	if err == nil {
		t.Errorf("resource limit RLIMIT_FAKE doesn't exist")
	}

	if err := Set("RLIMIT_FAKE", fileCur, fileMax); err == nil {
		t.Errorf("resource limit RLIMIT_FAKE doesn't exist")
	}
}
