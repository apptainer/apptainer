// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package cli

import (
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestCreateConfDir(t *testing.T) {
	// create a random name for a directory
	// TODO - go 1.20 initializes seed randomly by default, so can drop this
	// deprecated call in future.
	rand.Seed(time.Now().UnixNano()) //nolint:staticcheck
	bytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		bytes[i] = byte(65 + rand.Intn(25))
	}
	dir := "/tmp/" + string(bytes)

	// create the directory and check that it exists
	handleConfDir(dir, "")
	defer os.RemoveAll(dir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("failed to create directory %s", dir)
	} else {
		// stick something in the directory and make sure it isn't deleted
		os.WriteFile(dir+"/foo", []byte(""), 0o655)
		handleConfDir(dir, "")
		if _, err := os.Stat(dir + "/foo"); os.IsNotExist(err) {
			t.Errorf("inadvertently overwrote existing directory %s", dir)
		}
	}
}
