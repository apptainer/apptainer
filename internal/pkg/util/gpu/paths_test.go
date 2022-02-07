// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020-2021, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

//go:build linux
// +build linux

package gpu

import (
	"reflect"
	"testing"
)

const testLibFile = "testdata/testliblist.conf"

var testLibList = []string{"libc.so", "echo"}

func Test_gpuliblist(t *testing.T) {
	gotLibs, err := gpuliblist(testLibFile)
	if err != nil {
		t.Errorf("gpuliblist() error = %v", err)
		return
	}
	if len(gotLibs) == 0 {
		t.Error("gpuliblist() gave no results")
	}
	if !reflect.DeepEqual(gotLibs, testLibList) {
		t.Errorf("gpuliblist() gave unexpected results, got: %v expected: %v", gotLibs, testLibList)
	}
}
